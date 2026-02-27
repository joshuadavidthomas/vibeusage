package codex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/keychain"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Codex struct{}

func (c Codex) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "codex",
		Name:         "Codex",
		Description:  "OpenAI's ChatGPT and Codex",
		Homepage:     "https://chatgpt.com",
		StatusURL:    "https://status.openai.com",
		DashboardURL: "https://chatgpt.com/codex/settings/usage",
	}
}

func (c Codex) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.codex/auth.json"},
		EnvVars:  []string{"OPENAI_API_KEY"},
	}
}

func (c Codex) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&OAuthStrategy{HTTPTimeout: timeout}}
}

func (c Codex) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.openai.com")
}

// Auth returns the manual bearer token flow for Codex.
// Codex uses OAuth tokens managed by the Codex CLI — users must authenticate
// with the CLI first, or provide an access token obtained from the browser.
func (c Codex) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Codex uses OAuth tokens from the Codex CLI (recommended):\n" +
			"  Install the CLI and run `codex login`\n" +
			"\n" +
			"Or provide an access token manually:\n" +
			"  1. Open https://chatgpt.com in your browser and sign in\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I) → Network tab\n" +
			"  3. Reload the page and click any request to chatgpt.com/backend-api/\n" +
			"  4. In Request Headers, find the Authorization header\n" +
			"  5. Copy the value after \"Bearer \" (starts with ey...)\n" +
			"\n" +
			"Note: Manually obtained tokens won't auto-refresh — run auth again when they expire.",
		Placeholder: "ey... (OAuth access token)",
		Validate:    provider.ValidateNotEmpty,
		CredPath:    config.CredentialPath("codex", "oauth"),
		JSONKey:     "access_token",
	}
}

func init() {
	provider.Register(Codex{})
}

const (
	// OAuth client ID extracted from the Codex CLI installation.
	// Required to refresh tokens stored in ~/.codex/auth.json.
	codexClientID      = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexTokenURL      = "https://auth.openai.com/oauth/token"
	defaultUsageURL    = "https://chatgpt.com/backend-api/wham/usage"
	codexKeychainLabel = "Codex Auth"
)

var readKeychainSecret = keychain.ReadGenericPassword

type OAuthStrategy struct {
	HTTPTimeout float64
}

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return s.loadKeychainCredentials() != nil
}

func (s *OAuthStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found"), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(ctx, creds)
		if refreshed == nil {
			refreshed = s.tryRefreshViaCLI(ctx)
		}
		if refreshed == nil {
			return fetch.ResultFatal("OAuth token expired and could not be refreshed. Re-authenticate with `codex login`."), nil
		}
		creds = refreshed
	}

	usageURL := s.getUsageURL()
	client := httpclient.NewFromConfig(s.HTTPTimeout)

	result, retry := s.fetchUsage(ctx, client, usageURL, creds)
	if retry {
		// The Codex CLI doesn't store expires_at, so NeedsRefresh() can't
		// detect expiry upfront. Try refreshing now that the API told us
		// the token is stale.
		refreshed := s.refreshToken(ctx, creds)
		if refreshed == nil {
			refreshed = s.tryRefreshViaCLI(ctx)
		}
		if refreshed != nil {
			result, _ = s.fetchUsage(ctx, client, usageURL, refreshed)
			return result, nil
		}
		return fetch.ResultFatal("OAuth token expired or invalid. Re-authenticate with `codex login`."), nil
	}
	return result, nil
}

// fetchUsage makes the usage API request and parses the response. The second
// return value is true when the caller should attempt a token refresh and retry
// (i.e. a 401 response).
func (s *OAuthStrategy) fetchUsage(ctx context.Context, client *httpclient.Client, usageURL string, creds *Credentials) (fetch.FetchResult, bool) {
	var usageResp UsageResponse
	resp, err := client.GetJSONCtx(ctx, usageURL, &usageResp, httpclient.WithBearer(creds.AccessToken))
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), false
	}

	if resp.StatusCode == 401 {
		return fetch.FetchResult{}, true
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFail("Not authorized. Account may not have ChatGPT Plus/Pro subscription."), false
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), false
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from usage endpoint: %v", resp.JSONErr)), false
	}

	snapshot := s.parseTypedUsageResponse(usageResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), false
	}

	return fetch.ResultOK(*snapshot), false
}

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return provider.CredentialSearchPaths("codex", "oauth", filepath.Join(home, ".codex", "auth.json"))
}

