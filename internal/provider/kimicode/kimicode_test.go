package kimicode

import (
	"context"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestAPIKeyStrategy_IsAvailable_EnvVar(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "test-key")
	s := &APIKeyStrategy{}
	if !s.IsAvailable() {
		t.Error("IsAvailable() = false, want true when KIMI_CODE_API_KEY is set")
	}
}

func TestAPIKeyStrategy_IsAvailable_NoEnvVar(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "")
	s := &APIKeyStrategy{}
	if s.loadAPIKey() == "" && s.IsAvailable() {
		t.Error("IsAvailable() = true, want false when no API key available")
	}
}

func TestAPIKeyStrategy_Fetch_NoKey(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "")
	s := &APIKeyStrategy{}
	if s.loadAPIKey() != "" {
		t.Skip("credential file present — skipping no-key test")
	}

	result, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected non-successful result when no key available")
	}
}

func TestDeviceFlowStrategy_Fetch_NoCredentials(t *testing.T) {
	s := &DeviceFlowStrategy{}
	if s.loadCredentials() != nil {
		t.Skip("credential file present — skipping no-cred test")
	}

	result, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected non-successful result when no credentials")
	}
}

func TestParseUsageResponse_FullResponse(t *testing.T) {
	resp := UsageResponse{
		User: &User{
			Membership: &Membership{Level: "LEVEL_PRO"},
		},
		Usage: &UsageDetail{
			Limit:     "100",
			Used:      "40",
			Remaining: "60",
			ResetTime: "2026-03-01T00:00:00Z",
		},
		Limits: []Limit{
			{
				Window: Window{Duration: 300, TimeUnit: "TIME_UNIT_MINUTE"},
				Detail: &UsageDetail{
					Limit:     "100",
					Remaining: "60",
					ResetTime: "2026-03-01T00:00:00Z",
				},
			},
		},
	}

	snapshot := parseUsageResponse(resp, "device_flow")

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "kimicode" {
		t.Errorf("Provider = %q, want %q", snapshot.Provider, "kimicode")
	}
	if snapshot.Source != "device_flow" {
		t.Errorf("Source = %q, want %q", snapshot.Source, "device_flow")
	}
	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Pro" {
		t.Errorf("Plan = %q, want %q", snapshot.Identity.Plan, "Pro")
	}
	// Both rows shown: weekly usage counter + per-window rate limit.
	if len(snapshot.Periods) != 2 {
		t.Fatalf("expected 2 periods (weekly + session limit), got %d", len(snapshot.Periods))
	}

	// First: weekly usage summary.
	weekly := snapshot.Periods[0]
	if weekly.Name != "Weekly" {
		t.Errorf("Periods[0].Name = %q, want %q", weekly.Name, "Weekly")
	}
	if weekly.PeriodType != models.PeriodWeekly {
		t.Errorf("Periods[0].PeriodType = %q, want %q", weekly.PeriodType, models.PeriodWeekly)
	}
	if weekly.Utilization != 40 {
		t.Errorf("Periods[0].Utilization = %d, want 40", weekly.Utilization)
	}

	// Second: 5h rate limit window.
	session := snapshot.Periods[1]
	if session.Name != "Session (5h)" {
		t.Errorf("Periods[1].Name = %q, want %q", session.Name, "Session (5h)")
	}
	if session.PeriodType != models.PeriodSession {
		t.Errorf("Periods[1].PeriodType = %q, want %q", session.PeriodType, models.PeriodSession)
	}
	if session.ResetsAt == nil {
		t.Fatal("expected Periods[1].ResetsAt to be set")
	}
}

func TestParseUsageResponse_APIKeySource(t *testing.T) {
	resp := UsageResponse{
		Usage: &UsageDetail{
			Limit:     "100",
			Remaining: "100",
			ResetTime: "2026-03-01T00:00:00Z",
		},
	}

	snapshot := parseUsageResponse(resp, "api_key")

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Source != "api_key" {
		t.Errorf("Source = %q, want %q", snapshot.Source, "api_key")
	}
}

func TestParseUsageResponse_NoData(t *testing.T) {
	resp := UsageResponse{}

	snapshot := parseUsageResponse(resp, "api_key")

	if snapshot != nil {
		t.Error("expected nil snapshot when no usage data")
	}
}

func TestParseUsageResponse_NoIdentity(t *testing.T) {
	resp := UsageResponse{
		Usage: &UsageDetail{
			Limit:     "100",
			Remaining: "50",
		},
	}

	snapshot := parseUsageResponse(resp, "device_flow")

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no membership")
	}
}

func TestKimiCode_Meta(t *testing.T) {
	k := KimiCode{}
	meta := k.Meta()
	if meta.ID != "kimicode" {
		t.Errorf("ID = %q, want %q", meta.ID, "kimicode")
	}
	if meta.Name != "Kimi Code" {
		t.Errorf("Name = %q, want %q", meta.Name, "Kimi Code")
	}
}
