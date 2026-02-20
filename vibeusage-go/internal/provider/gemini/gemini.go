package gemini

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

type Gemini struct{}

func (g Gemini) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "gemini",
		Name:         "Gemini",
		Description:  "Google Gemini AI",
		Homepage:     "https://gemini.google.com",
		DashboardURL: "https://aistudio.google.com/app/usage",
	}
}

func (g Gemini) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&OAuthStrategy{},
		&APIKeyStrategy{},
	}
}

func (g Gemini) FetchStatus() models.ProviderStatus {
	return fetchGeminiStatus()
}

func init() {
	provider.Register(Gemini{})
}

// OAuth Strategy
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

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		config.CredentialPath("gemini", "oauth"),
		filepath.Join(home, ".gemini", "oauth_creds.json"),
	}
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

	quotaData, userTierData := s.fetchQuotaData(accessToken)
	if quotaData == nil {
		return fetch.ResultFail("Failed to fetch quota data"), nil
	}

	snapshot := s.parseUsageResponse(quotaData, userTierData)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
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

		// Handle Gemini CLI nested format
		if installed, ok := creds["installed"].(map[string]any); ok {
			return convertGeminiCLIFormat(installed)
		}
		if _, ok := creds["token"]; ok {
			if _, ok2 := creds["access_token"]; !ok2 {
				return map[string]any{
					"access_token":  creds["token"],
					"refresh_token": creds["refresh_token"],
					"expires_at":    creds["expiry_date"],
				}
			}
		}
		if _, ok := creds["access_token"]; ok {
			return creds
		}
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
	return time.Now().UTC().Add(5 * time.Minute).After(expiry)
}

func (s *OAuthStrategy) refreshToken(creds map[string]any) map[string]any {
	refreshToken, _ := creds["refresh_token"].(string)
	if refreshToken == "" {
		return nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {"77185425430.apps.googleusercontent.com"},
		"client_secret": {"GOCSPX-1mdrl61JR9D-iFHq4QPq2mJGwZv"},
	}

	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
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
	_ = config.WriteCredential(config.CredentialPath("gemini", "oauth"), content)

	return data
}

func (s *OAuthStrategy) fetchQuotaData(accessToken string) (map[string]any, map[string]any) {
	client := &http.Client{Timeout: 30 * time.Second}
	var quotaData, userData map[string]any

	// Quota
	qReq, _ := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota", strings.NewReader("{}"))
	qReq.Header.Set("Authorization", "Bearer "+accessToken)
	qReq.Header.Set("Content-Type", "application/json")
	if qResp, err := client.Do(qReq); err == nil && qResp.StatusCode == 200 {
		body, _ := io.ReadAll(qResp.Body)
		qResp.Body.Close()
		json.Unmarshal(body, &quotaData)
	}

	// User tier
	tReq, _ := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", strings.NewReader("{}"))
	tReq.Header.Set("Authorization", "Bearer "+accessToken)
	tReq.Header.Set("Content-Type", "application/json")
	if tResp, err := client.Do(tReq); err == nil && tResp.StatusCode == 200 {
		body, _ := io.ReadAll(tResp.Body)
		tResp.Body.Close()
		json.Unmarshal(body, &userData)
	}

	return quotaData, userData
}

func (s *OAuthStrategy) parseUsageResponse(quotaData, userData map[string]any) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	if buckets, ok := quotaData["quota_buckets"].([]any); ok {
		for _, b := range buckets {
			bucket, ok := b.(map[string]any)
			if !ok {
				continue
			}
			remainingFraction := 1.0
			if rf, ok := bucket["remaining_fraction"].(float64); ok {
				remainingFraction = rf
			}
			utilization := int((1 - remainingFraction) * 100)
			modelID, _ := bucket["model_id"].(string)
			modelName := modelID
			if idx := strings.LastIndex(modelID, "/"); idx >= 0 {
				modelName = modelID[idx+1:]
			}

			var resetsAt *time.Time
			if resetStr, ok := bucket["reset_time"].(string); ok {
				if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
					resetsAt = &t
				}
			}

			displayName := strutil.TitleCase(strings.ReplaceAll(strings.ReplaceAll(modelName, "-", " "), "_", " "))
			periods = append(periods, models.UsagePeriod{
				Name:        displayName,
				Utilization: utilization,
				PeriodType:  models.PeriodDaily,
				ResetsAt:    resetsAt,
				Model:       modelName,
			})
		}
	}

	if len(periods) == 0 {
		tomorrow := nextMidnightUTC()
		periods = append(periods, models.UsagePeriod{
			Name:        "Daily",
			Utilization: 0,
			PeriodType:  models.PeriodDaily,
			ResetsAt:    &tomorrow,
		})
	}

	var identity *models.ProviderIdentity
	if userData != nil {
		if tier, ok := userData["user_tier"].(string); ok {
			identity = &models.ProviderIdentity{Plan: tier}
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "gemini",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    "oauth",
	}
}

