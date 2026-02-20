package cursor

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
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

func (c Cursor) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{&WebStrategy{}}
}

func (c Cursor) FetchStatus() models.ProviderStatus {
	return provider.FetchStatuspageStatus("https://status.cursor.com/api/v2/status.json")
}

func init() {
	provider.Register(Cursor{})
}

type WebStrategy struct{}

func (s *WebStrategy) Name() string { return "web" }

func (s *WebStrategy) IsAvailable() bool {
	path := config.CredentialPath("cursor", "session")
	_, err := os.Stat(path)
	return err == nil
}

func (s *WebStrategy) Fetch() (fetch.FetchResult, error) {
	sessionToken := s.loadSessionToken()
	if sessionToken == "" {
		return fetch.ResultFail("No session token found"), nil
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch usage
	req, _ := http.NewRequest("POST", "https://www.cursor.com/api/usage-summary", nil)
	req.AddCookie(&http.Cookie{Name: "__Secure-next-auth.session-token", Value: sessionToken})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Session token expired or invalid"), nil
	}
	if resp.StatusCode == 404 {
		return fetch.ResultFail("User not found or no active subscription"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail("Usage request failed: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var usageResp UsageSummaryResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return fetch.ResultFail("Invalid usage response"), nil
	}

	// Fetch user data
	var userResp *UserMeResponse
	userReq, _ := http.NewRequest("GET", "https://www.cursor.com/api/auth/me", nil)
	userReq.AddCookie(&http.Cookie{Name: "__Secure-next-auth.session-token", Value: sessionToken})
	userReq.Header.Set("User-Agent", "Mozilla/5.0")

	userHTTPResp, err := client.Do(userReq)
	if err == nil {
		defer userHTTPResp.Body.Close()
		if userHTTPResp.StatusCode == 200 {
			userBody, _ := io.ReadAll(userHTTPResp.Body)
			var u UserMeResponse
			if json.Unmarshal(userBody, &u) == nil {
				userResp = &u
			}
		}
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

	if usageResp.PremiumRequests != nil {
		used := usageResp.PremiumRequests.Used
		available := usageResp.PremiumRequests.Available
		total := used + available
		var utilization int
		if total > 0 {
			utilization = int((used / total) * 100)
		}

		var resetsAt *time.Time
		if usageResp.BillingCycle != nil {
			resetsAt = usageResp.BillingCycle.EndTime()
		}

		periods = append(periods, models.UsagePeriod{
			Name:        "Premium Requests",
			Utilization: utilization,
			PeriodType:  models.PeriodMonthly,
			ResetsAt:    resetsAt,
		})
	}

	if usageResp.OnDemandSpend != nil && usageResp.OnDemandSpend.LimitCents > 0 {
		overage = &models.OverageUsage{
			Used:      usageResp.OnDemandSpend.UsedCents / 100.0,
			Limit:     usageResp.OnDemandSpend.LimitCents / 100.0,
			Currency:  "USD",
			IsEnabled: true,
		}
	}

	var identity *models.ProviderIdentity
	if userResp != nil && (userResp.Email != "" || userResp.MembershipType != "") {
		identity = &models.ProviderIdentity{Email: userResp.Email, Plan: userResp.MembershipType}
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
