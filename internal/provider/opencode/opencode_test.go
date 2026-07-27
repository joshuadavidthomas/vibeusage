package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

const openCodeUsageBody = `rollingUsage:{status:"ok",resetInSec:3600,usagePercent:10},weeklyUsage:{status:"ok",resetInSec:7200,usagePercent:20}`

func isolateOpenCodeTest(t *testing.T) {
	t.Helper()
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	t.Setenv("OPENCODE_WORKSPACE_ID", "")
}

func writeSessionCredential(t *testing.T, content string) {
	t.Helper()
	if err := config.WriteCredential("opencode", "session", []byte(content)); err != nil {
		t.Fatalf("write session credential: %v", err)
	}
}

func TestMeta(t *testing.T) {
	o := Opencode{}
	m := o.Meta()
	if m.ID != "opencode" {
		t.Errorf("id = %q, want %q", m.ID, "opencode")
	}
	if m.Name != "OpenCode" {
		t.Errorf("name = %q, want %q", m.Name, "OpenCode")
	}
	if m.Homepage != "https://opencode.ai" {
		t.Errorf("homepage = %q, want %q", m.Homepage, "https://opencode.ai")
	}
}

func TestAuth(t *testing.T) {
	o := Opencode{}
	a := o.Auth()
	if a == nil {
		t.Fatal("expected non-nil auth flow")
	}
}

func TestCredentialSources(t *testing.T) {
	cs := (Opencode{}).CredentialSources()
	if len(cs.EnvVars) != 0 || len(cs.CLIPaths) != 0 {
		t.Errorf("credential sources = %#v, want none", cs)
	}
}

func TestWebStrategy_IsAvailable_NoCredential(t *testing.T) {
	isolateOpenCodeTest(t)

	if (&WebStrategy{}).IsAvailable() {
		t.Error("expected IsAvailable to be false without credential")
	}
}

func TestLoadSessionToken_ExactShape(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"  session-value  "}`)

	token, err := (&WebStrategy{}).loadSessionToken()
	if err != nil {
		t.Fatalf("loadSessionToken() error = %v", err)
	}
	if token != "session-value" {
		t.Errorf("loadSessionToken() = %q, want session-value", token)
	}
}

func TestLoadSessionToken_RejectsAlias(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"token":"legacy-value"}`)

	token, err := (&WebStrategy{}).loadSessionToken()
	if err != nil {
		t.Fatalf("loadSessionToken() error = %v", err)
	}
	if token != "" {
		t.Errorf("loadSessionToken() = %q, want empty token", token)
	}
}

func TestLoadSessionToken_ReportsParseError(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `"not-an-object"`)

	_, err := (&WebStrategy{}).loadSessionToken()
	if err == nil {
		t.Fatal("expected credential parse error")
	}
	if !strings.Contains(err.Error(), "parsing OpenCode session credential") {
		t.Errorf("error = %q, want credential parse context", err)
	}
}

func TestWorkspaceID_ConfigPrecedesEnvironment(t *testing.T) {
	isolateOpenCodeTest(t)
	t.Setenv("OPENCODE_WORKSPACE_ID", "wrk_env")
	cfg := config.DefaultConfig()
	cfg.Providers["opencode"] = config.ProviderConfig{WorkspaceID: "  wrk_config  "}
	config.Override(t, cfg)

	if got := workspaceID(); got != "wrk_config" {
		t.Errorf("workspaceID() = %q, want wrk_config", got)
	}
}

func TestWorkspaceID_EnvironmentFallback(t *testing.T) {
	isolateOpenCodeTest(t)
	t.Setenv("OPENCODE_WORKSPACE_ID", "  wrk_env  ")
	config.Override(t, config.DefaultConfig())

	if got := workspaceID(); got != "wrk_env" {
		t.Errorf("workspaceID() = %q, want wrk_env", got)
	}
}

