package claude

import (
	"context"
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
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

const (
	oauthUsageURL        = "https://api.anthropic.com/api/oauth/usage"
	oauthTokenURL        = "https://api.anthropic.com/oauth/token"
	webBaseURL           = "https://claude.ai/api/organizations"
	anthropicBetaTag     = "oauth-2025-04-20"
	claudeKeychainSecret = "Claude Code-credentials"
)

var readKeychainSecret = keychain.ReadGenericPassword

type Claude struct{}

func (c Claude) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "claude",
		Name:         "Claude",
		Description:  "Anthropic's Claude AI assistant",
		Homepage:     "https://claude.ai",
		StatusURL:    "https://status.anthropic.com",
		DashboardURL: "https://claude.ai/settings/usage",
	}
}

func (c Claude) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.claude/.credentials.json"},
		EnvVars:  []string{"ANTHROPIC_API_KEY"},
	}
}

func (c Claude) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
		&APIKeyStrategy{HTTPTimeout: timeout},
		&WebStrategy{HTTPTimeout: timeout},
	}
}

func (c Claude) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.anthropic.com/api/v2/status.json")
}

// Auth returns a manual credential flow for Claude.
//
// Accepted inputs:
// - Anthropic API key (sk-ant-api... / sk-ant-admin-...)
// - claude.ai sessionKey cookie (sk-ant-sid01-...) as web fallback
func (c Claude) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Provide one of the following credentials:\n" +
			"\n" +
			"Option A (recommended): Claude CLI OAuth\n" +
			"  Run `claude auth login` and vibeusage will auto-detect it.\n" +
			"\n" +
			"Option B: Anthropic API key\n" +
			"  Use a key from https://platform.claude.com/settings/keys (starts with sk-ant-api or sk-ant-admin-).\n" +
			"\n" +
			"Option C (fallback): claude.ai session key\n" +
			"  1. Open https://claude.ai in your browser\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://claude.ai\n" +
			"  4. Find the sessionKey cookie\n" +
			"  5. Copy its value (starts with sk-ant-sid01-)",
		Placeholder: "sk-ant-sid01-... or sk-ant-api...",
		Validate:    provider.ValidateAnyPrefix("sk-ant-sid01-", "sk-ant-api", "sk-ant-admin-"),
		Save:        saveClaudeCredential,
	}
}

func saveClaudeCredential(value string) error {
	value = strings.TrimSpace(value)

	path := config.CredentialPath("claude", "session")
	key := "session_key"
	if strings.HasPrefix(value, "sk-ant-api") || strings.HasPrefix(value, "sk-ant-admin-") {
		path = config.CredentialPath("claude", "apikey")
		key = "api_key"
	}

	content, _ := json.Marshal(map[string]string{key: value})
	return config.WriteCredential(path, content)
}

func init() {
	provider.Register(Claude{})
}

// OAuthStrategy fetches Claude usage using OAuth credentials.
type OAuthStrategy struct {
	HTTPTimeout float64
}

