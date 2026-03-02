package warp

import (
	"context"
	"fmt"
	"runtime"
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
		StatusURL:   "https://status.warp.dev",
	}
}

func (w Warp) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{EnvVars: []string{"WARP_API_KEY", "WARP_TOKEN"}}
}

func (w Warp) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&APIKeyStrategy{HTTPTimeout: timeout}}
}

func (w Warp) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.warp.dev")
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
		return fetch.ResultFail("No API key found. Set WARP_API_KEY or use 'vibeusage auth warp'"), nil
	}
	return fetchUsage(ctx, token, s.HTTPTimeout)
}

func fetchUsage(ctx context.Context, token string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)

	osName := runtime.GOOS
	payload := GraphQLRequest{
		OperationName: "GetRequestLimitInfo",
		Query:         warpUsageQuery,
		Variables: map[string]any{
			"requestContext": map[string]any{
				"clientContext": map[string]any{},
				"osContext": map[string]any{
					"category": osName,
					"name":     osName,
				},
			},
		},
	}

	headers := []httpclient.RequestOption{
		httpclient.WithBearer(token),
		httpclient.WithHeader("x-warp-client-id", "vibeusage"),
		httpclient.WithHeader("x-warp-os", osName),
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

const warpUsageQuery = `query GetRequestLimitInfo($requestContext: RequestContext!) {
  user(requestContext: $requestContext) {
    __typename
    ... on UserOutput {
      user {
        requestLimitInfo {
          isUnlimited
          nextRefreshTime
          requestLimit
          requestsUsedSinceLastRefresh
          requestLimitRefreshDuration
          isUnlimitedVoice
          voiceRequestLimit
          voiceTokenLimit
          voiceRequestsUsedSinceLastRefresh
          voiceTokensUsedSinceLastRefresh
          isUnlimitedCodebaseIndices
          maxCodebaseIndices
          maxFilesPerRepo
          embeddingGenerationBatchSize
          requestLimitPooling
        }
        bonusGrants {
          requestCreditsGranted
          requestCreditsRemaining
          expiration
        }
        workspaces {
          name
          bonusGrantsInfo {
            grants {
              requestCreditsGranted
              requestCreditsRemaining
              expiration
            }
          }
        }
      }
    }
  }
}`

// GraphQL request/response types

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
	User *UserUnion `json:"user"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type UserUnion struct {
	TypeName string      `json:"__typename"`
	User     *UserFields `json:"user"`
}

type UserFields struct {
	RequestLimitInfo *RequestLimitInfo `json:"requestLimitInfo"`
	BonusGrants      []BonusGrant      `json:"bonusGrants"`
	Workspaces       []Workspace       `json:"workspaces"`
}

type RequestLimitInfo struct {
	IsUnlimited                       bool   `json:"isUnlimited"`
	NextRefreshTime                   string `json:"nextRefreshTime"`
	RequestLimit                      int    `json:"requestLimit"`
	RequestsUsedSinceLastRefresh      int    `json:"requestsUsedSinceLastRefresh"`
	RequestLimitRefreshDuration       string `json:"requestLimitRefreshDuration"`
	IsUnlimitedVoice                  bool   `json:"isUnlimitedVoice"`
	VoiceRequestLimit                 int    `json:"voiceRequestLimit"`
	VoiceTokenLimit                   int    `json:"voiceTokenLimit"`
	VoiceRequestsUsedSinceLastRefresh int    `json:"voiceRequestsUsedSinceLastRefresh"`
	VoiceTokensUsedSinceLastRefresh   int    `json:"voiceTokensUsedSinceLastRefresh"`
	IsUnlimitedCodebaseIndices        bool   `json:"isUnlimitedCodebaseIndices"`
	MaxCodebaseIndices                int    `json:"maxCodebaseIndices"`
	MaxFilesPerRepo                   int    `json:"maxFilesPerRepo"`
	EmbeddingGenerationBatchSize      int    `json:"embeddingGenerationBatchSize"`
	RequestLimitPooling               string `json:"requestLimitPooling"`
}

type BonusGrant struct {
	RequestCreditsGranted   int    `json:"requestCreditsGranted"`
	RequestCreditsRemaining int    `json:"requestCreditsRemaining"`
	Expiration              string `json:"expiration"`
}

type Workspace struct {
	Name            string           `json:"name"`
	BonusGrantsInfo *BonusGrantsInfo `json:"bonusGrantsInfo"`
}

type BonusGrantsInfo struct {
	Grants []BonusGrant `json:"grants"`
}

func parseUsageSnapshot(resp GraphQLResponse) (*models.UsageSnapshot, error) {
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	if resp.Data == nil || resp.Data.User == nil {
		return nil, fmt.Errorf("missing user data in response")
	}

	userUnion := resp.Data.User
	if userUnion.TypeName != "" && userUnion.TypeName != "UserOutput" {
		return nil, fmt.Errorf("unexpected user type %q", userUnion.TypeName)
	}
	if userUnion.User == nil || userUnion.User.RequestLimitInfo == nil {
		return nil, fmt.Errorf("missing requestLimitInfo")
	}

	info := userUnion.User.RequestLimitInfo

	utilization := 0
	if !info.IsUnlimited && info.RequestLimit > 0 {
		utilization = models.ClampPct((info.RequestsUsedSinceLastRefresh * 100) / info.RequestLimit)
	}

	periodType := models.PeriodMonthly
	if strings.EqualFold(info.RequestLimitRefreshDuration, "DAILY") {
		periodType = models.PeriodDaily
	} else if strings.EqualFold(info.RequestLimitRefreshDuration, "WEEKLY") {
		periodType = models.PeriodWeekly
	}

	primary := models.UsagePeriod{
		Name:        "Monthly Credits",
		Utilization: utilization,
		PeriodType:  periodType,
		ResetsAt:    models.ParseRFC3339Ptr(info.NextRefreshTime),
	}

	periods := []models.UsagePeriod{primary}

	// Combine user-level and workspace-level bonus grants
	bonus := combineBonusGrants(userUnion.User)
	if bonus.total > 0 {
		bonusUsed := bonus.total - bonus.remaining
		bonusUtil := 0
		if bonus.total > 0 {
			bonusUtil = models.ClampPct((bonusUsed * 100) / bonus.total)
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Bonus Credits",
			Utilization: bonusUtil,
			PeriodType:  models.PeriodMonthly,
		})
	}

	plan := "Free"
	if info.IsUnlimited {
		plan = "Unlimited"
	}
	pooling := info.RequestLimitPooling
	if pooling != "" && !strings.EqualFold(pooling, "USER") {
		plan += " (" + pooling + ")"
	}

	return &models.UsageSnapshot{
		Provider:  "warp",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Identity: &models.ProviderIdentity{
			Plan: plan,
		},
		Source: "api_key",
	}, nil
}

type bonusSummary struct {
	remaining int
	total     int
}

func combineBonusGrants(user *UserFields) bonusSummary {
	var remaining, total int

	for _, g := range user.BonusGrants {
		remaining += g.RequestCreditsRemaining
		total += g.RequestCreditsGranted
	}

	for _, ws := range user.Workspaces {
		if ws.BonusGrantsInfo == nil {
			continue
		}
		for _, g := range ws.BonusGrantsInfo.Grants {
			remaining += g.RequestCreditsRemaining
			total += g.RequestCreditsGranted
		}
	}

	return bonusSummary{remaining: remaining, total: total}
}

func isAuthGraphQLError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unauthorized") || strings.Contains(s, "forbidden") || strings.Contains(s, "auth")
}
