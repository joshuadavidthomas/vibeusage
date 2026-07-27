package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

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
	if m.Name != "OpenCode Go" {
		t.Errorf("name = %q, want %q", m.Name, "OpenCode Go")
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

func TestFetch_MissingWorkspace(t *testing.T) {
	isolateOpenCodeTest(t)
	writeSessionCredential(t, `{"session_token":"session-value"}`)
	config.Override(t, config.DefaultConfig())

	result, err := (&WebStrategy{}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success || result.ShouldFallback {
		t.Fatalf("result = %#v, want fatal configuration failure", result)
	}
	if !strings.Contains(result.Error, "workspace ID is required") {
		t.Errorf("error = %q, want workspace configuration hint", result.Error)
	}
}

func TestFetchUsage_ClassifiesResponses(t *testing.T) {
	usageBody := `rollingUsage:{status:"ok",resetInSec:3600,usagePercent:10},weeklyUsage:{status:"ok",resetInSec:7200,usagePercent:20}`
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
				srv.URL,
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

func TestFetchUsage_NetworkFailureAllowsCacheFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	result := (&WebStrategy{}).fetchUsage(context.Background(), httpclient.New(), url, "session-value")
	if result.Success || !result.ShouldFallback {
		t.Fatalf("result = %#v, want fallback-eligible network failure", result)
	}
	if !strings.Contains(result.Error, "OpenCode request failed") {
		t.Errorf("error = %q, want request failure context", result.Error)
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