func (s *OAuthStrategy) Name() string { return "oauth" }

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	if !provider.ExternalCredentialReuseEnabled() {
		return false
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
		if refreshed == nil && provider.ExternalCredentialReuseEnabled() {
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
	if !provider.ExternalCredentialReuseEnabled() {
		return nil
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
	if creds.RefreshToken == "" {
		return nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var tokenResp OAuthTokenResponse
	resp, err := client.PostFormCtx(ctx, oauthTokenURL,
		map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": creds.RefreshToken,
		},
		&tokenResp,
		httpclient.WithHeader("anthropic-beta", anthropicBetaTag),
	)
	if err != nil {
		return nil
	}
	if resp.StatusCode != 200 {
		return nil
	}
	if resp.JSONErr != nil {
		return nil
	}

	updated := &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}

	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		updated.ExpiresAt = expiresAt.Format(time.RFC3339)
	}

	// Preserve refresh token if the server didn't issue a new one
	if updated.RefreshToken == "" {
		updated.RefreshToken = creds.RefreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath("claude", "oauth"), content)

	return updated
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

// inferClaudePlan guesses the plan tier from usage response features.
func inferClaudePlan(resp OAuthUsageResponse) string {
	if resp.ExtraUsage != nil && resp.ExtraUsage.IsEnabled {
		return "Pro"
	}
	if resp.SevenDayOpus != nil {
		return "Pro"
	}
	return ""
}

func (s *OAuthStrategy) parseOAuthUsageResponse(resp OAuthUsageResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	type periodInfo struct {
		name       string
		periodType models.PeriodType
	}

	standardPeriods := []struct {
		data *UsagePeriodResponse
		info periodInfo
	}{
		{resp.FiveHour, periodInfo{"Session (5h)", models.PeriodSession}},
		{resp.SevenDay, periodInfo{"All Models", models.PeriodWeekly}},
		{resp.Monthly, periodInfo{"Monthly", models.PeriodMonthly}},
	}

	for _, sp := range standardPeriods {
		if sp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        sp.info.name,
			Utilization: int(sp.data.Utilization),
			PeriodType:  sp.info.periodType,
		}
		if sp.data.ResetsAt != "" {
			if t, err := time.Parse(time.RFC3339, sp.data.ResetsAt); err == nil {
				p.ResetsAt = &t
			}
		}
		periods = append(periods, p)
	}

	modelPeriods := []struct {
		data      *UsagePeriodResponse
		modelName string
	}{
		{resp.SevenDaySonnet, "Sonnet"},
		{resp.SevenDayOpus, "Opus"},
		{resp.SevenDayHaiku, "Haiku"},
	}

	for _, mp := range modelPeriods {
		if mp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        mp.modelName,
			Utilization: int(mp.data.Utilization),
			PeriodType:  models.PeriodWeekly,
			Model:       strings.ToLower(mp.modelName),
		}
		if mp.data.ResetsAt != "" {
			if t, err := time.Parse(time.RFC3339, mp.data.ResetsAt); err == nil {
				p.ResetsAt = &t
			}
		}
		periods = append(periods, p)
	}

	var overage *models.OverageUsage
	if resp.ExtraUsage != nil && resp.ExtraUsage.IsEnabled {
		overage = &models.OverageUsage{
			Used:      resp.ExtraUsage.UsedCredits / 100.0,
			Limit:     resp.ExtraUsage.MonthlyLimit / 100.0,
			Currency:  "USD",
			IsEnabled: true,
		}
	}

	// Build identity from plan field or infer from usage features.
	plan := resp.Plan
	if plan == "" && resp.BillingType != "" {
		plan = resp.BillingType
	}
	if plan == "" {
		plan = inferClaudePlan(resp)
	}

	var identity *models.ProviderIdentity
	if plan != "" {
		identity = &models.ProviderIdentity{Plan: plan}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    "oauth",
	}
}

// APIKeyStrategy recognizes Anthropic API keys and preserves them for auth
// workflows. Claude consumer quota metrics still come from OAuth/session data.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) Name() string { return "apikey" }

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadAPIKey() != ""
}

func (s *APIKeyStrategy) Fetch(_ context.Context) (fetch.FetchResult, error) {
	key := s.loadAPIKey()
	if key == "" {
		return fetch.ResultFail("No API key found"), nil
	}

	if !strings.HasPrefix(key, "sk-ant-") {
		return fetch.ResultFatal("Invalid Anthropic API key format"), nil
	}

	return fetch.ResultFail("Anthropic API keys are configured, but claude.ai plan usage requires Claude OAuth/session credentials."), nil
}

func (s *APIKeyStrategy) loadAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); v != "" {
		return v
	}

	data, err := config.ReadCredential(config.CredentialPath("claude", "apikey"))
	if err != nil || data == nil {
		return ""
	}

	var key struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &key); err != nil {
		return ""
	}
	return strings.TrimSpace(key.APIKey)
}

