package claude

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

type WebStrategy struct{}

func (s *WebStrategy) Name() string { return "web" }

func (s *WebStrategy) IsAvailable() bool {
	_, err := os.Stat(config.CredentialPath("claude", "session"))
	return err == nil
}

func (s *WebStrategy) Fetch() (fetch.FetchResult, error) {
	sessionKey := s.loadSessionKey()
	if sessionKey == "" {
		return fetch.ResultFail("No session key found"), nil
	}

	orgID := s.getOrgID(sessionKey)
	if orgID == "" {
		return fetch.ResultFail("Failed to get organization ID"), nil
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch usage
	usageURL := "https://claude.ai/api/organizations/" + orgID + "/usage"
	req, _ := http.NewRequest("GET", usageURL, nil)
	req.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})

	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Session key expired or invalid"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail("Usage request failed: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var usageData map[string]any
	if err := json.Unmarshal(body, &usageData); err != nil {
		return fetch.ResultFail("Invalid usage response"), nil
	}

	// Fetch overage
	var overage *models.OverageUsage
	overageURL := "https://claude.ai/api/organizations/" + orgID + "/overage_spend_limit"
	oReq, _ := http.NewRequest("GET", overageURL, nil)
	oReq.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})

	oResp, err := client.Do(oReq)
	if err == nil && oResp.StatusCode == 200 {
		oBody, _ := io.ReadAll(oResp.Body)
		oResp.Body.Close()
		var oData map[string]any
		if json.Unmarshal(oBody, &oData) == nil {
			overage = parseOverage(oData)
		}
	}

	snapshot := s.parseUsageResponse(usageData, overage)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *WebStrategy) loadSessionKey() string {
	path := config.CredentialPath("claude", "session")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}
	var creds map[string]any
	if err := json.Unmarshal(data, &creds); err != nil {
		// Try raw string
		return string(data)
	}
	if sk, ok := creds["session_key"].(string); ok {
		return sk
	}
	return ""
}

func (s *WebStrategy) getOrgID(sessionKey string) string {
	// Check cache
	if cached := config.LoadCachedOrgID("claude"); cached != "" {
		return cached
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", "https://claude.ai/api/organizations", nil)
	req.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var orgs []map[string]any
	if err := json.Unmarshal(body, &orgs); err != nil {
		return ""
	}

	// Find org with "chat" capability
	for _, org := range orgs {
		if caps, ok := org["capabilities"].([]any); ok {
			for _, cap := range caps {
				if capStr, ok := cap.(string); ok && capStr == "chat" {
					orgID := getStringField(org, "uuid", "id")
					if orgID != "" {
						_ = config.CacheOrgID("claude", orgID)
						return orgID
					}
				}
			}
		}
	}

	// Fallback to first org
	if len(orgs) > 0 {
		orgID := getStringField(orgs[0], "uuid", "id")
		if orgID != "" {
			_ = config.CacheOrgID("claude", orgID)
			return orgID
		}
	}
	return ""
}

func (s *WebStrategy) parseUsageResponse(data map[string]any, overage *models.OverageUsage) *models.UsageSnapshot {
	if data == nil {
		return nil
	}

	var periods []models.UsagePeriod

	usageAmount, _ := data["usage_amount"].(float64)
	usageLimit, _ := data["usage_limit"].(float64)

	if usageLimit > 0 {
		utilization := int((usageAmount / usageLimit) * 100)
		var resetsAt *time.Time
		if periodEnd, ok := data["period_end"].(string); ok {
			if t, err := time.Parse(time.RFC3339, periodEnd); err == nil {
				resetsAt = &t
			}
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Usage",
			Utilization: utilization,
			PeriodType:  models.PeriodDaily,
			ResetsAt:    resetsAt,
		})
	}

	var identity *models.ProviderIdentity
	if _, ok := data["email"]; ok {
		email, _ := data["email"].(string)
		org, _ := data["organization"].(string)
		plan, _ := data["plan"].(string)
		identity = &models.ProviderIdentity{Email: email, Organization: org, Plan: plan}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    "web",
	}
}

func parseOverage(data map[string]any) *models.OverageUsage {
	hasLimit, _ := data["has_hard_limit"].(bool)
	if !hasLimit {
		return nil
	}
	used, _ := data["current_spend"].(float64)
	limit, _ := data["hard_limit"].(float64)
	return &models.OverageUsage{
		Used:      used,
		Limit:     limit,
		Currency:  "USD",
		IsEnabled: true,
	}
}

func getStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
