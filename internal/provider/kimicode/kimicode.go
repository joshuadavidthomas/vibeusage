package kimicode

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/deviceflow"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type KimiCode struct{}

func (k KimiCode) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "kimicode",
		Name:        "Kimi Code",
		Description: "Moonshot AI coding assistant",
		Homepage:    "https://www.kimi.com",
	}
}

func (k KimiCode) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		EnvVars: []string{"KIMI_CODE_API_KEY"},
	}
}

func (k KimiCode) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&DeviceFlowStrategy{HTTPTimeout: timeout},
		&APIKeyStrategy{HTTPTimeout: timeout},
	}
}

func (k KimiCode) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

// Auth returns the Kimi Code device flow.
func (k KimiCode) Auth() provider.AuthFlow {
	return provider.DeviceAuthFlow{
		Config: deviceflow.Config{
			DeviceCodeURL: oauthBaseURL() + deviceCodePath,
			DeviceCodeParams: map[string]string{
				"client_id": clientID,
			},
			TokenURL: oauthBaseURL() + tokenPath,
			TokenParams: map[string]string{
				"client_id":  clientID,
				"grant_type": deviceFlowGrant,
			},
			HTTPOptions:    commonHeaders(),
			HTTPTimeout:    config.Get().Fetch.Timeout,
			CredentialPath: config.CredentialPath("kimicode", "oauth"),
		},
	}
}

func init() {
	provider.Register(KimiCode{})
}

const usageURL = "https://api.kimi.com/coding/v1/usages"

// baseURL returns the API base URL, respecting the KIMI_CODE_BASE_URL override.
func baseURL() string {
	if base := os.Getenv("KIMI_CODE_BASE_URL"); base != "" {
		return base + "/coding/v1/usages"
	}
	return usageURL
}

// commonHeaders returns the headers that kimi-cli sends with all requests.
func commonHeaders() []httpclient.RequestOption {
	hostname, _ := os.Hostname()
	return []httpclient.RequestOption{
		httpclient.WithHeader("X-Msh-Platform", "vibeusage"),
		httpclient.WithHeader("X-Msh-Device-Name", hostname),
		httpclient.WithHeader("X-Msh-Device-Model", runtime.GOOS+" "+runtime.GOARCH),
	}
}

// fetchUsage makes the API call and parses the response. Shared between strategies.
func fetchUsage(ctx context.Context, token, source string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	opts := append(commonHeaders(), httpclient.WithBearer(token))

	var usageResp UsageResponse
	resp, err := client.GetJSONCtx(ctx, baseURL(), &usageResp, opts...)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Token expired or invalid. Run `vibeusage auth kimicode` to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Kimi API: %v", resp.JSONErr)), nil
	}

	snapshot := parseUsageResponse(usageResp, source)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse Kimi usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

// parseUsageResponse converts the API response to a UsageSnapshot.
func parseUsageResponse(resp UsageResponse, source string) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	// Overall weekly usage counter — always shown when present. This is
	// distinct from the per-window rate limits below: Kimi tracks a rolling
	// weekly quota separately from short-window rate limits.
	if resp.Usage != nil {
		periods = append(periods, models.UsagePeriod{
			Name:        "Weekly",
			Utilization: resp.Usage.Utilization(),
			PeriodType:  models.PeriodWeekly,
			ResetsAt:    resp.Usage.ResetTimeUTC(),
		})
	}

	// Per-window limits
	for _, limit := range resp.Limits {
		if limit.Detail == nil {
			continue
		}
		periodType := limit.Window.PeriodType()
		name := limit.Window.DisplayName()
		switch periodType {
		case models.PeriodSession:
			name = "Session (" + limit.Window.DisplayName() + ")"
		case models.PeriodDaily:
			name = "Daily"
		case models.PeriodWeekly:
			name = "Weekly"
		case models.PeriodMonthly:
			name = "Monthly"
		}

		periods = append(periods, models.UsagePeriod{
			Name:        name,
			Utilization: limit.Detail.Utilization(),
			PeriodType:  periodType,
			ResetsAt:    limit.Detail.ResetTimeUTC(),
		})
	}

	if len(periods) == 0 {
		return nil
	}

	var identity *models.ProviderIdentity
	if resp.User != nil && resp.User.Membership != nil && resp.User.Membership.Level != "" {
		plan := PlanName(resp.User.Membership.Level)
		if plan != "" {
			identity = &models.ProviderIdentity{Plan: plan}
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "kimicode",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    source,
	}
}
