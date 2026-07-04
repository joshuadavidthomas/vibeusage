package codex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/keychain"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
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

// Auth points users at the Codex CLI for credential setup. vibeusage is a
// read-only consumer of the Codex CLI's rotating OAuth chain; manually
// pasted access tokens never refreshed and silently broke once expired.
func (c Codex) Auth() provider.AuthFlow {
	return provider.CustomAuthFlow{
		RunFlow: func(w io.Writer, quiet bool) (bool, error) {
			// We can only detect what the Codex CLI has already written. If a
			// canonical source is present, accept it as success; otherwise
			// instruct the user and return failure so the auth command exits
			// non-zero (instead of silently no-oping).
			s := &OAuthStrategy{}
			if s.IsAvailable() {
				if !quiet {
					_, _ = fmt.Fprintln(w, "✓ Codex CLI credentials detected.")
				}
				return true, nil
			}
			if !quiet {
				_, _ = fmt.Fprintln(w, "Codex uses OAuth tokens managed by the Codex CLI.")
				_, _ = fmt.Fprintln(w, "Install the CLI and run `codex login`, then re-run this command.")
			}
			return false, fmt.Errorf("Codex CLI credentials not detected")
		},
	}
}

func init() {
	provider.Register(Codex{})
}

const (
	defaultUsageURL    = "https://chatgpt.com/backend-api/wham/usage"
	codexKeychainLabel = "Codex Auth"
)

var readKeychainSecret = keychain.ReadGenericPassword

type OAuthStrategy struct {
	HTTPTimeout float64
}

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.externalPaths() {
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

	usageURL := s.getUsageURL()
	client := httpclient.NewFromConfig(s.HTTPTimeout)

	result, unauthorized, err := s.fetchUsage(ctx, client, usageURL, creds)
	if err != nil || !unauthorized || creds.RefreshToken == "" {
		return result, err
	}

	refreshed := s.refreshViaCLI(ctx)
	if refreshed == nil {
		return fetch.ResultFatal("OAuth token expired and could not be refreshed. Re-authenticate with `codex login`."), nil
	}
	result, _, err = s.fetchUsage(ctx, client, usageURL, refreshed)
	return result, err
}

func (s *OAuthStrategy) refreshViaCLI(ctx context.Context) *oauth.Credentials {
	return oauth.RefreshViaCLI(ctx, oauth.CLIRefreshConfig{
		BinaryName: "codex",
		Args: []string{
			"exec", "say ok",
			"--skip-git-repo-check",
			"--sandbox", "read-only",
		},
		LoadCredentials: func() *oauth.Credentials {
			return s.loadCredentials()
		},
	})
}

func (s *OAuthStrategy) fetchUsage(ctx context.Context, client *httpclient.Client, usageURL string, creds *oauth.Credentials) (fetch.FetchResult, bool, error) {
	var usageResp UsageResponse
	resp, err := client.GetJSONCtx(ctx, usageURL, &usageResp, httpclient.WithBearer(creds.AccessToken))
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), false, nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("OAuth token expired or invalid. Re-authenticate with `codex login`."), true, nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFail("Not authorized. Account may not have ChatGPT Plus/Pro subscription."), false, nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), false, nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from usage endpoint: %v", resp.JSONErr)), false, nil
	}

	snapshot := s.parseTypedUsageResponse(usageResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), false, nil
	}

	return fetch.ResultOK(*snapshot), false, nil
}

func (s *OAuthStrategy) externalPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".codex", "auth.json")}
}

func (s *OAuthStrategy) loadCredentials() *oauth.Credentials {
	// Read only from canonical Codex CLI sources. The OAuth chain is owned
	// by the Codex CLI; vibeusage is a piggy-back consumer and never writes
	// new tokens. Any stale entry in vibeusage's own credentials store is
	// cleared lazily so it can't shadow the live source.
	for _, path := range s.externalPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cliCreds CLICredentials
		if err := json.Unmarshal(data, &cliCreds); err != nil {
			continue
		}
		if creds := cliCreds.EffectiveCredentials(); creds != nil {
			cleanupOrphanOAuthSlot()
			return creds
		}
	}

	if creds := s.loadKeychainCredentials(); creds != nil {
		cleanupOrphanOAuthSlot()
		return creds
	}
	return nil
}

// cleanupOrphanOAuthSlot removes any vibeusage-owned codex/oauth credential.
// Earlier versions of vibeusage wrote rotated refresh tokens here (and accepted
// manually pasted access tokens), both of which silently invalidated the Codex
// CLI's own refresh token. The slot has no legitimate use now that Auth()
// points users at `codex login`, so it is safe to drop whenever a canonical
// CLI/keychain source is present.
func cleanupOrphanOAuthSlot() {
	if config.HasCredential("codex", "oauth") {
		config.DeleteCredential("codex", "oauth")
	}
}

func (s *OAuthStrategy) loadKeychainCredentials() *oauth.Credentials {
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
		periods = append(periods, rateLimitPeriods(rl, "", "Session", "Weekly")...)
	}

	// Code review rate limit as its own set of periods.
	if cr := resp.CodeReviewRateLimit; cr != nil {
		periods = append(periods, rateLimitPeriods(cr, "", "Code Review", "Code Review Weekly")...)
	}

	// Additional (model-specific) rate limits.
	for _, arl := range resp.AdditionalRateLimits {
		if arl.RateLimit == nil || arl.LimitName == "" {
			continue
		}
		periods = append(periods, rateLimitPeriods(
			arl.RateLimit,
			arl.LimitName,
			arl.LimitName,
			arl.LimitName+" Weekly",
		)...)
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

// rateLimitPeriods extracts UsagePeriod entries from a RateLimits struct.
// model is set on the resulting periods for model-specific limits.
func rateLimitPeriods(rl *RateLimits, model, primaryName, secondaryName string) []models.UsagePeriod {
	var periods []models.UsagePeriod

	if primary := rl.EffectivePrimary(); primary != nil {
		p := models.UsagePeriod{
			Name:        primaryName,
			Utilization: int(primary.UsedPercent),
			PeriodType:  models.PeriodSession,
			Model:       model,
		}
		if ts := primary.EffectiveResetTimestamp(); ts > 0 {
			t := time.Unix(int64(ts), 0).UTC()
			p.ResetsAt = &t
		}
		periods = append(periods, p)
	}

	if secondary := rl.EffectiveSecondary(); secondary != nil {
		p := models.UsagePeriod{
			Name:        secondaryName,
			Utilization: int(secondary.UsedPercent),
			PeriodType:  models.PeriodWeekly,
			Model:       model,
		}
		if ts := secondary.EffectiveResetTimestamp(); ts > 0 {
			t := time.Unix(int64(ts), 0).UTC()
			p.ResetsAt = &t
		}
		periods = append(periods, p)
	}

	return periods
}
