package warp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Warp struct{}

func (w Warp) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "warp",
		Name:        "Warp",
		Description: "Warp terminal AI",
		Homepage:    "https://warp.dev",
	}
}

func (w Warp) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{EnvVars: []string{"WARP_API_KEY", "WARP_TOKEN"}}
}

func (w Warp) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&APIKeyStrategy{HTTPTimeout: timeout}}
}

func (w Warp) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

func (w Warp) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your Warp API token:\n" +
			"  1. Open Warp settings\n" +
			"  2. Copy your token (starts with wk-)",
		Placeholder: "wk-...",
		Validate:    provider.ValidatePrefix("wk-"),
		CredPath:    config.CredentialPath("warp", "apikey"),
		JSONKey:     "api_key",
	}
}

func init() {
	provider.Register(Warp{})
}

var requestLimitURL = "https://app.warp.dev/graphql/v2?op=GetRequestLimitInfo"

// APIKeyStrategy fetches Warp usage via GraphQL.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

var warpAPIKey = provider.APIKeySource{
	EnvVars:  []string{"WARP_API_KEY", "WARP_TOKEN"},
	CredPath: config.CredentialPath("warp", "apikey"),
	JSONKeys: []string{"api_key", "token"},
}

func (s *APIKeyStrategy) IsAvailable() bool {
	return warpAPIKey.Load() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := warpAPIKey.Load()
	if token == "" {
		return fetch.ResultFail("No API key found. Set WARP_API_KEY or use 'vibeusage key warp set'"), nil
	}
	return fetchUsage(ctx, token, s.HTTPTimeout)
}

func fetchUsage(ctx context.Context, token string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	payload := GraphQLRequest{
		OperationName: "GetRequestLimitInfo",
		Query:         warpUsageQuery,
		Variables:     map[string]any{},
	}

	headers := []httpclient.RequestOption{
		httpclient.WithBearer(token),
		httpclient.WithHeader("x-warp-client-id", "vibeusage"),
		httpclient.WithHeader("x-warp-os", "linux"),
		httpclient.WithHeader("User-Agent", "warp/0 vibeusage/1"),
	}

	var gqlResp GraphQLResponse
	resp, err := client.PostJSONCtx(ctx, requestLimitURL, payload, &gqlResp, headers...)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if r := provider.CheckResponse(resp, "warp", "Warp"); r != nil {
		return *r, nil
	}

	snapshot, err := parseUsageSnapshot(gqlResp)
	if err != nil {
		if isAuthGraphQLError(err) {
			return fetch.ResultFatal("Token invalid or expired. Run `vibeusage auth warp` to re-authenticate."), nil
		}
		return fetch.ResultFail("Failed to parse Warp usage: " + err.Error()), nil
	}
	return fetch.ResultOK(*snapshot), nil
}

const warpUsageQuery = `query GetRequestLimitInfo { requestLimitInfo { requestLimit requestsUsed nextRefreshTime isUnlimited bonusCredits { used total nextRefreshTime } } }`

type GraphQLRequest struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
}

type GraphQLResponse struct {
	Data   *GraphQLData   `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type GraphQLData struct {
	RequestLimitInfo *RequestLimitInfo `json:"requestLimitInfo"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type RequestLimitInfo struct {
	RequestLimit    int           `json:"requestLimit"`
	RequestsUsed    int           `json:"requestsUsed"`
	NextRefreshTime string        `json:"nextRefreshTime"`
	IsUnlimited     bool          `json:"isUnlimited"`
	BonusCredits    *BonusCredits `json:"bonusCredits,omitempty"`
}

type BonusCredits struct {
	Used            int    `json:"used"`
	Total           int    `json:"total"`
	NextRefreshTime string `json:"nextRefreshTime"`
}

func parseUsageSnapshot(resp GraphQLResponse) (*models.UsageSnapshot, error) {
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	if resp.Data == nil || resp.Data.RequestLimitInfo == nil {
		return nil, fmt.Errorf("missing requestLimitInfo")
	}
	info := resp.Data.RequestLimitInfo

	utilization := 0
	if !info.IsUnlimited && info.RequestLimit > 0 {
		utilization = models.ClampPct((info.RequestsUsed * 100) / info.RequestLimit)
	}
	primary := models.UsagePeriod{
		Name:        "Monthly Credits",
		Utilization: utilization,
		PeriodType:  models.PeriodMonthly,
		ResetsAt:    models.ParseRFC3339Ptr(info.NextRefreshTime),
	}

	periods := []models.UsagePeriod{primary}
	if info.BonusCredits != nil && info.BonusCredits.Total > 0 {
		bonusUtil := models.ClampPct((info.BonusCredits.Used * 100) / info.BonusCredits.Total)
		periods = append(periods, models.UsagePeriod{
			Name:        "Bonus Credits",
			Utilization: bonusUtil,
			PeriodType:  models.PeriodMonthly,
			ResetsAt:    models.ParseRFC3339Ptr(info.BonusCredits.NextRefreshTime),
		})
	}

	return &models.UsageSnapshot{
		Provider:  "warp",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Source:    "api_key",
	}, nil
}

func isAuthGraphQLError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unauthorized") || strings.Contains(s, "forbidden") || strings.Contains(s, "auth")
}
