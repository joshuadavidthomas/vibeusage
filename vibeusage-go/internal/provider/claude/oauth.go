package claude

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

type OAuthStrategy struct{}

func (s *OAuthStrategy) Name() string { return "oauth" }

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func (s *OAuthStrategy) Fetch() (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found"), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(creds)
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
	}

	req, _ := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fetch.ResultFail("OAuth token expired or invalid"), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFail("Not authorized to access usage"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail("Usage request failed: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var usageResp OAuthUsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return fetch.ResultFail("Invalid response from usage endpoint"), nil
	}

	snapshot := s.parseOAuthUsageResponse(usageResp)

	return fetch.ResultOK(*snapshot), nil
}

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		config.CredentialPath("claude", "oauth"),
		filepath.Join(home, ".claude", ".credentials.json"),
	}
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
	return nil
}

func (s *OAuthStrategy) refreshToken(creds *OAuthCredentials) *OAuthCredentials {
	if creds.RefreshToken == "" {
		return nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {creds.RefreshToken},
	}

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
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
			Used:      resp.ExtraUsage.UsedCredits,
			Limit:     resp.ExtraUsage.MonthlyLimit,
			Currency:  "USD",
			IsEnabled: true,
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Source:    "oauth",
	}
}
