package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

const (
	openCodeBaseURL          = "https://opencode.ai"
	zenBalanceUnitsPerDollar = 100_000_000
	zenFetchGrace            = 2 * time.Second
)

var (
	usagePattern      = regexp.MustCompile(`(rollingUsage|weeklyUsage|monthlyUsage):(?:\$R\[\d+\]=)?\{status:"[^"]+",resetInSec:(\d+),usagePercent:(\d+)\}`)
	zenBalancePattern = regexp.MustCompile(
		`(?:^|[,{])(?:"customerID"|customerID)\s*:\s*(?:\$R\[\d+\]\s*=\s*)?"[^"]+"[^{}]*?,\s*(?:"balance"|balance)\s*:\s*(?:\$R\[\d+\]\s*=\s*)?(-?\d+)`,
	)
)

type Opencode struct{}

func (o Opencode) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "opencode",
		Name:         "OpenCode",
		Description:  "OpenCode Go usage and Zen credits",
		Homepage:     "https://opencode.ai",
		StatusURL:    "https://status.opencode.ai",
		DashboardURL: "https://opencode.ai/settings/billing",
	}
}

func (o Opencode) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{}
}

func (o Opencode) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&WebStrategy{HTTPTimeout: timeout}}
}

func (o Opencode) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchOnlineOrNotStatus(ctx, "https://status.opencode.ai")
}

func (o Opencode) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your OpenCode session cookie:\n" +
			"  1. Open https://opencode.ai in your browser and sign in\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://opencode.ai\n" +
			"  4. Find the cookie named 'auth' and copy its value\n" +
			"  5. Paste it below\n\n" +
			"vibeusage will use your most recently opened workspace. You can select a different one with workspace_id under [providers.opencode].",
		Placeholder: "paste auth cookie value here",
		Validate:    provider.ValidateNotEmpty,
		ProviderID:  "opencode",
		CredType:    "session",
		JSONKey:     "session_token",
	}
}

func init() {
	provider.Register(Opencode{})
}

type WebStrategy struct {
	HTTPTimeout float64
	baseURL     string
}

type sessionCredentials struct {
	SessionToken string `json:"session_token"`
}

func (s *WebStrategy) IsAvailable() bool {
	data, err := config.ReadCredential("opencode", "session")
	return err != nil || data != nil
}

func (s *WebStrategy) loadSessionToken() (string, error) {
	data, err := config.ReadCredential("opencode", "session")
	if err != nil {
		return "", fmt.Errorf("reading OpenCode session credential: %w", err)
	}
	if data == nil {
		return "", nil
	}

	var credentials sessionCredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return "", fmt.Errorf("parsing OpenCode session credential: %w", err)
	}
	return strings.TrimSpace(credentials.SessionToken), nil
}

func workspaceID() string {
	if id := strings.TrimSpace(config.Get().Providers["opencode"].WorkspaceID); id != "" {
		return id
	}
	return strings.TrimSpace(os.Getenv("OPENCODE_WORKSPACE_ID"))
}

type workspacePage struct {
	ID      string
	Billing *models.BillingDetail
}

type workspaceOutcome struct {
	page    workspacePage
	failure *workspaceFailure
}

