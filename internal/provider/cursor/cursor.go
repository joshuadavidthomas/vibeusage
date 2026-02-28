package cursor

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

type Cursor struct{}

func (c Cursor) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "cursor",
		Name:         "Cursor",
		Description:  "AI-powered code editor",
		Homepage:     "https://cursor.com",
		StatusURL:    "https://status.cursor.com",
		DashboardURL: "https://cursor.com/settings/usage",
	}
}

func (c Cursor) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		EnvVars: []string{"CURSOR_API_KEY"},
	}
}

func (c Cursor) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&WebStrategy{HTTPTimeout: timeout}}
}

func (c Cursor) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.cursor.com")
}

const (
	usageSummaryURL = "https://cursor.com/api/usage-summary"
	authMeURL       = "https://cursor.com/api/auth/me"
)

// Auth returns the manual session token flow for Cursor.
func (c Cursor) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your session token from cursor.com:\n" +
			"  1. Open https://cursor.com in your browser\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://cursor.com\n" +
			"  4. Find one of: WorkosCursorSessionToken, __Secure-next-auth.session-token\n" +
			"  5. Copy its value",
		Placeholder: "paste token here",
		Validate:    provider.ValidateNotEmpty,
		CredPath:    config.CredentialPath("cursor", "session"),
		JSONKey:     "session_token",
	}
}

func init() {
	provider.Register(Cursor{})
}

type WebStrategy struct {
	HTTPTimeout float64
}

func (s *WebStrategy) IsAvailable() bool {
	path := config.CredentialPath("cursor", "session")
	_, err := os.Stat(path)
	return err == nil
}

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionToken := s.loadSessionToken()
	if sessionToken == "" {
		return fetch.ResultFail("No session token found"), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	sessionCookie := httpclient.WithCookie("__Secure-next-auth.session-token", sessionToken)
	userAgent := httpclient.WithHeader("User-Agent", "Mozilla/5.0")

	// Fetch usage
	var usageResp UsageSummaryResponse
	resp, err := client.PostJSONCtx(ctx, usageSummaryURL, nil, &usageResp,
		sessionCookie, userAgent,
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Session token expired or invalid"), nil
	}
	if resp.StatusCode == 404 {
		return fetch.ResultFail("User not found or no active subscription"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid usage response: %v", resp.JSONErr)), nil
	}

	// Fetch user data
	var userResp *UserMeResponse
	var u UserMeResponse
	uResp, err := client.GetJSONCtx(ctx, authMeURL, &u,
		sessionCookie, userAgent,
	)
	if err == nil && uResp.StatusCode == 200 && uResp.JSONErr == nil {
		userResp = &u
	}

	snapshot := s.parseTypedResponse(usageResp, userResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *WebStrategy) loadSessionToken() string {
	path := config.CredentialPath("cursor", "session")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}
	var creds SessionCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return strings.TrimSpace(string(data))
	}
	if tok := creds.EffectiveToken(); tok != "" {
		return tok
	}
	return strings.TrimSpace(string(data))
}

func (s *WebStrategy) parseTypedResponse(usageResp UsageSummaryResponse, userResp *UserMeResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod
	var overage *models.OverageUsage

	resetsAt := usageResp.BillingCycleEndTime()

	if usageResp.IndividualUsage != nil && usageResp.IndividualUsage.Plan != nil {
		plan := usageResp.IndividualUsage.Plan
		var utilization int

		if plan.TotalPercentUsed > 0 {
			utilization = int(plan.TotalPercentUsed)
		} else if plan.Limit > 0 {
			utilization = int((plan.Used / plan.Limit) * 100)
		}

		periods = append(periods, models.UsagePeriod{
			Name:        "Plan Usage",
			Utilization: utilization,
			PeriodType:  models.PeriodMonthly,
			ResetsAt:    resetsAt,
		})
	}

	if usageResp.IndividualUsage != nil && usageResp.IndividualUsage.OnDemand != nil {
		od := usageResp.IndividualUsage.OnDemand
		enabled := od.Enabled != nil && *od.Enabled
		if enabled && od.Limit != nil && *od.Limit > 0 {
			overage = &models.OverageUsage{
				Used:      od.Used / 100.0,
				Limit:     *od.Limit / 100.0,
				Currency:  "USD",
				IsEnabled: true,
			}
		}
	}

	// Identity: prefer membershipType from usage-summary, email from user/me response
	membershipType := usageResp.MembershipType
	var identity *models.ProviderIdentity
	if userResp != nil && (userResp.Email != "" || userResp.MembershipType != "") {
		email := userResp.Email
		plan := userResp.MembershipType
		if plan == "" {
			plan = membershipType
		}
		identity = &models.ProviderIdentity{Email: email, Plan: plan}
	} else if membershipType != "" {
		identity = &models.ProviderIdentity{Plan: membershipType}
	}

	if len(periods) == 0 {
		return nil
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "cursor",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    "web",
	}
}
