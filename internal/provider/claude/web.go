package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

const (
	webBaseURL     = "https://claude.ai/api/organizations"
	webRoutinesURL = "https://claude.ai/v1/code/routines/run-budget"
)

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

	// Fetch usage, prepaid credits, and routines budget concurrently.
	// Credits and routines are best-effort — failures are silently ignored.
	type usageOutcome struct {
		resp    OAuthUsageResponse
		status  int
		header  http.Header
		jsonErr error
		err     error
	}
	type creditsOutcome struct {
		credits *WebPrepaidCreditsResponse
	}
	type routinesOutcome struct {
		routines *RoutinesBudgetResponse
	}

	usageCh := make(chan usageOutcome, 1)
	creditsCh := make(chan creditsOutcome, 1)
	routinesCh := make(chan routinesOutcome, 1)

	go func() {
		var usageResp OAuthUsageResponse
		resp, err := client.GetJSONCtx(ctx, orgBase+"/usage", &usageResp, sessionCookie)
		if err != nil {
			usageCh <- usageOutcome{err: err}
			return
		}
		usageCh <- usageOutcome{resp: usageResp, status: resp.StatusCode, header: resp.Header, jsonErr: resp.JSONErr}
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

	go func() {
		var routinesResp RoutinesBudgetResponse
		resp, err := client.GetJSONCtx(ctx, webRoutinesURL, &routinesResp, sessionCookie)
		if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
			routinesCh <- routinesOutcome{}
			return
		}
		routinesCh <- routinesOutcome{routines: &routinesResp}
	}()

	usage := <-usageCh
	credits := <-creditsCh
	routines := <-routinesCh

	if usage.err != nil {
		return fetch.ResultFail("Request failed: " + usage.err.Error()), nil
	}
	if usage.status == 401 {
		return fetch.ResultFatal("Session key expired or invalid"), nil
	}
	if usage.status == 429 {
		retryAt := parseRetryAfter(usage.header, time.Now().UTC())
		return fetch.ResultThrottled("Rate limited by Anthropic", retryAt), nil
	}
	if usage.status != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", usage.status)), nil
	}
	if usage.jsonErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid usage response: %v", usage.jsonErr)), nil
	}

	snapshot := parseUsageResponse(usage.resp, "web")

	if credits.credits != nil {
		snapshot.Billing = credits.credits.ToBillingDetail()
	}

	if routines.routines != nil {
		if p := routinesToPeriod(routines.routines); p != nil {
			snapshot.Periods = append(snapshot.Periods, *p)
		}
	}

	return fetch.ResultOK(*snapshot), nil
}

// routinesToPeriod converts a routines budget response into a count-based
// usage period. Returns nil if the response cannot be parsed.
func routinesToPeriod(r *RoutinesBudgetResponse) *models.UsagePeriod {
	limit, err := r.Limit.Int64()
	if err != nil {
		return nil
	}
	used, err := r.Used.Int64()
	if err != nil {
		return nil
	}
	usedInt := int(used)
	limitInt := int(limit)
	utilization := 0
	if limitInt > 0 {
		utilization = int((float64(usedInt) / float64(limitInt)) * 100)
		if utilization > 100 {
			utilization = 100
		}
	}
	return &models.UsagePeriod{
		Name:        "Daily routine runs",
		Utilization: utilization,
		PeriodType:  models.PeriodDaily,
		Used:        &usedInt,
		Limit:       &limitInt,
	}
}

func (s *WebStrategy) loadSessionKey() string {
	data, err := config.ReadCredential("claude", "session")
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
