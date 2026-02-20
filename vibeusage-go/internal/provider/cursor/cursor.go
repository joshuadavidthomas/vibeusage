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
	var usageData map[string]any
	if err := json.Unmarshal(body, &usageData); err != nil {
		return fetch.ResultFail("Invalid usage response"), nil
	}

	// Fetch user data
	var userData map[string]any
	userReq, _ := http.NewRequest("GET", "https://www.cursor.com/api/auth/me", nil)
	userReq.AddCookie(&http.Cookie{Name: "__Secure-next-auth.session-token", Value: sessionToken})
	userReq.Header.Set("User-Agent", "Mozilla/5.0")

	userResp, err := client.Do(userReq)
	if err == nil && userResp.StatusCode == 200 {
		userBody, _ := io.ReadAll(userResp.Body)
		userResp.Body.Close()
		json.Unmarshal(userBody, &userData)
	}

	snapshot := s.parseResponse(usageData, userData)
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
	var creds map[string]any
	if err := json.Unmarshal(data, &creds); err != nil {
		return strings.TrimSpace(string(data))
	}
	for _, key := range []string{"session_token", "token", "session_key", "session"} {
		if v, ok := creds[key].(string); ok && v != "" {
			return v
		}
	}
	return strings.TrimSpace(string(data))
}

func (s *WebStrategy) parseResponse(usageData, userData map[string]any) *models.UsageSnapshot {
	if usageData == nil {
		return nil
	}

	var periods []models.UsagePeriod
	var overage *models.OverageUsage

	// Premium requests
	if premium, ok := usageData["premium_requests"].(map[string]any); ok {
		used, _ := premium["used"].(float64)
		available, _ := premium["available"].(float64)
		total := used + available
		var utilization int
		if total > 0 {
			utilization = int((used / total) * 100)
		}

		var resetsAt *time.Time
		if bc, ok := usageData["billing_cycle"].(map[string]any); ok {
			if end, ok := bc["end"].(string); ok {
				end = strings.Replace(end, "Z", "+00:00", 1)
				if t, err := time.Parse(time.RFC3339, end); err == nil {
					resetsAt = &t
				}
			} else if endF, ok := bc["end"].(float64); ok {
				t := time.UnixMilli(int64(endF)).UTC()
				resetsAt = &t
			}
		}

		periods = append(periods, models.UsagePeriod{
			Name:        "Premium Requests",
			Utilization: utilization,
			PeriodType:  models.PeriodMonthly,
			ResetsAt:    resetsAt,
		})
	}

	// On-demand spend
	if onDemand, ok := usageData["on_demand_spend"].(map[string]any); ok {
		limitCents, _ := onDemand["limit_cents"].(float64)
		if limitCents > 0 {
			usedCents, _ := onDemand["used_cents"].(float64)
			overage = &models.OverageUsage{
				Used:      usedCents / 100.0,
				Limit:     limitCents / 100.0,
				Currency:  "USD",
				IsEnabled: true,
			}
		}
	}

	var identity *models.ProviderIdentity
	if userData != nil {
		email, _ := userData["email"].(string)
		memberType, _ := userData["membership_type"].(string)
		if email != "" || memberType != "" {
			identity = &models.ProviderIdentity{Email: email, Plan: memberType}
		}
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
