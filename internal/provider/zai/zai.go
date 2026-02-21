package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Zai struct{}

func (z Zai) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "zai",
		Name:        "Z.ai",
		Description: "Zhipu AI coding assistant",
		Homepage:    "https://z.ai",
	}
}

func (z Zai) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&BearerTokenStrategy{},
	}
}

func (z Zai) FetchStatus() models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

func init() {
	provider.Register(Zai{})
}

const (
	quotaURL = "https://api.z.ai/api/monitor/usage/quota/limit"
)

// BearerTokenStrategy fetches Z.ai usage using an API key or JWT bearer token.
type BearerTokenStrategy struct{}

func (s *BearerTokenStrategy) Name() string { return "bearer_token" }

func (s *BearerTokenStrategy) IsAvailable() bool {
	return s.loadToken() != ""
}

func (s *BearerTokenStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := s.loadToken()
	if token == "" {
		return fetch.ResultFail("No API key found. Set ZAI_API_KEY or use 'vibeusage key zai set'"), nil
	}

	return fetchQuota(ctx, token)
}

func (s *BearerTokenStrategy) loadToken() string {
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		return key
	}
	path := config.CredentialPath("zai", "apikey")
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
		httpclient.WithHeader("Accept-Language", "en-US,en"),
	}

	var quotaResp QuotaResponse
	resp, err := client.GetJSONCtx(ctx, quotaURL, &quotaResp, opts...)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 || quotaResp.Code == 1001 {
		return fetch.ResultFatal("Token invalid or expired. Use 'vibeusage auth zai' to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Z.ai API: %v", resp.JSONErr)), nil
	}
	if !quotaResp.Success || quotaResp.Data == nil {
		msg := quotaResp.Msg
		if msg == "" {
			msg = "API returned unsuccessful response"
		}
		return fetch.ResultFail(msg), nil
	}

	snapshot := parseQuotaResponse(quotaResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse Z.ai quota response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

// parseQuotaResponse converts the API response to a UsageSnapshot.
func parseQuotaResponse(resp QuotaResponse) *models.UsageSnapshot {
	if resp.Data == nil || len(resp.Data.Limits) == 0 {
		return nil
	}

	var periods []models.UsagePeriod

	for _, limit := range resp.Data.Limits {
		period := models.UsagePeriod{
			Name:        limit.DisplayName(),
			Utilization: clamp(limit.Percentage, 0, 100),
			PeriodType:  limit.PeriodType(),
			ResetsAt:    limit.ResetTime(),
		}
		periods = append(periods, period)
	}

	if len(periods) == 0 {
		return nil
	}

	var identity *models.ProviderIdentity
	if resp.Data.Level != "" {
		plan := PlanName(resp.Data.Level)
		if plan != "" {
			identity = &models.ProviderIdentity{Plan: plan}
		}
	}

	return &models.UsageSnapshot{
		Provider:  "zai",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Identity:  identity,
		Source:    "bearer_token",
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
