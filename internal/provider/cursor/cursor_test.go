package cursor

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func boolPtr(b bool) *bool          { return &b }
func float64Ptr(f float64) *float64 { return &f }

func TestParseTypedResponse_FullResponse(t *testing.T) {
	usage := UsageSummaryResponse{
		BillingCycleEnd: "2026-03-14T21:47:25.853Z",
		MembershipType:  "pro",
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled:          boolPtr(true),
				Used:             2322,
				Limit:            5000,
				Remaining:        2678,
				TotalPercentUsed: 46.44,
			},
			OnDemand: &OnDemandUsage{
				Enabled:   boolPtr(true),
				Used:      1500,
				Limit:     float64Ptr(10000),
				Remaining: float64Ptr(8500),
			},
		},
	}
	user := &UserMeResponse{Email: "user@example.com"}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, user)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "cursor" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "cursor")
	}
	if snapshot.Source != "web" {
		t.Errorf("source = %q, want %q", snapshot.Source, "web")
	}

	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}

	p := snapshot.Periods[0]
	if p.Name != "Plan Usage" {
		t.Errorf("name = %q, want %q", p.Name, "Plan Usage")
	}
	if p.Utilization != 46 {
		t.Errorf("utilization = %d, want 46", p.Utilization)
	}
	if p.PeriodType != models.PeriodMonthly {
		t.Errorf("period_type = %q, want %q", p.PeriodType, models.PeriodMonthly)
	}
	if p.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}

	if snapshot.Overage == nil {
		t.Fatal("expected overage")
	}
	if snapshot.Overage.Used != 15.0 {
		t.Errorf("overage used = %v, want 15.0", snapshot.Overage.Used)
	}
	if snapshot.Overage.Limit != 100.0 {
		t.Errorf("overage limit = %v, want 100.0", snapshot.Overage.Limit)
	}
	if snapshot.Overage.Currency != "USD" {
		t.Errorf("overage currency = %q, want %q", snapshot.Overage.Currency, "USD")
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", snapshot.Identity.Email, "user@example.com")
	}
	if snapshot.Identity.Plan != "pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "pro")
	}
}

func TestParseTypedResponse_NoPlanUsage(t *testing.T) {
	usage := UsageSummaryResponse{}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot != nil {
		t.Error("expected nil snapshot when no plan usage")
	}
}

func TestParseTypedResponse_NoOnDemand(t *testing.T) {
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled:          boolPtr(true),
				Used:             1000,
				Limit:            5000,
				TotalPercentUsed: 20,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected nil overage")
	}
}

func TestParseTypedResponse_OnDemandDisabled(t *testing.T) {
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
			OnDemand: &OnDemandUsage{
				Enabled:   boolPtr(false),
				Used:      0,
				Limit:     nil,
				Remaining: nil,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected nil overage when on-demand disabled")
	}
}

func TestParseTypedResponse_OnDemandZeroLimit(t *testing.T) {
	zero := float64(0)
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
			OnDemand: &OnDemandUsage{
				Enabled:   boolPtr(true),
				Used:      0,
				Limit:     &zero,
				Remaining: &zero,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected nil overage when on-demand limit is 0")
	}
}

func TestParseTypedResponse_NoUserData(t *testing.T) {
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity")
	}
}

func TestParseTypedResponse_MembershipTypeFromUsageSummary(t *testing.T) {
	usage := UsageSummaryResponse{
		MembershipType: "pro",
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity == nil {
		t.Fatal("expected identity from membershipType")
	}
	if snapshot.Identity.Plan != "pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "pro")
	}
}

func TestParseTypedResponse_UserEmailOverridesMembershipType(t *testing.T) {
	usage := UsageSummaryResponse{
		MembershipType: "pro",
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
		},
	}
	user := &UserMeResponse{Email: "user@example.com", MembershipType: "enterprise"}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, user)

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", snapshot.Identity.Email, "user@example.com")
	}
	// User response membership_type takes priority over usage summary
	if snapshot.Identity.Plan != "enterprise" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "enterprise")
	}
}

func TestParseTypedResponse_FallbackPercentFromLimit(t *testing.T) {
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled:          boolPtr(true),
				Used:             2500,
				Limit:            5000,
				TotalPercentUsed: 0, // API returns 0, fallback to calculated
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].Utilization != 50 {
		t.Errorf("utilization = %d, want 50", snapshot.Periods[0].Utilization)
	}
}

func TestParseTypedResponse_ZeroUsage(t *testing.T) {
	usage := UsageSummaryResponse{
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    0,
				Limit:   5000,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0", snapshot.Periods[0].Utilization)
	}
}

func TestParseTypedResponse_BillingCycleISO8601(t *testing.T) {
	usage := UsageSummaryResponse{
		BillingCycleEnd: "2026-03-14T21:47:25.853Z",
		IndividualUsage: &IndividualUsage{
			Plan: &PlanUsage{
				Enabled: boolPtr(true),
				Used:    1000,
				Limit:   5000,
			},
		},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].ResetsAt == nil {
		t.Fatal("expected resets_at for ISO 8601 billing cycle end")
	}
}
