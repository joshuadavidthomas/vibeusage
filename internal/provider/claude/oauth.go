package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/keychain"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

const (
	oauthUsageURL        = "https://api.anthropic.com/api/oauth/usage"
	oauthAccountURL      = "https://api.anthropic.com/api/oauth/account"
	anthropicBetaTag     = "oauth-2025-04-20"
	claudeKeychainSecret = "Claude Code-credentials"

	// accountReuseTTL is how long a cached identity (email, plan, org)
	// is trusted before we re-fetch /oauth/account. Identity data changes
	// on the order of months (plan upgrades, org renames); reusing it for
	// a day halves the per-fetch request count at negligible freshness cost.
	accountReuseTTL = 24 * time.Hour
)

var (
	oauthUsageEndpoint   = oauthUsageURL
	oauthAccountEndpoint = oauthAccountURL
)

var readKeychainSecret = keychain.ReadGenericPassword

// OAuthStrategy fetches Claude usage using OAuth credentials.
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
		return fetch.ResultFatal("Invalid OAuth credentials: missing access_token"), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	result, unauthorized, err := s.fetchWithCredentials(ctx, client, creds)
	if err != nil || !unauthorized || creds.RefreshToken == "" {
		return result, err
	}

	refreshed := s.refreshViaCLI(ctx)
	if refreshed == nil {
		return fetch.ResultFatal("OAuth token expired and could not be refreshed. Re-authenticate with the Claude CLI."), nil
	}
	result, _, err = s.fetchWithCredentials(ctx, client, refreshed)
	return result, err
}

func (s *OAuthStrategy) refreshViaCLI(ctx context.Context) *oauth.Credentials {
	return oauth.RefreshViaCLI(ctx, oauth.CLIRefreshConfig{
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

func (s *OAuthStrategy) fetchWithCredentials(ctx context.Context, client *httpclient.Client, creds *oauth.Credentials) (fetch.FetchResult, bool, error) {
	authOpts := []httpclient.RequestOption{
		httpclient.WithBearer(creds.AccessToken),
		httpclient.WithHeader("anthropic-beta", anthropicBetaTag),
	}

	cachedIdentity := loadCachedIdentity()

	// Fetch usage, and (when no recent cached identity) account concurrently.
	// Account enrichment is best-effort — failures are silently ignored.
	type usageOutcome struct {
		resp    OAuthUsageResponse
		status  int
		header  http.Header
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
		resp, err := client.GetJSONCtx(ctx, oauthUsageEndpoint, &usageResp, authOpts...)
		if err != nil {
			usageCh <- usageOutcome{err: err}
			return
		}
		usageCh <- usageOutcome{resp: usageResp, status: resp.StatusCode, header: resp.Header, jsonErr: resp.JSONErr}
	}()

	if cachedIdentity != nil {
		accountCh <- accountOutcome{}
	} else {
		go func() {
			var acctResp OAuthAccountResponse
			resp, err := client.GetJSONCtx(ctx, oauthAccountEndpoint, &acctResp, authOpts...)
			if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
				accountCh <- accountOutcome{}
				return
			}
			accountCh <- accountOutcome{resp: &acctResp}
		}()
	}

	usage := <-usageCh
	account := <-accountCh

	if usage.err != nil {
		return fetch.ResultFail("Request failed: " + usage.err.Error()), false, nil
	}
	if usage.status == 401 {
		return fetch.ResultFatal("OAuth token expired or invalid. Re-authenticate with the Claude CLI."), true, nil
	}
	if usage.status == 403 {
		return fetch.ResultFatal("Not authorized to access usage."), false, nil
	}
	if usage.status == 429 {
		retryAt := parseRetryAfter(usage.header, time.Now().UTC())
		return fetch.ResultThrottled("Rate limited by Anthropic", retryAt), false, nil
	}
	if usage.status != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", usage.status)), false, nil
	}
	if usage.jsonErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from usage endpoint: %v", usage.jsonErr)), false, nil
	}

	snapshot := s.parseOAuthUsageResponse(usage.resp)

	if cachedIdentity != nil {
		snapshot.Identity = cachedIdentity
	} else {
		enrichWithAccount(snapshot, account.resp)
	}

	return fetch.ResultOK(*snapshot), false, nil
}

// loadCachedIdentity returns the cached Claude identity if the on-disk
// snapshot is within accountReuseTTL and has a populated Identity. Returns
// nil otherwise, signalling that /oauth/account should be fetched.
func loadCachedIdentity() *models.ProviderIdentity {
	cached := config.LoadCachedSnapshot("claude")
	if cached == nil || cached.Identity == nil {
		return nil
	}
	if time.Since(cached.FetchedAt) > accountReuseTTL {
		return nil
	}
	id := cached.Identity
	if id.Email == "" && id.Plan == "" && id.Organization == "" {
		return nil
	}
	return id
}

func (s *OAuthStrategy) externalPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".claude", ".credentials.json")}
}

func (s *OAuthStrategy) loadCredentials() *oauth.Credentials {
	// Read only from canonical Claude CLI sources. The OAuth chain is owned
	// by the Claude CLI; vibeusage is a piggy-back consumer and never writes
	// new tokens. Any stale entry in vibeusage's own credentials store is
	// cleared lazily so it can't shadow the live source.
	for _, path := range s.externalPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if creds := parseClaudeCredentials(data); creds != nil {
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

// cleanupOrphanOAuthSlot removes any vibeusage-owned claude/oauth credential.
// Earlier versions of vibeusage wrote rotated refresh tokens here, which
// silently invalidated the Claude CLI's own refresh token. The vibeusage slot
// has no legitimate use for Claude (manual auth uses claude/session), so it
// is safe to drop whenever a canonical CLI/keychain source is present.
func cleanupOrphanOAuthSlot() {
	if config.HasCredential("claude", "oauth") {
		config.DeleteCredential("claude", "oauth")
	}
}

func parseClaudeCredentials(data []byte) *oauth.Credentials {
	// Try Claude CLI format first
	var cliCreds ClaudeCLICredentials
	if err := json.Unmarshal(data, &cliCreds); err == nil && cliCreds.ClaudeAiOauth != nil {
		creds := cliCreds.ClaudeAiOauth.ToOAuthCredentials()
		return &creds
	}

	// Standard vibeusage format
	var creds oauth.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil
	}
	if creds.AccessToken != "" {
		return &creds
	}
	return nil
}

func (s *OAuthStrategy) loadKeychainCredentials() *oauth.Credentials {
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

func (s *OAuthStrategy) parseOAuthUsageResponse(resp OAuthUsageResponse) *models.UsageSnapshot {
	return parseUsageResponse(resp, "oauth")
}
