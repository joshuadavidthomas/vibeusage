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
	"unicode"

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

	accessToken, _ := creds["access_token"].(string)
	if accessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	// Check if token needs refresh
	if s.needsRefresh(creds) {
		refreshed := s.refreshToken(creds)
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
		accessToken, _ = creds["access_token"].(string)
	}

	// Fetch usage
	req, _ := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
		config.CredentialPath("claude", "oauth"),
		filepath.Join(home, ".claude", ".credentials.json"),
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

		// Handle Claude CLI format
		if nested, ok := creds["claudeAiOauth"].(map[string]any); ok {
			creds = convertCamelToSnake(nested)
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
	return time.Now().UTC().After(expiry)
}

func (s *OAuthStrategy) refreshToken(creds map[string]any) map[string]any {
	refreshToken, _ := creds["refresh_token"].(string)
	if refreshToken == "" {
		return nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
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
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	// Update expires_at
	if expiresIn, ok := data["expires_in"].(float64); ok {
		expiresAt := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
		data["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	// Save updated credentials
	content, _ := json.Marshal(data)
	_ = config.WriteCredential(config.CredentialPath("claude", "oauth"), content)

	return data
}

func (s *OAuthStrategy) parseUsageResponse(data map[string]any) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	// Standard periods
	periodMapping := map[string]struct {
		name       string
		periodType models.PeriodType
	}{
		"five_hour": {"Session (5h)", models.PeriodSession},
		"seven_day": {"All Models", models.PeriodWeekly},
		"monthly":   {"Monthly", models.PeriodMonthly},
	}

	for key, info := range periodMapping {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		periodData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		util, ok := periodData["utilization"].(float64)
		if !ok {
			continue
		}
		var resetsAt *time.Time
		if rStr, ok := periodData["resets_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, rStr); err == nil {
				resetsAt = &t
			}
		}
		periods = append(periods, models.UsagePeriod{
			Name:        info.name,
			Utilization: int(util),
			PeriodType:  info.periodType,
			ResetsAt:    resetsAt,
		})
	}

	// Model-specific periods
	modelPrefixes := map[string]string{
		"seven_day_sonnet": "Sonnet",
		"seven_day_opus":   "Opus",
		"seven_day_haiku":  "Haiku",
	}

	for key, modelName := range modelPrefixes {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		periodData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		util, ok := periodData["utilization"].(float64)
		if !ok {
			continue
		}
		var resetsAt *time.Time
		if rStr, ok := periodData["resets_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, rStr); err == nil {
				resetsAt = &t
			}
		}
		periods = append(periods, models.UsagePeriod{
			Name:        modelName,
			Utilization: int(util),
			PeriodType:  models.PeriodWeekly,
			ResetsAt:    resetsAt,
			Model:       strings.ToLower(modelName),
		})
	}

	// Overage
	var overage *models.OverageUsage
	if extraUsage, ok := data["extra_usage"].(map[string]any); ok {
		if isEnabled, _ := extraUsage["is_enabled"].(bool); isEnabled {
			used, _ := extraUsage["used_credits"].(float64)
			limit, _ := extraUsage["monthly_limit"].(float64)
			overage = &models.OverageUsage{
				Used:      used,
				Limit:     limit,
				Currency:  "USD",
				IsEnabled: true,
			}
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

func convertCamelToSnake(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		var snakeKey strings.Builder
		for i, r := range key {
			if unicode.IsUpper(r) {
				if i > 0 {
					snakeKey.WriteByte('_')
				}
				snakeKey.WriteRune(unicode.ToLower(r))
			} else {
				snakeKey.WriteRune(r)
			}
		}
		sk := snakeKey.String()

		// Convert expiresAt millisecond timestamp to ISO string
		if sk == "expires_at" {
			if ts, ok := value.(float64); ok {
				t := time.UnixMilli(int64(ts))
				value = t.UTC().Format(time.RFC3339)
			}
		}

		result[sk] = value
	}
	return result
}
