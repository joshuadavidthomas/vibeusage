package warp

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

func (s *APIKeyStrategy) Name() string { return "api_key" }

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadToken() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := s.loadToken()
	if token == "" {
		return fetch.ResultFail("No API key found. Set WARP_API_KEY or use 'vibeusage key warp set'"), nil
	}
	return fetchUsage(ctx, token, s.HTTPTimeout)
}

func (s *APIKeyStrategy) loadToken() string {
	for _, envVar := range []string{"WARP_API_KEY", "WARP_TOKEN"} {
		if key := strings.TrimSpace(os.Getenv(envVar)); key != "" {
			return key
		}
	}
	data, err := config.ReadCredential(config.CredentialPath("warp", "apikey"))
	if err != nil || data == nil {
		return ""
	}
	var creds struct {
		APIKey string `json:"api_key"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	if strings.TrimSpace(creds.APIKey) != "" {
		return strings.TrimSpace(creds.APIKey)
	}
	return strings.TrimSpace(creds.Token)
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

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fetch.ResultFatal("Token invalid or expired. Run `vibeusage auth warp` to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Warp usage request failed: HTTP %d (%s)", resp.StatusCode, summarizeBody(resp.Body))), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Warp API: %v", resp.JSONErr)), nil
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
		utilization = clampPct((info.RequestsUsed * 100) / info.RequestLimit)
	}
	primary := models.UsagePeriod{
		Name:        "Monthly Credits",
		Utilization: utilization,
		PeriodType:  models.PeriodMonthly,
		ResetsAt:    parseRFC3339Ptr(info.NextRefreshTime),
	}

	periods := []models.UsagePeriod{primary}
	if info.BonusCredits != nil && info.BonusCredits.Total > 0 {
		bonusUtil := clampPct((info.BonusCredits.Used * 100) / info.BonusCredits.Total)
		periods = append(periods, models.UsagePeriod{
			Name:        "Bonus Credits",
			Utilization: bonusUtil,
			PeriodType:  models.PeriodMonthly,
			ResetsAt:    parseRFC3339Ptr(info.BonusCredits.NextRefreshTime),
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

func parseRFC3339Ptr(raw string) *time.Time {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

func clampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
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
