package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

const webBaseURL = "https://claude.ai/api/organizations"

// WebStrategy fetches Claude usage using a session cookie.
type WebStrategy struct {
	HTTPTimeout float64
}

func (s *WebStrategy) IsAvailable() bool {
	return s.loadSessionKey() != ""
}

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionKey := s.loadSessionKey()
	if sessionKey == "" {
		return fetch.ResultFail("No session key found"), nil
	}

	orgID := s.getOrgID(ctx, sessionKey)
	if orgID == "" {
		return fetch.ResultFail("Failed to get organization ID"), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	sessionCookie := httpclient.WithCookie("sessionKey", sessionKey)
	orgBase := webBaseURL + "/" + orgID

	// Fetch usage, overage, and prepaid credits concurrently.
	// Overage and credits are best-effort — failures are silently ignored.
	type usageOutcome struct {
		resp    OAuthUsageResponse
		status  int
		jsonErr error
		err     error
	}
	type overageOutcome struct {
		overage *models.OverageUsage
	}
	type creditsOutcome struct {
		credits *WebPrepaidCreditsResponse
	}

	usageCh := make(chan usageOutcome, 1)
	overageCh := make(chan overageOutcome, 1)
	creditsCh := make(chan creditsOutcome, 1)

	go func() {
		var usageResp OAuthUsageResponse
		resp, err := client.GetJSONCtx(ctx, orgBase+"/usage", &usageResp, sessionCookie)
		if err != nil {
			usageCh <- usageOutcome{err: err}
			return
		}
		usageCh <- usageOutcome{resp: usageResp, status: resp.StatusCode, jsonErr: resp.JSONErr}
	}()

	go func() {
		var overageResp WebOverageResponse
		resp, err := client.GetJSONCtx(ctx, orgBase+"/overage_spend_limit", &overageResp, sessionCookie)
		if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
			overageCh <- overageOutcome{}
			return
		}
		overageCh <- overageOutcome{overage: overageResp.ToOverageUsage()}
	}()

	go func() {
		var creditsResp WebPrepaidCreditsResponse
		resp, err := client.GetJSONCtx(ctx, orgBase+"/prepaid/credits", &creditsResp, sessionCookie)
		if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
			creditsCh <- creditsOutcome{}
			return
		}
		creditsCh <- creditsOutcome{credits: &creditsResp}
	}()

	usage := <-usageCh
	overage := <-overageCh
	credits := <-creditsCh

	if usage.err != nil {
		return fetch.ResultFail("Request failed: " + usage.err.Error()), nil
	}
	if usage.status == 401 {
		return fetch.ResultFatal("Session key expired or invalid"), nil
	}
	if usage.status != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", usage.status)), nil
	}
	if usage.jsonErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid usage response: %v", usage.jsonErr)), nil
	}

	snapshot := parseUsageResponse(usage.resp, "web", overage.overage)

	if credits.credits != nil {
		snapshot.Billing = credits.credits.ToBillingDetail()
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *WebStrategy) loadSessionKey() string {
	path := config.CredentialPath("claude", "session")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}

	value := ""
	var creds WebSessionCredentials
	if err := json.Unmarshal(data, &creds); err == nil {
		value = strings.TrimSpace(creds.SessionKey)
	} else {
		// Try raw string
		value = strings.TrimSpace(string(data))
	}

	if strings.HasPrefix(value, "sk-ant-sid01-") {
		return value
	}
	return ""
}

func (s *WebStrategy) getOrgID(ctx context.Context, sessionKey string) string {
	// Check cache
	if cached := config.LoadCachedOrgID("claude"); cached != "" {
		return cached
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var orgs []WebOrganization
	resp, err := client.GetJSONCtx(ctx, webBaseURL, &orgs,
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
