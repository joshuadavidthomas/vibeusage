package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
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

	client := httpclient.New()
	sessionCookie := httpclient.WithCookie("sessionKey", sessionKey)

	// Fetch usage
	usageURL := "https://claude.ai/api/organizations/" + orgID + "/usage"
	var usageResp WebUsageResponse
	resp, err := client.GetJSON(usageURL, &usageResp, sessionCookie)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("Session key expired or invalid"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail("Invalid usage response"), nil
	}

	// Fetch overage
	var overage *models.OverageUsage
	overageURL := "https://claude.ai/api/organizations/" + orgID + "/overage_spend_limit"
	var overageResp WebOverageResponse
	oResp, err := client.GetJSON(overageURL, &overageResp, sessionCookie)
	if err == nil && oResp.StatusCode == 200 && oResp.JSONErr == nil {
		overage = overageResp.ToOverageUsage()
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

	client := httpclient.New()
	var orgs []WebOrganization
	resp, err := client.GetJSON("https://claude.ai/api/organizations", &orgs,
		httpclient.WithCookie("sessionKey", sessionKey),
	)
	if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
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
