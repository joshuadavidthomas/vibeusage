package opencode

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

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
	o := Opencode{}
	cs := o.CredentialSources()
	if len(cs.EnvVars) == 0 {
		t.Error("expected at least one env var")
	}
}

func TestWebStrategy_IsAvailable_NoCredential(t *testing.T) {
	s := WebStrategy{}
	if s.IsAvailable() {
		t.Error("expected IsAvailable to be false without credential")
	}
}

func TestFetch_NoToken(t *testing.T) {
	s := WebStrategy{}
	result, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure")
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

func TestDiscoverWorkspaceID_FromWorkspaceLink(t *testing.T) {
	body := `<a href="/workspace/wrk_01KYC68MEBCSSKQ3Y8D1MG67TG">Default</a>`
	re := regexp.MustCompile(`/workspace/(wrk_[a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		t.Fatal("expected workspace ID match")
	}
	if matches[1] != "wrk_01KYC68MEBCSSKQ3Y8D1MG67TG" {
		t.Errorf("id = %q", matches[1])
	}
}
