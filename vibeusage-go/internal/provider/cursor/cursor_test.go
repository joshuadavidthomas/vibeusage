package cursor

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseTypedResponse_FullResponse(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 42.0, Available: 58.0},
		BillingCycle:    &BillingCycle{EndRaw: []byte(`"2025-03-01T00:00:00Z"`)},
		OnDemandSpend:   &OnDemandSpend{LimitCents: 5000, UsedCents: 1500},
	}
	user := &UserMeResponse{Email: "user@example.com", MembershipType: "pro"}

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
	if p.Name != "Premium Requests" {
		t.Errorf("name = %q, want %q", p.Name, "Premium Requests")
	}
	if p.Utilization != 42 {
		t.Errorf("utilization = %d, want 42", p.Utilization)
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
	if snapshot.Overage.Limit != 50.0 {
		t.Errorf("overage limit = %v, want 50.0", snapshot.Overage.Limit)
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

func TestParseTypedResponse_NoPremiumRequests(t *testing.T) {
	usage := UsageSummaryResponse{}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot != nil {
		t.Error("expected nil snapshot when no premium requests")
	}
}

func TestParseTypedResponse_NoOnDemandSpend(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 10.0, Available: 90.0},
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

func TestParseTypedResponse_ZeroOnDemandLimit(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 10.0, Available: 90.0},
		OnDemandSpend:   &OnDemandSpend{LimitCents: 0, UsedCents: 0},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected nil overage when limit is 0")
	}
}

func TestParseTypedResponse_NoUserData(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 10.0, Available: 90.0},
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

func TestParseTypedResponse_BillingCycleNumeric(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 10.0, Available: 90.0},
		BillingCycle:    &BillingCycle{EndRaw: []byte(`1740787200000`)},
	}

	s := WebStrategy{}
	snapshot := s.parseTypedResponse(usage, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Periods[0].ResetsAt == nil {
		t.Fatal("expected resets_at for numeric billing cycle end")
	}
}

func TestParseTypedResponse_ZeroUsage(t *testing.T) {
	usage := UsageSummaryResponse{
		PremiumRequests: &PremiumRequests{Used: 0, Available: 100.0},
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