// API Key Strategy
type APIKeyStrategy struct{}

func (s *APIKeyStrategy) Name() string { return "api_key" }

func (s *APIKeyStrategy) IsAvailable() bool {
	if os.Getenv("GEMINI_API_KEY") != "" {
		return true
	}
	paths := []string{
		config.CredentialPath("gemini", "api_key") + ".txt",
		config.CredentialPath("gemini", "api_key"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func (s *APIKeyStrategy) Fetch() (fetch.FetchResult, error) {
	apiKey := s.loadAPIKey()
	if apiKey == "" {
		return fetch.ResultFail("No API key found. Set GEMINI_API_KEY or use 'vibeusage key set gemini'"), nil
	}

	// Validate key by fetching models
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models?key="+apiKey, nil)

	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("API key is invalid or expired"), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFatal("API key does not have access to Generative Language API"), nil
	}
	if resp.StatusCode == 429 {
		return fetch.ResultFatal("Rate limit exceeded"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFatal("Failed to validate API key: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	json.Unmarshal(body, &data)

	modelCount := 0
	if modelsSlice, ok := data["models"].([]any); ok {
		modelCount = len(modelsSlice)
	}

	tomorrow := nextMidnightUTC()
	snapshot := models.UsageSnapshot{
		Provider:  "gemini",
		FetchedAt: time.Now().UTC(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Daily",
				Utilization: 0,
				PeriodType:  models.PeriodDaily,
				ResetsAt:    &tomorrow,
			},
		},
		Identity: &models.ProviderIdentity{
			Plan:         "API Key",
			Organization: "Available models: " + strconv.Itoa(modelCount),
		},
		Source: "api_key",
	}

	return fetch.ResultOK(snapshot), nil
}

func (s *APIKeyStrategy) loadAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	// Check credential files
	for _, suffix := range []string{".txt", ".json"} {
		path := filepath.Join(config.CredentialsDir(), "gemini", "api_key"+suffix)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if suffix == ".json" {
			var obj map[string]any
			if json.Unmarshal(data, &obj) == nil {
				for _, k := range []string{"api_key", "key"} {
					if v, ok := obj[k].(string); ok {
						return v
					}
				}
			}
		}
		return strings.TrimSpace(string(data))
	}
	return ""
}

// Status
func fetchGeminiStatus() models.ProviderStatus {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://www.google.com/appsstatus/dashboard/incidents.json")
	if err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	var incidents []map[string]any
	if err := json.Unmarshal(body, &incidents); err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	geminiKeywords := []string{"gemini", "ai studio", "aistudio", "generative ai", "vertex ai", "cloud code"}

	for _, incident := range incidents {
		if _, ok := incident["end_time"]; ok {
			continue // ended
		}
		title, _ := incident["title"].(string)
		titleLower := strings.ToLower(title)

		for _, keyword := range geminiKeywords {
			if strings.Contains(titleLower, keyword) {
				severity, _ := incident["severity"].(string)
				level := severityToLevel(severity)
				now := time.Now().UTC()
				return models.ProviderStatus{
					Level:       level,
					Description: title,
					UpdatedAt:   &now,
				}
			}
		}
	}

	now := time.Now().UTC()
	return models.ProviderStatus{
		Level:       models.StatusOperational,
		Description: "All systems operational",
		UpdatedAt:   &now,
	}
}

func severityToLevel(severity string) models.StatusLevel {
	switch strings.ToLower(severity) {
	case "low", "medium":
		return models.StatusDegraded
	case "high":
		return models.StatusPartialOutage
	case "critical", "severe":
		return models.StatusMajorOutage
	default:
		return models.StatusDegraded
	}
}

func convertGeminiCLIFormat(data map[string]any) map[string]any {
	accessToken, _ := data["token"].(string)
	if accessToken == "" {
		accessToken, _ = data["access_token"].(string)
	}
	if accessToken == "" {
		return nil
	}
	result := map[string]any{
		"access_token":  accessToken,
		"refresh_token": data["refresh_token"],
	}
	if expiry := data["expiry_date"]; expiry != nil {
		switch v := expiry.(type) {
		case float64:
			t := time.UnixMilli(int64(v)).UTC()
			result["expires_at"] = t.Format(time.RFC3339)
		case string:
			result["expires_at"] = v
		}
	}
	return result
}

func nextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
