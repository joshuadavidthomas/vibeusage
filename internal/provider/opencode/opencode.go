package opencode

import (
	"context"
	"encoding/json"
	"fmt"
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
	return provider.CredentialInfo{
		EnvVars: []string{"OPENCODE_SESSION_TOKEN"},
	}
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

func (s *WebStrategy) IsAvailable() bool {
	return config.HasCredential("opencode", "session")
}

func (s *WebStrategy) loadSessionToken() string {
	data, err := config.ReadCredential("opencode", "session")
	if err != nil || data == nil {
		return ""
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	for _, key := range []string{"session_token", "token", "session_key", "session"} {
		if v := raw[key]; v != "" {
			return v
		}
	}
	return ""
}

func (s *WebStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	sessionToken := s.loadSessionToken()
	if sessionToken == "" {
		return fetch.ResultFail("No session token found. Run `vibeusage auth opencode` to set one up."), nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	sessionCookie := httpclient.WithCookie("auth", sessionToken)
	userAgent := httpclient.WithHeader("User-Agent", "Mozilla/5.0")

	wsID := config.Get().Providers["opencode"].WorkspaceID
	if wsID == "" {
		wsID = os.Getenv("OPENCODE_WORKSPACE_ID")
	}
	if wsID == "" {
		return fetch.ResultFatal("workspace ID is required. Set it via `vibeusage config edit` under [providers.opencode] workspace_id = \"...\" or set OPENCODE_WORKSPACE_ID"), nil
	}

	usageURL := fmt.Sprintf("https://opencode.ai/workspace/%s/go", wsID)
	result, err := s.fetchUsage(ctx, client, usageURL, sessionCookie, userAgent)
	if err != nil {
		return fetch.ResultFatal(err.Error()), nil
	}
	return fetch.ResultOK(*result), nil
}

func (s *WebStrategy) fetchUsage(ctx context.Context, client *httpclient.Client, url string, sessionCookie, userAgent httpclient.RequestOption) (*models.UsageSnapshot, error) {
	resp, err := client.DoCtx(ctx, "GET", url, nil, sessionCookie, userAgent)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("session token expired or invalid. Run `vibeusage auth opencode` to re-authenticate")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenCode request failed: HTTP %d", resp.StatusCode)
	}

	body := string(resp.Body)
	return parseUsageFromSSR(body)
}

func parseUsageFromSSR(body string) (*models.UsageSnapshot, error) {
	re := regexp.MustCompile(`(rollingUsage|weeklyUsage|monthlyUsage):(?:\$R\[\d+\]=)?\{status:"[^"]+",resetInSec:(\d+),usagePercent:(\d+)\}`)
	matches := re.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no usage data found in SSR response. The workspace may not have a Go subscription, or the session is invalid")
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
