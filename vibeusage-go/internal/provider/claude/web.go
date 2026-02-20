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
	var usageResp WebUsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return fetch.ResultFail("Invalid usage response"), nil
	}

	// Fetch overage
	var overage *models.OverageUsage
	overageURL := "https://claude.ai/api/organizations/" + orgID + "/overage_spend_limit"
	oReq, _ := http.NewRequest("GET", overageURL, nil)
	oReq.AddCookie(&http.Cookie{Name: "sessionKey", Value: sessionKey})

	oResp, err := client.Do(oReq)
	if err == nil {
		defer oResp.Body.Close()
		if oResp.StatusCode == 200 {
			oBody, _ := io.ReadAll(oResp.Body)
			var overageResp WebOverageResponse
			if json.Unmarshal(oBody, &overageResp) == nil {
				overage = overageResp.ToOverageUsage()
			}
		}
	}

	snapshot := s.parseWebUsageResponse(usageResp, overage)
	return fetch.ResultOK(*snapshot), nil
}

func (s *WebStrategy) loadSessionKey() string {
	path := config.CredentialPath("claude", "session")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}
	var creds WebSessionCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		// Try raw string
		return string(data)
	}
	if creds.SessionKey != "" {
		return creds.SessionKey
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
	var orgs []WebOrganization
	if err := json.Unmarshal(body, &orgs); err != nil {
		return ""
	}

	orgID := s.findChatOrgID(orgs)
	if orgID != "" {
		_ = config.CacheOrgID("claude", orgID)
	}
	return orgID
}

// findChatOrgID finds the first organization with "chat" capability,
// falling back to the first organization if none have it.
func (s *WebStrategy) findChatOrgID(orgs []WebOrganization) string {
	for _, org := range orgs {
		if org.HasCapability("chat") {
			if id := org.OrgID(); id != "" {
				return id
			}
		}
	}

	// Fallback to first org
	if len(orgs) > 0 {
		return orgs[0].OrgID()
	}
	return ""
}

func (s *WebStrategy) parseWebUsageResponse(resp WebUsageResponse, overage *models.OverageUsage) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	if resp.UsageLimit > 0 {
		utilization := int((resp.UsageAmount / resp.UsageLimit) * 100)
		var resetsAt *time.Time
		if resp.PeriodEnd != "" {
			if t, err := time.Parse(time.RFC3339, resp.PeriodEnd); err == nil {
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
	if resp.Email != "" || resp.Organization != "" || resp.Plan != "" {
		identity = &models.ProviderIdentity{
			Email:        resp.Email,
			Organization: resp.Organization,
			Plan:         resp.Plan,
		}
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