type workspaceFailure struct {
	message   string
	transient bool
}

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionToken, err := s.loadSessionToken()
	if err != nil {
		return fetch.ResultFatal(err.Error()), nil
	}
	if sessionToken == "" {
		return fetch.ResultFatal("no OpenCode session token found; run `vibeusage auth opencode` to set one up"), nil
	}

	baseURL := strings.TrimRight(s.baseURL, "/")
	if baseURL == "" {
		baseURL = openCodeBaseURL
	}
	client := httpclient.NewFromConfig(s.HTTPTimeout)
	wsID := workspaceID()
	if wsID == "" {
		page, failure := s.fetchWorkspacePage(ctx, client, baseURL+"/auth", sessionToken)
		if failure != nil {
			if failure.transient {
				return fetch.ResultFail(failure.message), nil
			}
			return fetch.ResultFatal(failure.message), nil
		}
		result := s.fetchUsage(ctx, client, workspacePageURL(baseURL, page.ID)+"/go", page.ID, sessionToken)
		return mergeOpenCodeData(result, page.Billing), nil
	}

	workspaceCtx, cancelWorkspace := context.WithCancel(ctx)
	workspaceCh := make(chan workspaceOutcome, 1)
	go func() {
		page, failure := s.fetchWorkspacePage(workspaceCtx, client, workspacePageURL(baseURL, wsID), sessionToken)
		workspaceCh <- workspaceOutcome{page: page, failure: failure}
	}()

	result := s.fetchUsage(ctx, client, workspacePageURL(baseURL, wsID)+"/go", wsID, sessionToken)
	if !result.Success || result.Snapshot == nil {
		outcome := <-workspaceCh
		cancelWorkspace()
		if outcome.failure == nil && outcome.page.ID == wsID {
			return mergeOpenCodeData(result, outcome.page.Billing), nil
		}
		return result, nil
	}

	wait := zenFetchGrace
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			wait = 0
		} else if remaining/2 < wait {
			wait = remaining / 2
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()

	var outcome workspaceOutcome
	select {
	case outcome = <-workspaceCh:
		cancelWorkspace()
	case <-timer.C:
		cancelWorkspace()
		outcome = <-workspaceCh
	case <-ctx.Done():
		cancelWorkspace()
		<-workspaceCh
		return fetch.ResultFail(fmt.Sprintf("OpenCode request failed: %v", ctx.Err())), nil
	}
	if outcome.failure == nil && outcome.page.ID == wsID {
		result = mergeOpenCodeData(result, outcome.page.Billing)
	}
	return result, nil
}

func mergeOpenCodeData(usage fetch.FetchResult, billing *models.BillingDetail) fetch.FetchResult {
	if billing == nil {
		return usage
	}
	if usage.Success && usage.Snapshot != nil {
		usage.Snapshot.Billing = billing
		return usage
	}
	return fetch.ResultOK(models.UsageSnapshot{
		Provider:  "opencode",
		FetchedAt: time.Now().UTC(),
		Billing:   billing,
		Source:    "web",
	})
}

func workspacePageURL(baseURL, wsID string) string {
	return baseURL + "/workspace/" + url.PathEscape(wsID)
}

func (s *WebStrategy) fetchWorkspacePage(ctx context.Context, client *httpclient.Client, pageURL, sessionToken string) (workspacePage, *workspaceFailure) {
	resp, err := client.DoCtx(
		ctx,
		http.MethodGet,
		pageURL,
		nil,
		httpclient.WithCookie("auth", sessionToken),
		httpclient.WithHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"),
		httpclient.WithHeader("User-Agent", "Mozilla/5.0"),
	)
	if err != nil {
		return workspacePage{}, &workspaceFailure{message: fmt.Sprintf("OpenCode workspace request failed: %v", err), transient: true}
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return workspacePage{}, &workspaceFailure{message: "OpenCode session token expired or invalid; run `vibeusage auth opencode` to re-authenticate"}
	case resp.StatusCode == http.StatusNotFound:
		return workspacePage{}, &workspaceFailure{message: "OpenCode workspace not found; check workspace_id in `vibeusage config edit` or OPENCODE_WORKSPACE_ID"}
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return workspacePage{}, &workspaceFailure{message: fmt.Sprintf("OpenCode workspace request failed: HTTP %d", resp.StatusCode), transient: true}
	case resp.StatusCode != http.StatusOK:
		return workspacePage{}, &workspaceFailure{message: fmt.Sprintf("OpenCode workspace request failed: HTTP %d", resp.StatusCode)}
	}
	if isAuthURL(resp.URL) {
		return workspacePage{}, &workspaceFailure{message: "OpenCode session token expired or invalid; run `vibeusage auth opencode` to re-authenticate"}
	}
	if !sameOrigin(pageURL, resp.URL) {
		return workspacePage{}, &workspaceFailure{message: "OpenCode workspace request redirected to an unexpected origin"}
	}

	wsID := workspaceIDFromURL(resp.URL)
	if wsID == "" {
		return workspacePage{}, &workspaceFailure{message: "could not discover an OpenCode workspace; open a workspace at opencode.ai or set workspace_id in `vibeusage config edit`"}
	}
	return workspacePage{ID: wsID, Billing: parseZenBillingFromSSR(string(resp.Body))}, nil
}

