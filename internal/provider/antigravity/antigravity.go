package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

type Antigravity struct{}

func (a Antigravity) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "antigravity",
		Name:         "Antigravity",
		Description:  "Google Antigravity AI IDE",
		Homepage:     "https://antigravity.google",
		DashboardURL: "https://one.google.com/ai",
	}
}

func (a Antigravity) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&OAuthStrategy{},
	}
}

func (a Antigravity) FetchStatus() models.ProviderStatus {
	return fetchAntigravityStatus()
}

func init() {
	provider.Register(Antigravity{})
}

const (
	// OAuth client credentials from the Antigravity IDE.
	antigravityClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret = "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"

	googleTokenURL    = "https://oauth2.googleapis.com/token"
	quotaURL          = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	codeAssistURL     = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	googleIncidentURL = "https://www.google.com/appsstatus/dashboard/incidents.json"
)

// OAuthStrategy fetches Antigravity quota using Google OAuth credentials.
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
		config.CredentialPath("antigravity", "oauth"),
		filepath.Join(home, ".config", "Antigravity", "credentials.json"),
	}
}

func (s *OAuthStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found"), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(ctx, creds)
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
	}

	quotaResp, codeAssistResp := s.fetchQuotaData(ctx, creds.AccessToken)
	if quotaResp == nil {
		return fetch.ResultFail("Failed to fetch quota data"), nil
	}

	snapshot := s.parseQuotaResponse(*quotaResp, codeAssistResp)
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

		// Try the Antigravity credential format
		var agCreds AntigravityCredentials
		if err := json.Unmarshal(data, &agCreds); err == nil {
			if creds := agCreds.ToOAuthCredentials(); creds != nil {
				return creds
			}
		}

		// Try direct OAuth credentials format
		var oauthCreds OAuthCredentials
		if err := json.Unmarshal(data, &oauthCreds); err == nil && oauthCreds.AccessToken != "" {
			return &oauthCreds
		}
	}
	return nil
}

func (s *OAuthStrategy) refreshToken(ctx context.Context, creds *OAuthCredentials) *OAuthCredentials {
	if creds.RefreshToken == "" {
		return nil
	}

	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	var tokenResp TokenResponse
	resp, err := client.PostFormCtx(ctx, googleTokenURL,
		map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": creds.RefreshToken,
			"client_id":     antigravityClientID,
			"client_secret": antigravityClientSecret,
		},
		&tokenResp,
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
		updated.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	// Preserve refresh token if the server didn't issue a new one
	if updated.RefreshToken == "" {
		updated.RefreshToken = creds.RefreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath("antigravity", "oauth"), content)

	return updated
}

func (s *OAuthStrategy) fetchQuotaData(ctx context.Context, accessToken string) (*QuotaResponse, *CodeAssistResponse) {
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	bearer := httpclient.WithBearer(accessToken)
	var quotaResp *QuotaResponse
	var codeAssistResp *CodeAssistResponse

	// Quota — Antigravity requires IDE metadata in the request body
	reqBody := QuotaRequest{
		Metadata: QuotaRequestMetadata{
			IDEType:    "ANTIGRAVITY",
			Platform:   "PLATFORM_UNSPECIFIED",
			PluginType: "GEMINI",
		},
	}

	var qr QuotaResponse
	qResp, err := client.PostJSONCtx(ctx, quotaURL, reqBody, &qr, bearer)
	if err == nil && qResp.StatusCode == 200 && qResp.JSONErr == nil {
		quotaResp = &qr
	}

	// User tier
	var ca CodeAssistResponse
	tResp, err := client.PostJSONCtx(ctx, codeAssistURL,
		json.RawMessage("{}"), &ca, bearer,
	)
	if err == nil && tResp.StatusCode == 200 && tResp.JSONErr == nil {
		codeAssistResp = &ca
	}

	return quotaResp, codeAssistResp
}

// periodTypeForTier returns the appropriate period type based on the user's plan.
// Free tier uses weekly refresh cycles; paid tiers (Pro/Ultra) use 5-hour sessions.
func periodTypeForTier(tier string) models.PeriodType {
	switch strings.ToLower(tier) {
	case "free", "":
		return models.PeriodWeekly
	default:
		return models.PeriodSession
	}
}

func (s *OAuthStrategy) parseQuotaResponse(quotaResp QuotaResponse, codeAssistResp *CodeAssistResponse) *models.UsageSnapshot {
	var tier string
	if codeAssistResp != nil {
		tier = codeAssistResp.UserTier
	}
	periodType := periodTypeForTier(tier)

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
			PeriodType:  periodType,
			ResetsAt:    bucket.ResetTimeUTC(),
			Model:       modelName,
		})
	}

	if len(periods) == 0 {
		periods = append(periods, models.UsagePeriod{
			Name:        "Usage",
			Utilization: 0,
			PeriodType:  periodType,
		})
	}

	var identity *models.ProviderIdentity
	if tier != "" {
		identity = &models.ProviderIdentity{Plan: tier}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "antigravity",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    "oauth",
	}
}

// Status
func fetchAntigravityStatus() models.ProviderStatus {
	client := httpclient.NewWithTimeout(10 * time.Second)
	var incidents []googleIncident
	resp, err := client.GetJSON(googleIncidentURL, &incidents)
	if err != nil || resp.JSONErr != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	keywords := []string{"antigravity", "gemini", "cloud code", "generative ai", "ai studio"}

	for _, incident := range incidents {
		if incident.EndTime != "" {
			continue
		}
		titleLower := strings.ToLower(incident.Title)

		for _, keyword := range keywords {
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

type googleIncident struct {
	Title    string `json:"title,omitempty"`
	Severity string `json:"severity,omitempty"`
	EndTime  string `json:"end_time,omitempty"`
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
