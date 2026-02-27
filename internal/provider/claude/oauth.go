package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/keychain"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

const (
	oauthUsageURL        = "https://api.anthropic.com/api/oauth/usage"
	oauthTokenURL        = "https://api.anthropic.com/oauth/token"
	anthropicBetaTag     = "oauth-2025-04-20"
	claudeKeychainSecret = "Claude Code-credentials"
)

var readKeychainSecret = keychain.ReadGenericPassword

// OAuthStrategy fetches Claude usage using OAuth credentials.
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
		return fetch.ResultFatal("Invalid OAuth credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(ctx, creds)
		if refreshed == nil {
			refreshed = s.tryRefreshViaCLI(ctx)
		}
		if refreshed == nil {
			return fetch.ResultFatal("OAuth token expired and could not be refreshed. Re-authenticate with the Claude CLI."), nil
		}
		creds = refreshed
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var usageResp OAuthUsageResponse
	resp, err := client.GetJSONCtx(ctx, oauthUsageURL, &usageResp,
		httpclient.WithBearer(creds.AccessToken),
		httpclient.WithHeader("anthropic-beta", anthropicBetaTag),
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("OAuth token expired or invalid. Re-authenticate with the Claude CLI."), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFatal("Not authorized to access usage."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from usage endpoint: %v", resp.JSONErr)), nil
	}

	snapshot := s.parseOAuthUsageResponse(usageResp)

	return fetch.ResultOK(*snapshot), nil
}

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return provider.CredentialSearchPaths("claude", "oauth", filepath.Join(home, ".claude", ".credentials.json"))
}

func (s *OAuthStrategy) loadCredentials() *OAuthCredentials {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}

		// Try Claude CLI format first
		var cliCreds ClaudeCLICredentials
		if err := json.Unmarshal(data, &cliCreds); err == nil && cliCreds.ClaudeAiOauth != nil {
			creds := cliCreds.ClaudeAiOauth.ToOAuthCredentials()
			return &creds
		}

		// Standard vibeusage format
		var creds OAuthCredentials
		if err := json.Unmarshal(data, &creds); err != nil {
			continue
		}
		if creds.AccessToken != "" {
			return &creds
		}
	}
	return s.loadKeychainCredentials()
}

func (s *OAuthStrategy) loadKeychainCredentials() *OAuthCredentials {
	secret, err := readKeychainSecret(claudeKeychainSecret, "")
	if err != nil || secret == "" {
		return nil
	}

	var cliCreds ClaudeCLICredentials
	if err := json.Unmarshal([]byte(secret), &cliCreds); err != nil || cliCreds.ClaudeAiOauth == nil {
		return nil
	}

	creds := cliCreds.ClaudeAiOauth.ToOAuthCredentials()
	if creds.AccessToken == "" {
		return nil
	}
	return &creds
}

func (s *OAuthStrategy) refreshToken(ctx context.Context, creds *OAuthCredentials) *OAuthCredentials {
	return oauth.Refresh(ctx, creds.RefreshToken, oauth.RefreshConfig{
		TokenURL:    oauthTokenURL,
		Headers:     []httpclient.RequestOption{httpclient.WithHeader("anthropic-beta", anthropicBetaTag)},
		ProviderID:  "claude",
		HTTPTimeout: s.HTTPTimeout,
	})
}

// tryRefreshViaCLI attempts to refresh the OAuth token by running Claude CLI
// print mode, which has been observed to refresh credentials as a side effect.
// We prefer haiku to minimize refresh cost.
func (s *OAuthStrategy) tryRefreshViaCLI(ctx context.Context) *OAuthCredentials {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tctx, claudePath,
		"-p", "ok",
		"--model", "haiku",
		"--output-format", "json",
		"--no-session-persistence",
		"--permission-mode", "plan",
		"--allowed-tools", "",
		"--max-budget-usd", "0.001",
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

// inferClaudePlan returns the plan tier when it can be reliably determined
// from the usage response. The API does not include an explicit plan field,
// so we only return a value when plan/billing_type is present in the
// response — we no longer guess from feature availability since that
// can't distinguish Pro from Max tiers.
func inferClaudePlan(resp OAuthUsageResponse) string {
	return ""
}

func (s *OAuthStrategy) parseOAuthUsageResponse(resp OAuthUsageResponse) *models.UsageSnapshot {
	return parseUsageResponse(resp, "oauth", nil)
}
