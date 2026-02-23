package minimax

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

type Minimax struct{}

func (m Minimax) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "minimax",
		Name:        "Minimax",
		Description: "Minimax AI coding assistant",
		Homepage:    "https://www.minimax.io",
	}
}

func (m Minimax) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&APIKeyStrategy{},
	}
}

func (m Minimax) FetchStatus() models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

// Auth returns the manual API key flow for Minimax.
func (m Minimax) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your Coding Plan API key from Minimax:\n" +
			"  1. Open https://platform.minimax.io/user-center/payment/coding-plan\n" +
			"  2. Copy your Coding Plan API key (starts with sk-cp-)\n" +
			"\n" +
			"Note: Standard API keys (sk-api-) won't work — you need a Coding Plan key.",
		Placeholder: "sk-cp-...",
		Validate:    ValidateCodingPlanKey,
		CredPath:    config.CredentialPath("minimax", "apikey"),
		JSONKey:     "api_key",
	}
}

func init() {
	provider.Register(Minimax{})
}

const (
	quotaURL = "https://platform.minimax.io/v1/api/openplatform/coding_plan/remains"
)

// APIKeyStrategy fetches Minimax usage using a coding plan API key (sk-cp-...).
type APIKeyStrategy struct{}

func (s *APIKeyStrategy) Name() string { return "api_key" }

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadToken() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := s.loadToken()
	if token == "" {
		return fetch.ResultFail("No API key found. Set MINIMAX_API_KEY or use 'vibeusage key minimax set'"), nil
	}

	return fetchQuota(ctx, token)
}

func (s *APIKeyStrategy) loadToken() string {
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		return key
	}
	path := config.CredentialPath("minimax", "apikey")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return creds.APIKey
}

// fetchQuota makes the API call and parses the response.
func fetchQuota(ctx context.Context, token string) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	opts := []httpclient.RequestOption{
		httpclient.WithBearer(token),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithHeader("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
		httpclient.WithHeader("Referer", "https://platform.minimax.io/"),
	}

	var planResp CodingPlanResponse
	resp, err := client.GetJSONCtx(ctx, quotaURL, &planResp, opts...)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fetch.ResultFatal("Token invalid or expired. Use 'vibeusage auth minimax' to re-authenticate."), nil
	}
	// Error responses use two shapes: flat (top-level status_code) or nested (base_resp.status_code).
	apiStatusCode := planResp.StatusCode
	if apiStatusCode == 0 {
		apiStatusCode = planResp.BaseResp.StatusCode
	}
	if apiStatusCode == 1004 {
		return fetch.ResultFatal("Invalid API key. Minimax requires a Coding Plan key (sk-cp-...). Use 'vibeusage auth minimax' to set one."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Minimax API: %v", resp.JSONErr)), nil
	}
	if apiStatusCode != 0 {
		msg := planResp.BaseResp.StatusMsg
		if msg == "" {
			msg = planResp.StatusMsg
		}
		if msg == "" {
			msg = fmt.Sprintf("API error: status_code=%d", apiStatusCode)
		}
		return fetch.ResultFail(msg), nil
	}

	snapshot := parseResponse(planResp)
	if snapshot == nil {
		return fetch.ResultFail("No model data in Minimax response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

// parseResponse converts the API response to a UsageSnapshot.
func parseResponse(resp CodingPlanResponse) *models.UsageSnapshot {
	if len(resp.ModelRemains) == 0 {
		return nil
	}

	// Build a summary period from the highest utilization across all models,
	// plus individual per-model periods.
	var periods []models.UsagePeriod
	maxUtil := 0
	var summaryReset *time.Time
	maxTotal := 0

	for _, m := range resp.ModelRemains {
		period := m.ToUsagePeriod()
		periods = append(periods, period)

		util := m.Utilization()
		if util > maxUtil {
			maxUtil = util
		}
		if m.CurrentIntervalTotalCount > maxTotal {
			maxTotal = m.CurrentIntervalTotalCount
		}
		if summaryReset == nil {
			summaryReset = m.ResetTime()
		}
	}

	// Prepend a summary period when there are multiple models.
	if len(periods) > 1 {
		summary := models.UsagePeriod{
			Name:        "Coding Plan",
			Utilization: maxUtil,
			PeriodType:  models.PeriodSession,
			ResetsAt:    summaryReset,
		}
		periods = append([]models.UsagePeriod{summary}, periods...)
	} else if len(periods) == 1 {
		// Single model: use "Coding Plan" as the name instead of model name.
		periods[0].Name = "Coding Plan"
	}

	var identity *models.ProviderIdentity
	if plan := InferPlan(maxTotal); plan != "" {
		identity = &models.ProviderIdentity{Plan: plan}
	}

	return &models.UsageSnapshot{
		Provider:  "minimax",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Identity:  identity,
		Source:    "api_key",
	}
}

// ValidateCodingPlanKey checks that the key has the expected sk-cp- prefix.
func ValidateCodingPlanKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if !strings.HasPrefix(key, "sk-cp-") {
		return fmt.Errorf("Minimax Coding Plan keys start with 'sk-cp-'. Standard API keys (sk-api-) won't work for usage tracking")
	}
	return nil
}
