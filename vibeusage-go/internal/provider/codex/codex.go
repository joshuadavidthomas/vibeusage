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

	accessToken, _ := creds["access_token"].(string)
	if accessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if s.needsRefresh(creds) {
		refreshed := s.refreshToken(creds)
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
		accessToken, _ = creds["access_token"].(string)
	}

	usageURL := s.getUsageURL()

	req, _ := http.NewRequest("GET", usageURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return fetch.ResultFail("Invalid response from usage endpoint"), nil
	}

	snapshot := s.parseUsageResponse(data)
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

func (s *OAuthStrategy) loadCredentials() map[string]any {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}
		var creds map[string]any
		if err := json.Unmarshal(data, &creds); err != nil {
			continue
		}
		// Handle Codex CLI nested tokens
		if tokens, ok := creds["tokens"].(map[string]any); ok {
			creds = tokens
		}
		return creds
	}
	return nil
}

func (s *OAuthStrategy) needsRefresh(creds map[string]any) bool {
	expiresAt, ok := creds["expires_at"].(string)
	if !ok {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	threshold := time.Now().UTC().AddDate(0, 0, 8)
	return threshold.After(expiry)
}

func (s *OAuthStrategy) refreshToken(creds map[string]any) map[string]any {
	refreshToken, _ := creds["refresh_token"].(string)
	if refreshToken == "" {
		return nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {"app_EMoamEEZ73f0CkXaXp7hrann"},
	}

	req, _ := http.NewRequest("POST", "https://auth.openai.com/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	if expiresIn, ok := data["expires_in"].(float64); ok {
		data["expires_at"] = time.Now().UTC().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339)
	}
	if _, ok := data["refresh_token"]; !ok {
		data["refresh_token"] = refreshToken
	}

	content, _ := json.Marshal(data)
	_ = config.WriteCredential(config.CredentialPath("codex", "oauth"), content)

	return data
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

func (s *OAuthStrategy) parseUsageResponse(data map[string]any) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	rateLimits, _ := data["rate_limit"].(map[string]any)
	if rateLimits == nil {
		rateLimits, _ = data["rate_limits"].(map[string]any)
	}

	// Primary (session)
	primary := getNestedMap(rateLimits, "primary_window", "primary")
	if primary != nil {
		util := int(getFloat(primary, "used_percent"))
		var resetsAt *time.Time
		if ts := getFloat(primary, "reset_at", "reset_timestamp"); ts > 0 {
			t := time.Unix(int64(ts), 0).UTC()
			resetsAt = &t
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Session",
			Utilization: util,
			PeriodType:  models.PeriodSession,
			ResetsAt:    resetsAt,
		})
	}

	// Secondary (weekly)
	secondary := getNestedMap(rateLimits, "secondary_window", "secondary")
	if secondary != nil {
		util := int(getFloat(secondary, "used_percent"))
		var resetsAt *time.Time
		if ts := getFloat(secondary, "reset_at", "reset_timestamp"); ts > 0 {
			t := time.Unix(int64(ts), 0).UTC()
			resetsAt = &t
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Weekly",
			Utilization: util,
			PeriodType:  models.PeriodWeekly,
			ResetsAt:    resetsAt,
		})
	}

	if len(periods) == 0 {
		return nil
	}

	// Credits
	var overage *models.OverageUsage
	if credits, ok := data["credits"].(map[string]any); ok {
		if hasCredits, _ := credits["has_credits"].(bool); hasCredits {
			balance, _ := credits["balance"].(float64)
			overage = &models.OverageUsage{
				Used:      0,
				Limit:     balance,
				Currency:  "credits",
				IsEnabled: true,
			}
		}
	}

	var identity *models.ProviderIdentity
	if planType, ok := data["plan_type"].(string); ok {
		identity = &models.ProviderIdentity{Plan: planType}
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

func getNestedMap(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		if v, ok := m[k].(map[string]any); ok {
			return v
		}
	}
	return nil
}

func getFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return v
		}
	}
	return 0
}
