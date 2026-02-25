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
		p.ResetsAt = models.ParseRFC3339Ptr(sp.data.ResetsAt)
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
		p.ResetsAt = models.ParseRFC3339Ptr(mp.data.ResetsAt)
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