// WebStrategy fetches Claude usage using a session cookie.
type WebStrategy struct {
	HTTPTimeout float64
}

func (s *WebStrategy) Name() string { return "web" }

func (s *WebStrategy) IsAvailable() bool {
	return s.loadSessionKey() != ""
}

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionKey := s.loadSessionKey()
	if sessionKey == "" {
		return fetch.ResultFail("No session key found"), nil
	}

	orgID := s.getOrgID(ctx, sessionKey)
	if orgID == "" {
		return fetch.ResultFail("Failed to get organization ID"), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	sessionCookie := httpclient.WithCookie("sessionKey", sessionKey)

	// Fetch usage
	usageURL := webBaseURL + "/" + orgID + "/usage"
	var usageResp WebUsageResponse
	resp, err := client.GetJSONCtx(ctx, usageURL, &usageResp, sessionCookie)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Session key expired or invalid"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid usage response: %v", resp.JSONErr)), nil
	}

	// Fetch overage
	var overage *models.OverageUsage
	overageURL := webBaseURL + "/" + orgID + "/overage_spend_limit"
	var overageResp WebOverageResponse
	oResp, err := client.GetJSONCtx(ctx, overageURL, &overageResp, sessionCookie)
	if err == nil && oResp.StatusCode == 200 && oResp.JSONErr == nil {
		overage = overageResp.ToOverageUsage()
	}

	snapshot := s.parseWebUsageResponse(usageResp, overage)
	return fetch.ResultOK(*snapshot), nil
}

func (s *WebStrategy) loadSessionKey() string {
	path := config.CredentialPath("claude", "session")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}

	value := ""
	var creds WebSessionCredentials
	if err := json.Unmarshal(data, &creds); err == nil {
		value = strings.TrimSpace(creds.SessionKey)
	} else {
		// Try raw string
		value = strings.TrimSpace(string(data))
	}

	if strings.HasPrefix(value, "sk-ant-sid01-") {
		return value
	}
	return ""
}

func (s *WebStrategy) getOrgID(ctx context.Context, sessionKey string) string {
	// Check cache
	if cached := config.LoadCachedOrgID("claude"); cached != "" {
		return cached
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var orgs []WebOrganization
	resp, err := client.GetJSONCtx(ctx, webBaseURL, &orgs,
		httpclient.WithCookie("sessionKey", sessionKey),
	)
	if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
		return ""
	}

	orgID := s.findChatOrgID(orgs)
	if orgID != "" {
		_ = config.CacheOrgID("claude", orgID)
	}
	return orgID
}

// findChatOrgID finds the first organization with "chat" capability,
// falling back to the first organization if none have it.
func (s *WebStrategy) findChatOrgID(orgs []WebOrganization) string {
	for _, org := range orgs {
		if org.HasCapability("chat") {
			if id := org.OrgID(); id != "" {
				return id
			}
		}
	}

	// Fallback to first org
	if len(orgs) > 0 {
		return orgs[0].OrgID()
	}
	return ""
}

func (s *WebStrategy) parseWebUsageResponse(resp WebUsageResponse, overage *models.OverageUsage) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	if resp.UsageLimit > 0 {
		utilization := int((resp.UsageAmount / resp.UsageLimit) * 100)
		var resetsAt *time.Time
		if resp.PeriodEnd != "" {
			if t, err := time.Parse(time.RFC3339, resp.PeriodEnd); err == nil {
				resetsAt = &t
			}
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Usage",
			Utilization: utilization,
			PeriodType:  models.PeriodDaily,
			ResetsAt:    resetsAt,
		})
	}

	var identity *models.ProviderIdentity
	if resp.Email != "" || resp.Organization != "" || resp.Plan != "" {
		identity = &models.ProviderIdentity{
			Email:        resp.Email,
			Organization: resp.Organization,
			Plan:         resp.Plan,
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    "web",
	}
}
