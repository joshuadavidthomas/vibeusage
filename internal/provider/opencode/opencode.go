package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type Opencode struct{}

func (o Opencode) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "opencode",
		Name:         "OpenCode Go",
		Description:  "OpenCode Go low-cost coding models subscription",
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
			"After auth, set your workspace ID in the config:\n" +
			"  vibeusage config edit\n" +
			"  Then add under [providers.opencode]: workspace_id = \"wrk_...\"",
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

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionToken, err := s.loadSessionToken()
	if err != nil {
		return fetch.ResultFatal(err.Error()), nil
	}
	if sessionToken == "" {
		return fetch.ResultFatal("no OpenCode session token found; run `vibeusage auth opencode` to set one up"), nil
	}

	wsID := workspaceID()
	if wsID == "" {
		return fetch.ResultFatal("workspace ID is required; set it via `vibeusage config edit` under [providers.opencode] workspace_id = \"...\" or set OPENCODE_WORKSPACE_ID"), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	usageURL := fmt.Sprintf("https://opencode.ai/workspace/%s/go", wsID)
	return s.fetchUsage(ctx, client, usageURL, sessionToken), nil
}

func (s *WebStrategy) fetchUsage(ctx context.Context, client *httpclient.Client, url, sessionToken string) fetch.FetchResult {
	resp, err := client.DoCtx(
		ctx,
		http.MethodGet,
		url,
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

	snapshot, err := parseUsageFromSSR(string(resp.Body))
	if err != nil {
		return fetch.ResultFatal(fmt.Sprintf("parsing OpenCode usage: %v", err))
	}
	return fetch.ResultOK(*snapshot)
}

func parseUsageFromSSR(body string) (*models.UsageSnapshot, error) {
	re := regexp.MustCompile(`(rollingUsage|weeklyUsage|monthlyUsage):(?:\$R\[\d+\]=)?\{status:"[^"]+",resetInSec:(\d+),usagePercent:(\d+)\}`)
	matches := re.FindAllStringSubmatch(body, -1)
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
