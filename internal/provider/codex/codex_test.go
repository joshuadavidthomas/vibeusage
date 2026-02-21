package codex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseUsageResponse_FullResponse(t *testing.T) {
	resp := UsageResponse{
		RateLimit: &RateLimits{
			PrimaryWindow: &RateWindow{
				UsedPercent: 42.0,
				ResetAt:     1740000000,
			},
			SecondaryWindow: &RateWindow{
				UsedPercent: 75.0,
				ResetAt:     1740100000,
			},
		},
		Credits:  &Credits{HasCredits: true, RawBalance: json.RawMessage(`50.0`)},
		PlanType: "plus",
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "codex" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "codex")
	}
	if snapshot.Source != "oauth" {
		t.Errorf("source = %q, want %q", snapshot.Source, "oauth")
	}

	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2", len(snapshot.Periods))
	}

	session := snapshot.Periods[0]
	if session.Name != "Session" {
		t.Errorf("period[0].name = %q, want %q", session.Name, "Session")
	}
	if session.Utilization != 42 {
		t.Errorf("session utilization = %d, want 42", session.Utilization)
	}
	if session.PeriodType != models.PeriodSession {
		t.Errorf("session period_type = %q, want %q", session.PeriodType, models.PeriodSession)
	}
	if session.ResetsAt == nil {
		t.Fatal("expected session resets_at")
	}
	expectedReset := time.Unix(1740000000, 0).UTC()
	if !session.ResetsAt.Equal(expectedReset) {
		t.Errorf("session resets_at = %v, want %v", session.ResetsAt, expectedReset)
	}

	weekly := snapshot.Periods[1]
	if weekly.Name != "Weekly" {
		t.Errorf("period[1].name = %q, want %q", weekly.Name, "Weekly")
	}
	if weekly.Utilization != 75 {
		t.Errorf("weekly utilization = %d, want 75", weekly.Utilization)
	}
	if weekly.PeriodType != models.PeriodWeekly {
		t.Errorf("weekly period_type = %q, want %q", weekly.PeriodType, models.PeriodWeekly)
	}

	if snapshot.Overage == nil {
		t.Fatal("expected overage")
	}
	if snapshot.Overage.Limit != 50.0 {
		t.Errorf("overage limit = %v, want 50.0", snapshot.Overage.Limit)
	}
	if !snapshot.Overage.IsEnabled {
		t.Error("expected overage to be enabled")
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "plus" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "plus")
	}
}

func TestParseUsageResponse_AlternateKeys(t *testing.T) {
	resp := UsageResponse{
		RateLimits: &RateLimits{
			Primary: &RateWindow{
				UsedPercent:    30.0,
				ResetTimestamp: 1740000000,
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Utilization != 30 {
		t.Errorf("utilization = %d, want 30", snapshot.Periods[0].Utilization)
	}
	if snapshot.Periods[0].ResetsAt == nil {
		t.Fatal("expected resets_at to be set via reset_timestamp")
	}
}

func TestParseUsageResponse_NoRateLimits(t *testing.T) {
	resp := UsageResponse{
		PlanType: "free",
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot != nil {
		t.Error("expected nil snapshot when no rate limits")
	}
}

func TestParseUsageResponse_NoCredits(t *testing.T) {
	resp := UsageResponse{
		RateLimit: &RateLimits{
			PrimaryWindow: &RateWindow{UsedPercent: 10.0},
		},
		Credits: &Credits{HasCredits: false, RawBalance: json.RawMessage(`0`)},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected nil overage when no credits")
	}
}

func TestParseUsageResponse_NoPlanType(t *testing.T) {
	resp := UsageResponse{
		RateLimit: &RateLimits{
			PrimaryWindow: &RateWindow{UsedPercent: 10.0},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no plan_type")
	}
}

func TestParseUsageResponse_NoResetTimestamp(t *testing.T) {
	resp := UsageResponse{
		RateLimit: &RateLimits{
			PrimaryWindow: &RateWindow{UsedPercent: 50.0},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].ResetsAt != nil {
		t.Error("expected nil resets_at when no timestamp")
	}
}