func (s *OAuthStrategy) loadCredentials() *Credentials {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}
		var cliCreds CLICredentials
		if err := json.Unmarshal(data, &cliCreds); err != nil {
			continue
		}
		if creds := cliCreds.EffectiveCredentials(); creds != nil {
			return creds
		}
	}
	return s.loadKeychainCredentials()
}

func (s *OAuthStrategy) loadKeychainCredentials() *Credentials {
	secret, err := readKeychainSecret(codexKeychainLabel, codexKeychainAccount())
	if err != nil || secret == "" {
		return nil
	}

	var cliCreds CLICredentials
	if err := json.Unmarshal([]byte(secret), &cliCreds); err != nil {
		return nil
	}

	return cliCreds.EffectiveCredentials()
}

func codexKeychainAccount() string {
	home := codexHomeDir()
	sum := sha256.Sum256([]byte(home))
	hash := fmt.Sprintf("%x", sum[:])
	return "cli|" + hash[:16]
}

func codexHomeDir() string {
	if v := strings.TrimSpace(os.Getenv("CODEX_HOME")); v != "" {
		if strings.HasPrefix(v, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				return filepath.Clean(filepath.Join(home, v[2:]))
			}
		}
		return filepath.Clean(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(".codex")
	}
	return filepath.Join(home, ".codex")
}

func (s *OAuthStrategy) refreshToken(ctx context.Context, creds *Credentials) *Credentials {
	return oauth.Refresh(ctx, creds.RefreshToken, oauth.RefreshConfig{
		TokenURL:    codexTokenURL,
		FormFields:  map[string]string{"client_id": codexClientID},
		ProviderID:  "codex",
		HTTPTimeout: s.HTTPTimeout,
	})
}

// tryRefreshViaCLI attempts to refresh the OAuth token by running the Codex CLI
// in exec mode, which refreshes credentials as a side effect on startup.
func (s *OAuthStrategy) tryRefreshViaCLI(ctx context.Context) *Credentials {
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tctx, codexPath,
		"exec", "say ok",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
	)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		if creds := s.loadCredentials(); creds != nil && !creds.NeedsRefresh() {
			stopCommand(cmd)
			return creds
		}

		select {
		case <-done:
			creds := s.loadCredentials()
			if creds == nil || creds.NeedsRefresh() {
				return nil
			}
			return creds
		case <-tctx.Done():
			stopCommand(cmd)
			creds := s.loadCredentials()
			if creds == nil || creds.NeedsRefresh() {
				return nil
			}
			return creds
		case <-ticker.C:
		}
	}
}

func stopCommand(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (s *OAuthStrategy) getUsageURL() string {
	// Check for custom URL in codex config
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		// Simple TOML parse for usage_url
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "usage_url") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					url := strings.TrimSpace(parts[1])
					url = strings.Trim(url, `"'`)
					if url != "" {
						return url
					}
				}
			}
		}
	}
	return defaultUsageURL
}

func (s *OAuthStrategy) parseTypedUsageResponse(resp UsageResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	rl := resp.EffectiveRateLimits()
	if rl != nil {
		if primary := rl.EffectivePrimary(); primary != nil {
			p := models.UsagePeriod{
				Name:        "Session",
				Utilization: int(primary.UsedPercent),
				PeriodType:  models.PeriodSession,
			}
			if ts := primary.EffectiveResetTimestamp(); ts > 0 {
				t := time.Unix(int64(ts), 0).UTC()
				p.ResetsAt = &t
			}
			periods = append(periods, p)
		}

		if secondary := rl.EffectiveSecondary(); secondary != nil {
			p := models.UsagePeriod{
				Name:        "Weekly",
				Utilization: int(secondary.UsedPercent),
				PeriodType:  models.PeriodWeekly,
			}
			if ts := secondary.EffectiveResetTimestamp(); ts > 0 {
				t := time.Unix(int64(ts), 0).UTC()
				p.ResetsAt = &t
			}
			periods = append(periods, p)
		}
	}

	if len(periods) == 0 {
		return nil
	}

	var overage *models.OverageUsage
	if resp.Credits != nil && resp.Credits.HasCredits {
		overage = &models.OverageUsage{
			Used:      0,
			Limit:     resp.Credits.Balance(),
			Currency:  "credits",
			IsEnabled: true,
		}
	}

	var identity *models.ProviderIdentity
	if resp.PlanType != "" {
		identity = &models.ProviderIdentity{Plan: resp.PlanType}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "codex",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    "oauth",
	}
}
