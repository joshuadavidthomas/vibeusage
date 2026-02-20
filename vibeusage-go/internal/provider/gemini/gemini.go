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

	quotaResp, codeAssistResp := s.fetchQuotaData(creds.AccessToken)
	if quotaResp == nil {
		return fetch.ResultFail("Failed to fetch quota data"), nil
	}

	snapshot := s.parseTypedQuotaResponse(*quotaResp, codeAssistResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *OAuthStrategy) loadCredentials() *OAuthCredentials {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}
		var cliCreds GeminiCLICredentials
		if err := json.Unmarshal(data, &cliCreds); err != nil {
			continue
		}
		if creds := cliCreds.EffectiveCredentials(); creds != nil {
			return creds
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
		"client_id":     {"77185425430.apps.googleusercontent.com"},
		"client_secret": {"GOCSPX-1mdrl61JR9D-iFHq4QPq2mJGwZv"},
	}

	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
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

	updated := &OAuthCredentials{
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
	_ = config.WriteCredential(config.CredentialPath("gemini", "oauth"), content)

	return updated
}

func (s *OAuthStrategy) fetchQuotaData(accessToken string) (*QuotaResponse, *CodeAssistResponse) {
	client := &http.Client{Timeout: 30 * time.Second}
	var quotaResp *QuotaResponse
	var codeAssistResp *CodeAssistResponse

	// Quota
	qReq, _ := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota", strings.NewReader("{}"))
	qReq.Header.Set("Authorization", "Bearer "+accessToken)
	qReq.Header.Set("Content-Type", "application/json")
	if qHTTPResp, err := client.Do(qReq); err == nil {
		defer qHTTPResp.Body.Close()
		if qHTTPResp.StatusCode == 200 {
			body, _ := io.ReadAll(qHTTPResp.Body)
			var qr QuotaResponse
			if json.Unmarshal(body, &qr) == nil {
				quotaResp = &qr
			}
		}
	}

	// User tier
	tReq, _ := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", strings.NewReader("{}"))
	tReq.Header.Set("Authorization", "Bearer "+accessToken)
	tReq.Header.Set("Content-Type", "application/json")
	if tHTTPResp, err := client.Do(tReq); err == nil {
		defer tHTTPResp.Body.Close()
		if tHTTPResp.StatusCode == 200 {
			body, _ := io.ReadAll(tHTTPResp.Body)
			var ca CodeAssistResponse
			if json.Unmarshal(body, &ca) == nil {
				codeAssistResp = &ca
			}
		}
	}

	return quotaResp, codeAssistResp
}

func (s *OAuthStrategy) parseTypedQuotaResponse(quotaResp QuotaResponse, codeAssistResp *CodeAssistResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	for _, bucket := range quotaResp.QuotaBuckets {
		modelName := bucket.ModelID
		if idx := strings.LastIndex(bucket.ModelID, "/"); idx >= 0 {
			modelName = bucket.ModelID[idx+1:]
		}

		displayName := strutil.TitleCase(strings.ReplaceAll(strings.ReplaceAll(modelName, "-", " "), "_", " "))
		periods = append(periods, models.UsagePeriod{
			Name:        displayName,
			Utilization: bucket.Utilization(),
			PeriodType:  models.PeriodDaily,
			ResetsAt:    bucket.ResetTimeUTC(),
			Model:       modelName,
		})
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
	if codeAssistResp != nil && codeAssistResp.UserTier != "" {
		identity = &models.ProviderIdentity{Plan: codeAssistResp.UserTier}
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
	credDir := filepath.Join(config.CredentialsDir(), "gemini")
	for _, name := range []string{"api_key.txt", "api_key.json"} {
		if _, err := os.Stat(filepath.Join(credDir, name)); err == nil {
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
	var modelsResp ModelsResponse
	json.Unmarshal(body, &modelsResp)

	modelCount := len(modelsResp.Models)

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

// apiKeyFile represents JSON credential files that contain an API key.
type apiKeyFile struct {
	APIKey string `json:"api_key,omitempty"`
	Key    string `json:"key,omitempty"`
}

func (a *apiKeyFile) effectiveKey() string {
	if a.APIKey != "" {
		return a.APIKey
	}
	return a.Key
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
			var keyFile apiKeyFile
			if json.Unmarshal(data, &keyFile) == nil {
				if key := keyFile.effectiveKey(); key != "" {
					return key
				}
			}
			continue // Don't return raw JSON as an API key
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

	var incidents []googleIncident
	if err := json.Unmarshal(body, &incidents); err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	geminiKeywords := []string{"gemini", "ai studio", "aistudio", "generative ai", "vertex ai", "cloud code"}

	for _, incident := range incidents {
		if incident.EndTime != "" {
			continue // ended
		}
		titleLower := strings.ToLower(incident.Title)

		for _, keyword := range geminiKeywords {
			if strings.Contains(titleLower, keyword) {
				level := severityToLevel(incident.Severity)
				now := time.Now().UTC()
				return models.ProviderStatus{
					Level:       level,
					Description: incident.Title,
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

func nextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
