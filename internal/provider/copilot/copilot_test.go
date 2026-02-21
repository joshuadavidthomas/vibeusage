package copilot

import (
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseTypedUsageResponse_FullResponse(t *testing.T) {
	resp := UserResponse{
		QuotaResetDateUTC: "2025-03-01T00:00:00Z",
		CopilotPlan:       "pro",
		QuotaSnapshots: &QuotaSnapshots{
			PremiumInteractions: &Quota{Entitlement: 100, Remaining: 60, Unlimited: false},
			Chat:                &Quota{Entitlement: 500, Remaining: 300, Unlimited: false},
			Completions:         &Quota{Entitlement: 0, Remaining: 0, Unlimited: true},
		},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "copilot" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "copilot")
	}
	if snapshot.Source != "device_flow" {
		t.Errorf("source = %q, want %q", snapshot.Source, "device_flow")
	}

	if len(snapshot.Periods) != 3 {
		t.Fatalf("len(periods) = %d, want 3", len(snapshot.Periods))
	}

	premium := snapshot.Periods[0]
	if premium.Name != "Monthly (Premium)" {
		t.Errorf("period[0].name = %q, want %q", premium.Name, "Monthly (Premium)")
	}
	if premium.Utilization != 40 {
		t.Errorf("premium utilization = %d, want 40", premium.Utilization)
	}
	if premium.PeriodType != models.PeriodMonthly {
		t.Errorf("premium period_type = %q, want %q", premium.PeriodType, models.PeriodMonthly)
	}
	if premium.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}
	expectedReset := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !premium.ResetsAt.Equal(expectedReset) {
		t.Errorf("resets_at = %v, want %v", premium.ResetsAt, expectedReset)
	}

	chat := snapshot.Periods[1]
	if chat.Name != "Monthly (Chat)" {
		t.Errorf("period[1].name = %q, want %q", chat.Name, "Monthly (Chat)")
	}
	if chat.Utilization != 40 {
		t.Errorf("chat utilization = %d, want 40", chat.Utilization)
	}

	completions := snapshot.Periods[2]
	if completions.Name != "Monthly (Completions)" {
		t.Errorf("period[2].name = %q, want %q", completions.Name, "Monthly (Completions)")
	}
	if completions.Utilization != 0 {
		t.Errorf("completions utilization = %d, want 0", completions.Utilization)
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "pro")
	}
}

func TestParseTypedUsageResponse_NoQuotas(t *testing.T) {
	resp := UserResponse{
		CopilotPlan: "free",
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot != nil {
		t.Error("expected nil snapshot when no quotas")
	}
}

func TestParseTypedUsageResponse_EmptyQuotas(t *testing.T) {
	resp := UserResponse{
		QuotaSnapshots: &QuotaSnapshots{},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot != nil {
		t.Error("expected nil snapshot when all quotas nil")
	}
}

func TestParseTypedUsageResponse_ZeroEntitlementNotUnlimited(t *testing.T) {
	resp := UserResponse{
		QuotaSnapshots: &QuotaSnapshots{
			PremiumInteractions: &Quota{Entitlement: 0, Remaining: 0, Unlimited: false},
			Chat:                &Quota{Entitlement: 100, Remaining: 50, Unlimited: false},
		},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	// Only chat should appear (premium has no usage)
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Name != "Monthly (Chat)" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Monthly (Chat)")
	}
}

func TestParseTypedUsageResponse_NoPlan(t *testing.T) {
	resp := UserResponse{
		QuotaSnapshots: &QuotaSnapshots{
			Chat: &Quota{Entitlement: 100, Remaining: 50, Unlimited: false},
		},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no plan")
	}
}

func TestParseTypedUsageResponse_ResetDateWithZ(t *testing.T) {
	resp := UserResponse{
		QuotaResetDateUTC: "2025-03-01T00:00:00Z",
		QuotaSnapshots: &QuotaSnapshots{
			Chat: &Quota{Entitlement: 100, Remaining: 50, Unlimited: false},
		},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].ResetsAt == nil {
		t.Fatal("expected resets_at")
	}
}

func TestParseTypedUsageResponse_InvalidResetDate(t *testing.T) {
	resp := UserResponse{
		QuotaResetDateUTC: "not-a-date",
		QuotaSnapshots: &QuotaSnapshots{
			Chat: &Quota{Entitlement: 100, Remaining: 50, Unlimited: false},
		},
	}

	s := DeviceFlowStrategy{}
	snapshot := s.parseTypedUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].ResetsAt != nil {
		t.Error("expected nil resets_at for invalid date")
	}
}