func TestFetch_NoToken(t *testing.T) {
	isolateOpenCodeTest(t)

	result, err := (&WebStrategy{}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success || result.ShouldFallback {
		t.Fatalf("result = %#v, want fatal credential failure", result)
	}
	if !strings.Contains(result.Error, "auth opencode") {
		t.Errorf("error = %q, want auth hint", result.Error)
	}
}

func TestFetch_DiscoversWorkspaceAndZenBalance(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"session-value"}`)
	config.Override(t, config.DefaultConfig())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil || cookie.Value != "session-value" {
			t.Errorf("auth cookie = %#v, %v", cookie, err)
		}
		switch r.URL.Path {
		case "/auth":
			http.Redirect(w, r, "/workspace/wrk_discovered", http.StatusFound)
		case "/workspace/wrk_discovered":
			_, _ = w.Write([]byte(`{customerID:"cus_123",balance:$R[2]=2375000000,reload:!1}`))
		case "/workspace/wrk_discovered/go":
			_, _ = w.Write([]byte(`<html>workspace has Zen but no Go subscription</html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	result, err := (&WebStrategy{baseURL: srv.URL}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.Snapshot == nil {
		t.Fatalf("result = %#v, want successful discovered workspace fetch", result)
	}
	if result.Snapshot.Billing == nil || result.Snapshot.Billing.Balance == nil {
		t.Fatal("expected Zen balance")
	}
	if got := *result.Snapshot.Billing.Balance; got != 23.75 {
		t.Errorf("Zen balance = %v, want 23.75", got)
	}
	if len(result.Snapshot.Periods) != 0 {
		t.Errorf("periods = %#v, want none without a Go subscription", result.Snapshot.Periods)
	}
}

func TestFetch_ConfigWorkspaceOverridesDiscovery(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"session-value"}`)
	cfg := config.DefaultConfig()
	cfg.Providers["opencode"] = config.ProviderConfig{WorkspaceID: "wrk_config"}
	config.Override(t, cfg)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			t.Error("workspace discovery should not run with a configured workspace")
		case "/workspace/wrk_config":
			_, _ = w.Write([]byte(`{customerID:"cus_123",balance:0}`))
		case "/workspace/wrk_config/go":
			_, _ = w.Write([]byte(openCodeUsageBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	result, err := (&WebStrategy{baseURL: srv.URL}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.Snapshot == nil {
		t.Fatalf("result = %#v, want successful configured workspace fetch", result)
	}
	if result.Snapshot.Billing == nil || result.Snapshot.Billing.Balance == nil || *result.Snapshot.Billing.Balance != 0 {
		t.Errorf("billing = %#v, want zero Zen balance", result.Snapshot.Billing)
	}
}

func TestFetch_DiscoveryFailures(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.HandlerFunc
		wantFallback bool
		wantError    string
	}{
		{
			name: "invalid session",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/auth" {
					http.Redirect(w, r, "/auth/authorize", http.StatusFound)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			wantError: "auth opencode",
		},
		{
			name: "server failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantFallback: true,
			wantError:    "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOpenCodeTest(t)
			writeSessionCredential(t, `{"session_token":"session-value"}`)
			config.Override(t, config.DefaultConfig())
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			result, err := (&WebStrategy{baseURL: srv.URL}).Fetch(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success || result.ShouldFallback != tt.wantFallback {
				t.Fatalf("result = %#v, want fallback = %t", result, tt.wantFallback)
			}
			if !strings.Contains(result.Error, tt.wantError) {
				t.Errorf("error = %q, want substring %q", result.Error, tt.wantError)
			}
		})
	}
}

func TestFetch_ZenFailureDoesNotDiscardUsage(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"session-value"}`)
	cfg := config.DefaultConfig()
	cfg.Providers["opencode"] = config.ProviderConfig{WorkspaceID: "wrk_config"}
	config.Override(t, cfg)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspace/wrk_config":
			w.WriteHeader(http.StatusInternalServerError)
		case "/workspace/wrk_config/go":
			_, _ = w.Write([]byte(openCodeUsageBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	result, err := (&WebStrategy{baseURL: srv.URL}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || result.Snapshot == nil {
		t.Fatalf("result = %#v, want successful usage fetch", result)
	}
	if result.Snapshot.Billing != nil {
		t.Errorf("billing = %#v, want nil after supplemental request failure", result.Snapshot.Billing)
	}
}

func TestFetch_SlowZenRequestDoesNotDiscardUsage(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"session-value"}`)
	cfg := config.DefaultConfig()
	cfg.Providers["opencode"] = config.ProviderConfig{WorkspaceID: "wrk_config"}
	config.Override(t, cfg)

	billingStopped := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspace/wrk_config":
			<-r.Context().Done()
			close(billingStopped)
		case "/workspace/wrk_config/go":
			_, _ = w.Write([]byte(openCodeUsageBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	outcome := fetch.ExecutePipeline(
		context.Background(),
		"opencode",
		[]fetch.Strategy{&WebStrategy{HTTPTimeout: 30, baseURL: srv.URL}},
		false,
		fetch.PipelineConfig{Timeout: 400 * time.Millisecond},
	)
	if !outcome.Success || outcome.Snapshot == nil {
		t.Fatalf("outcome = %#v, want fresh usage despite slow Zen request", outcome)
	}
	select {
	case <-billingStopped:
	default:
		t.Fatal("supplemental Zen request still running after fetch returned")
	}
}

func TestFetchUsage_ClassifiesResponses(t *testing.T) {
	usageBody := openCodeUsageBody
	tests := []struct {
		name         string
		status       int
		body         string
		wantSuccess  bool
		wantFallback bool
		wantError    string
	}{
		{name: "success", status: http.StatusOK, body: usageBody, wantSuccess: true},
		{name: "invalid session", status: http.StatusUnauthorized, wantError: "auth opencode"},
		{name: "missing workspace", status: http.StatusNotFound, wantError: "workspace not found"},
		{name: "server failure", status: http.StatusInternalServerError, wantFallback: true, wantError: "HTTP 500"},
		{name: "schema drift", status: http.StatusOK, body: `<html>changed</html>`, wantError: "parsing OpenCode usage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cookie, err := r.Cookie("auth")
				if err != nil || cookie.Value != "session-value" {
					t.Errorf("auth cookie = %#v, %v", cookie, err)
				}
				if got := r.Header.Get("User-Agent"); got != "Mozilla/5.0" {
					t.Errorf("User-Agent = %q, want Mozilla/5.0", got)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			result := (&WebStrategy{}).fetchUsage(
				context.Background(),
				httpclient.New(),
				srv.URL+"/workspace/wrk_test/go",
				"wrk_test",
				"session-value",
			)
			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %t, want %t", result.Success, tt.wantSuccess)
			}
			if result.ShouldFallback != tt.wantFallback {
				t.Errorf("ShouldFallback = %t, want %t", result.ShouldFallback, tt.wantFallback)
			}
			if tt.wantError != "" && !strings.Contains(result.Error, tt.wantError) {
				t.Errorf("Error = %q, want substring %q", result.Error, tt.wantError)
			}
		})
	}
}

func TestFetchUsage_RejectsRedirectsOutsideRequestedWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		wantError string
	}{
		{name: "authentication", target: "/auth/authorize", wantError: "auth opencode"},
		{name: "other workspace", target: "/workspace/wrk_other/go", wantError: "different workspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/workspace/wrk_test/go" {
					http.Redirect(w, r, tt.target, http.StatusFound)
					return
				}
				if r.URL.Path != tt.target {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write([]byte(openCodeUsageBody))
			}))
			defer srv.Close()

			result := (&WebStrategy{}).fetchUsage(
				context.Background(),
				httpclient.New(),
				srv.URL+"/workspace/wrk_test/go",
				"wrk_test",
				"session-value",
			)
			if result.Success || result.ShouldFallback {
				t.Fatalf("result = %#v, want fatal redirect failure", result)
			}
			if !strings.Contains(result.Error, tt.wantError) {
				t.Errorf("error = %q, want substring %q", result.Error, tt.wantError)
			}
		})
	}
}

func TestFetchUsage_RejectsRedirectToAnotherOrigin(t *testing.T) {
	otherOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(openCodeUsageBody))
	}))
	defer otherOrigin.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherOrigin.URL+"/workspace/wrk_test/go", http.StatusFound)
	}))
	defer srv.Close()

	result := (&WebStrategy{}).fetchUsage(
		context.Background(),
		httpclient.New(),
		srv.URL+"/workspace/wrk_test/go",
		"wrk_test",
		"session-value",
	)
	if result.Success || result.ShouldFallback {
		t.Fatalf("result = %#v, want fatal origin failure", result)
	}
	if !strings.Contains(result.Error, "unexpected origin") {
		t.Errorf("error = %q, want unexpected origin", result.Error)
	}
}

func TestFetchUsage_NetworkFailureAllowsCacheFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL + "/workspace/wrk_test/go"
	srv.Close()

	result := (&WebStrategy{}).fetchUsage(context.Background(), httpclient.New(), url, "wrk_test", "session-value")
	if result.Success || !result.ShouldFallback {
		t.Fatalf("result = %#v, want fallback-eligible network failure", result)
	}
	if !strings.Contains(result.Error, "OpenCode request failed") {
		t.Errorf("error = %q, want request failure context", result.Error)
	}
}

func TestWorkspaceIDFromURL(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://opencode.ai/workspace/wrk_default", want: "wrk_default"},
		{rawURL: "https://opencode.ai/en/workspace/wrk_localized", want: "wrk_localized"},
		{rawURL: "https://opencode.ai/auth/authorize"},
		{rawURL: "https://opencode.ai/workspace/not-a-workspace"},
	}

	for _, tt := range tests {
		parsed, err := url.Parse(tt.rawURL)
		if err != nil {
			t.Fatalf("parse test URL: %v", err)
		}
		if got := workspaceIDFromURL(parsed); got != tt.want {
			t.Errorf("workspaceIDFromURL(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestParseZenBillingFromSSR(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantBalance      float64
		wantBalanceFound bool
	}{
		{name: "serialized reference", body: `{customerID:"cus_123",balance:$R[2]=2375000000,reload:!1}`, wantBalance: 23.75, wantBalanceFound: true},
		{name: "quoted fields", body: `{"customerID":"cus_123","balance":0}`, wantBalanceFound: true},
		{name: "overspent", body: `{customerID:"cus_123",balance:-125000000}`, wantBalance: -1.25, wantBalanceFound: true},
		{name: "no billing data", body: `{rollingUsage:{usagePercent:10}}`},
		{name: "balance without customer", body: `{balance:500000000}`},
		{name: "similarly named field", body: `{customerID:"cus_123",credit_balance:500000000}`},
		{name: "unrelated balance first", body: `{balance:500000000},{customerID:"cus_123",balance:2375000000}`, wantBalance: 23.75, wantBalanceFound: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			billing := parseZenBillingFromSSR(tt.body)
			if !tt.wantBalanceFound {
				if billing != nil {
					t.Fatalf("billing = %#v, want nil", billing)
				}
				return
			}
			if billing == nil || billing.Balance == nil {
				t.Fatal("expected billing balance")
			}
			if *billing.Balance != tt.wantBalance {
				t.Errorf("balance = %v, want %v", *billing.Balance, tt.wantBalance)
			}
		})
	}
}

func TestParseUsageFromSSR_Full(t *testing.T) {
	html := `some html before
$R[28]($R[18],{mine:true,rollingUsage:{status:"ok",resetInSec:13553,usagePercent:1},weeklyUsage:{status:"ok",resetInSec:92824,usagePercent:41},monthlyUsage:{status:"ok",resetInSec:2629167,usagePercent:20}});
some html after`

	snapshot, err := parseUsageFromSSR(html)
	if err != nil {
		t.Fatalf("parseUsageFromSSR failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "opencode" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "opencode")
	}
	if snapshot.Source != "web" {
		t.Errorf("source = %q, want %q", snapshot.Source, "web")
	}
	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2", len(snapshot.Periods))
	}

	r5h := snapshot.Periods[0]
	if r5h.Name != "Rolling 5-Hour" {
		t.Errorf("name = %q, want %q", r5h.Name, "Rolling 5-Hour")
	}
	if r5h.Utilization != 1 {
		t.Errorf("utilization = %d, want 1", r5h.Utilization)
	}
	if r5h.PeriodType != models.PeriodSession {
		t.Errorf("period_type = %q, want %q", r5h.PeriodType, models.PeriodSession)
	}
	if r5h.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}

	w := snapshot.Periods[1]
	if w.Name != "Weekly" {
		t.Errorf("name = %q, want %q", w.Name, "Weekly")
	}
	if w.Utilization != 41 {
		t.Errorf("utilization = %d, want 41", w.Utilization)
	}
	if w.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q", w.PeriodType, models.PeriodWeekly)
	}
}

func TestParseUsageFromSSR_NoData(t *testing.T) {
	html := `<html><body>no usage data here</body></html>`
	_, err := parseUsageFromSSR(html)
	if err == nil {
		t.Fatal("expected error for no usage data")
	}
	if strings.Contains(strings.ToLower(err.Error()), "session") {
		t.Errorf("parse error = %q, should not suggest an auth failure", err)
	}
}

func TestParseUsageFromSSR_MonthlyOnly(t *testing.T) {
	html := `monthlyUsage:{status:"ok",resetInSec:3600,usagePercent:50}`
	_, err := parseUsageFromSSR(html)
	if err == nil {
		t.Fatal("expected error when only monthlyUsage is present")
	}
}

func TestParseUsageFromSSR_UtilizationClamping(t *testing.T) {
	html := `rollingUsage:{status:"ok",resetInSec:3600,usagePercent:150}`

	snapshot, err := parseUsageFromSSR(html)
	if err != nil {
		t.Fatalf("parseUsageFromSSR failed: %v", err)
	}
	if snapshot.Periods[0].Utilization != 100 {
		t.Errorf("utilization = %d, want 100", snapshot.Periods[0].Utilization)
	}
}

func TestParseUsageFromSSR_ZeroUsage(t *testing.T) {
	html := `rollingUsage:{status:"ok",resetInSec:3600,usagePercent:0}`

	snapshot, err := parseUsageFromSSR(html)
	if err != nil {
		t.Fatalf("parseUsageFromSSR failed: %v", err)
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0", snapshot.Periods[0].Utilization)
	}
}

func TestParseUsageFromSSR_ResetsAtInFuture(t *testing.T) {
	before := time.Now()
	html := `rollingUsage:{status:"ok",resetInSec:3600,usagePercent:10}`

	snapshot, err := parseUsageFromSSR(html)
	if err != nil {
		t.Fatalf("parseUsageFromSSR failed: %v", err)
	}
	after := time.Now().Add(3600 * time.Second)
	if snapshot.Periods[0].ResetsAt == nil {
		t.Fatal("expected resets_at")
	}
	if snapshot.Periods[0].ResetsAt.Before(before) {
		t.Error("resets_at is in the past")
	}
	if snapshot.Periods[0].ResetsAt.After(after.Add(time.Second)) {
		t.Error("resets_at is too far in the future")
	}
}

func TestParseUsageFromSSR_IdentityPlanGo(t *testing.T) {
	html := `subscription:null lite.subscription.get some content rollingUsage:{status:"ok",resetInSec:3600,usagePercent:10}`

	snapshot, err := parseUsageFromSSR(html)
	if err != nil {
		t.Fatalf("parseUsageFromSSR failed: %v", err)
	}
	if snapshot.Identity == nil {
		t.Fatal("expected identity for Go plan")
	}
	if snapshot.Identity.Plan != "go" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "go")
	}
}

func TestParseUsageFromSSR_ClampPct(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{`rollingUsage:{status:"ok",resetInSec:3600,usagePercent:45}`, 45},
		{`rollingUsage:{status:"ok",resetInSec:3600,usagePercent:0}`, 0},
		{`rollingUsage:{status:"ok",resetInSec:3600,usagePercent:150}`, 100},
	}

	for _, tt := range tests {
		snapshot, err := parseUsageFromSSR(tt.input)
		if err != nil {
			t.Fatalf("parseUsageFromSSR failed: %v", err)
		}
		if snapshot.Periods[0].Utilization != tt.expected {
			t.Errorf("utilization = %d, want %d", snapshot.Periods[0].Utilization, tt.expected)
		}
	}
}