func (s *WebStrategy) fetchUsage(ctx context.Context, client *httpclient.Client, usageURL, wsID, sessionToken string) fetch.FetchResult {
	resp, err := client.DoCtx(
		ctx,
		http.MethodGet,
		usageURL,
		nil,
		httpclient.WithCookie("auth", sessionToken),
		httpclient.WithHeader("User-Agent", "Mozilla/5.0"),
	)
	if err != nil {
		return fetch.ResultFail(fmt.Sprintf("OpenCode request failed: %v", err))
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fetch.ResultFatal("OpenCode session token expired or invalid; run `vibeusage auth opencode` to re-authenticate")
	case resp.StatusCode == http.StatusNotFound:
		return fetch.ResultFatal("OpenCode workspace not found; check workspace_id in `vibeusage config edit` or OPENCODE_WORKSPACE_ID")
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return fetch.ResultFail(fmt.Sprintf("OpenCode request failed: HTTP %d", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return fetch.ResultFatal(fmt.Sprintf("OpenCode request failed: HTTP %d", resp.StatusCode))
	}
	if isAuthURL(resp.URL) {
		return fetch.ResultFatal("OpenCode session token expired or invalid; run `vibeusage auth opencode` to re-authenticate")
	}
	if !sameOrigin(usageURL, resp.URL) {
		return fetch.ResultFatal("OpenCode usage request redirected to an unexpected origin")
	}
	if finalWorkspaceID := workspaceIDFromURL(resp.URL); finalWorkspaceID != wsID {
		return fetch.ResultFatal("OpenCode usage request returned a different workspace; check workspace_id in `vibeusage config edit` or OPENCODE_WORKSPACE_ID")
	}

	snapshot, err := parseUsageFromSSR(string(resp.Body))
	if err != nil {
		return fetch.ResultFatal(fmt.Sprintf("parsing OpenCode usage: %v", err))
	}
	return fetch.ResultOK(*snapshot)
}

func sameOrigin(requestURL string, finalURL *url.URL) bool {
	requested, err := url.Parse(requestURL)
	if err != nil || finalURL == nil {
		return false
	}
	return strings.EqualFold(requested.Scheme, finalURL.Scheme) && strings.EqualFold(requested.Host, finalURL.Host)
}

func workspaceIDFromURL(finalURL *url.URL) string {
	if finalURL == nil {
		return ""
	}
	parts := strings.Split(strings.Trim(finalURL.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "workspace" && strings.HasPrefix(parts[i+1], "wrk_") {
			return parts[i+1]
		}
	}
	return ""
}

func isAuthURL(finalURL *url.URL) bool {
	if finalURL == nil {
		return false
	}
	for _, part := range strings.Split(strings.Trim(finalURL.Path, "/"), "/") {
		if part == "auth" {
			return true
		}
	}
	return false
}

func parseZenBillingFromSSR(body string) *models.BillingDetail {
	match := zenBalancePattern.FindStringSubmatch(body)
	if len(match) == 0 {
		return nil
	}
	rawBalance, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return nil
	}
	balance := float64(rawBalance) / zenBalanceUnitsPerDollar
	return &models.BillingDetail{Balance: &balance}
}

func parseUsageFromSSR(body string) (*models.UsageSnapshot, error) {
	matches := usagePattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no usage data found in SSR response; the page schema may have changed or the workspace may not have a Go subscription")
	}

	var periods []models.UsagePeriod
	now := time.Now().UTC()

	for _, m := range matches {
		name := m[1]
		resetSec, _ := strconv.ParseFloat(m[2], 64)
		pct, _ := strconv.ParseFloat(m[3], 64)

		var periodName string
		var periodType models.PeriodType

		switch name {
		case "rollingUsage":
			periodName = "Rolling 5-Hour"
			periodType = models.PeriodSession
		case "weeklyUsage":
			periodName = "Weekly"
			periodType = models.PeriodWeekly
		default:
			continue
		}

		resetsAt := now.Add(time.Duration(resetSec) * time.Second)
		periods = append(periods, models.UsagePeriod{
			Name:        periodName,
			Utilization: models.ClampPct(int(pct)),
			PeriodType:  periodType,
			ResetsAt:    &resetsAt,
		})
	}

	if len(periods) == 0 {
		return nil, fmt.Errorf("no recognized usage windows found (expected rollingUsage or weeklyUsage)")
	}

	snapshot := &models.UsageSnapshot{
		Provider:  "opencode",
		FetchedAt: now,
		Periods:   periods,
		Source:    "web",
	}

	if strings.Contains(body, `subscriptionPlan:null`) || strings.Contains(body, `subscription:null`) {
		if strings.Contains(body, `lite.subscription.get`) || strings.Contains(body, `rollingUsage`) {
			snapshot.Identity = &models.ProviderIdentity{Plan: "go"}
		}
	}

	return snapshot, nil
}
