package codex

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

func (c Codex) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{&OAuthStrategy{}}
}

func (c Codex) FetchStatus() models.ProviderStatus {
	return provider.FetchStatuspageStatus("https://status.openai.com/api/v2/status.json")
}

func init() {
	provider.Register(Codex{})
}

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

	usageURL := s.getUsageURL()

	req, _ := http.NewRequest("GET", usageURL, nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("OAuth token expired or invalid"), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFail("Not authorized. Account may not have ChatGPT Plus/Pro subscription."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail("Usage request failed: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var usageResp UsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return fetch.ResultFail("Invalid response from usage endpoint"), nil
	}

	snapshot := s.parseTypedUsageResponse(usageResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		config.CredentialPath("codex", "oauth"),
		filepath.Join(home, ".codex", "auth.json"),
	}
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
	return nil
}

func (s *OAuthStrategy) refreshToken(creds *Credentials) *Credentials {
	if creds.RefreshToken == "" {
		return nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {creds.RefreshToken},
		"client_id":     {"app_EMoamEEZ73f0CkXaXp7hrann"},
	}

	req, _ := http.NewRequest("POST", "https://auth.openai.com/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil
	}

	updated := &Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}

	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	// Preserve refresh token if the server didn't issue a new one
	if updated.RefreshToken == "" {
		updated.RefreshToken = creds.RefreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath("codex", "oauth"), content)

	return updated
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
	return "https://chatgpt.com/backend-api/wham/usage"
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
			Limit:     resp.Credits.Balance,
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
