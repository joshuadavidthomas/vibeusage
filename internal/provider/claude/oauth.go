package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	oauthAccountURL      = "https://api.anthropic.com/api/oauth/account"
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
			refreshed = oauth.RefreshViaCLI(ctx, oauth.CLIRefreshConfig{
				BinaryName: "claude",
				Args: []string{
					"-p", "ok",
					"--model", "haiku",
					"--output-format", "json",
					"--no-session-persistence",
					"--permission-mode", "plan",
					"--allowed-tools", "",
					"--max-budget-usd", "0.001",
				},
				LoadCredentials: func() *oauth.Credentials {
					return s.loadCredentials()
				},
			})
		}
		if refreshed == nil {
			return fetch.ResultFatal("OAuth token expired and could not be refreshed. Re-authenticate with the Claude CLI."), nil
		}
		creds = refreshed
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	authOpts := []httpclient.RequestOption{
		httpclient.WithBearer(creds.AccessToken),
		httpclient.WithHeader("anthropic-beta", anthropicBetaTag),
	}

	// Fetch usage and account concurrently. Account enrichment is
	// best-effort — failures are silently ignored.
	type usageOutcome struct {
		resp    OAuthUsageResponse
		status  int
		jsonErr error
		err     error
	}
	type accountOutcome struct {
		resp *OAuthAccountResponse
	}

	usageCh := make(chan usageOutcome, 1)
	accountCh := make(chan accountOutcome, 1)

	go func() {
		var usageResp OAuthUsageResponse
		resp, err := client.GetJSONCtx(ctx, oauthUsageURL, &usageResp, authOpts...)
		if err != nil {
			usageCh <- usageOutcome{err: err}
			return
		}
		usageCh <- usageOutcome{resp: usageResp, status: resp.StatusCode, jsonErr: resp.JSONErr}
	}()

	go func() {
		var acctResp OAuthAccountResponse
		resp, err := client.GetJSONCtx(ctx, oauthAccountURL, &acctResp, authOpts...)
		if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
			accountCh <- accountOutcome{}
			return
		}
		accountCh <- accountOutcome{resp: &acctResp}
	}()

	usage := <-usageCh
	account := <-accountCh

	if usage.err != nil {
		return fetch.ResultFail("Request failed: " + usage.err.Error()), nil
	}
	if usage.status == 401 {
		return fetch.ResultFatal("OAuth token expired or invalid. Re-authenticate with the Claude CLI."), nil
	}
	if usage.status == 403 {
		return fetch.ResultFatal("Not authorized to access usage."), nil
	}
	if usage.status != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", usage.status)), nil
	}
	if usage.jsonErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from usage endpoint: %v", usage.jsonErr)), nil
	}

	snapshot := s.parseOAuthUsageResponse(usage.resp)

	enrichWithAccount(snapshot, account.resp)

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



func (s *OAuthStrategy) parseOAuthUsageResponse(resp OAuthUsageResponse) *models.UsageSnapshot {
	return parseUsageResponse(resp, "oauth", nil)
}
