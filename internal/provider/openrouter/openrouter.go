package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type OpenRouter struct{}

func (o OpenRouter) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "openrouter",
		Name:        "OpenRouter",
		Description: "OpenRouter unified model gateway",
		Homepage:    "https://openrouter.ai",
	}
}

func (o OpenRouter) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{EnvVars: []string{"OPENROUTER_API_KEY"}}
}

func (o OpenRouter) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&APIKeyStrategy{HTTPTimeout: timeout}}
}

func (o OpenRouter) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

func (o OpenRouter) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your OpenRouter API key:\n" +
			"  1. Open https://openrouter.ai/settings/keys\n" +
			"  2. Create or copy an API key",
		Placeholder: "sk-or-...",
		Validate:    provider.ValidateNotEmpty,
		CredPath:    config.CredentialPath("openrouter", "apikey"),
		JSONKey:     "api_key",
	}
}

func init() {
	provider.Register(OpenRouter{})
}

var creditsURL = "https://openrouter.ai/api/v1/credits"

// APIKeyStrategy fetches OpenRouter usage from the credits endpoint.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) Name() string { return "api_key" }

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadToken() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := s.loadToken()
	if token == "" {
		return fetch.ResultFail("No API key found. Set OPENROUTER_API_KEY or use 'vibeusage key openrouter set'"), nil
	}
	return fetchCredits(ctx, token, s.HTTPTimeout)
}

func (s *APIKeyStrategy) loadToken() string {
	if key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); key != "" {
		return key
	}
	data, err := config.ReadCredential(config.CredentialPath("openrouter", "apikey"))
	if err != nil || data == nil {
		return ""
	}
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return strings.TrimSpace(creds.APIKey)
}

func fetchCredits(ctx context.Context, token string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	var parsed CreditsResponse
	resp, err := client.GetJSONCtx(ctx, creditsURL, &parsed, httpclient.WithBearer(token))
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fetch.ResultFatal("API key is invalid or expired. Run `vibeusage auth openrouter` to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("OpenRouter credits request failed: HTTP %d (%s)", resp.StatusCode, summarizeBody(resp.Body))), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from OpenRouter API: %v", resp.JSONErr)), nil
	}

	snapshot, err := parseCreditsSnapshot(parsed)
	if err != nil {
		return fetch.ResultFail("Failed to parse OpenRouter credits response: " + err.Error()), nil
	}
	return fetch.ResultOK(*snapshot), nil
}

type CreditsResponse struct {
	Data CreditsData `json:"data"`
}

type CreditsData struct {
	TotalCredits float64 `json:"total_credits"`
	TotalUsage   float64 `json:"total_usage"`
}

func parseCreditsSnapshot(resp CreditsResponse) (*models.UsageSnapshot, error) {
	total := resp.Data.TotalCredits
	used := resp.Data.TotalUsage
	if total < 0 {
		total = 0
	}
	if used < 0 {
		used = 0
	}

	utilization := 0
	if total > 0 {
		utilization = int((used / total) * 100)
		if utilization < 0 {
			utilization = 0
		}
		if utilization > 100 {
			utilization = 100
		}
	}

	now := time.Now().UTC()
	snapshot := &models.UsageSnapshot{
		Provider:  "openrouter",
		FetchedAt: now,
		Periods: []models.UsagePeriod{
			{
				Name:        "Credits",
				Utilization: utilization,
				PeriodType:  models.PeriodMonthly,
			},
		},
		Overage: &models.OverageUsage{
			Used:      used,
			Limit:     total,
			Currency:  "USD",
			IsEnabled: true,
		},
		Source: "api_key",
	}
	return snapshot, nil
}

func summarizeBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "empty body"
	}
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
