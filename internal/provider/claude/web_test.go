package claude

import (
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseWebUsageResponse_FullResponse(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount:  42.5,
		UsageLimit:   100.0,
		PeriodEnd:    "2025-02-19T22:00:00Z",
		Email:        "user@example.com",
		Organization: "My Org",
		Plan:         "pro",
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "claude" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "claude")
	}
	if snapshot.Source != "web" {
		t.Errorf("source = %q, want %q", snapshot.Source, "web")
	}

	// Should have 1 period
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}

	p := snapshot.Periods[0]
	if p.Name != "Usage" {
		t.Errorf("period name = %q, want %q", p.Name, "Usage")
	}
	// 42.5/100.0 = 42.5% -> int(42)
	if p.Utilization != 42 {
		t.Errorf("utilization = %d, want 42", p.Utilization)
	}
	if p.PeriodType != models.PeriodDaily {
		t.Errorf("period_type = %q, want %q", p.PeriodType, models.PeriodDaily)
	}
	if p.ResetsAt == nil {
		t.Fatal("expected resets_at to be set")
	}
	expected := time.Date(2025, 2, 19, 22, 0, 0, 0, time.UTC)
	if !p.ResetsAt.Equal(expected) {
		t.Errorf("resets_at = %v, want %v", p.ResetsAt, expected)
	}

	// Identity
	if snapshot.Identity == nil {
		t.Fatal("expected identity to be set")
	}
	if snapshot.Identity.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", snapshot.Identity.Email, "user@example.com")
	}
	if snapshot.Identity.Organization != "My Org" {
		t.Errorf("organization = %q, want %q", snapshot.Identity.Organization, "My Org")
	}
	if snapshot.Identity.Plan != "pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "pro")
	}

	// No overage
	if snapshot.Overage != nil {
		t.Error("expected overage to be nil")
	}
}

func TestParseWebUsageResponse_WithOverage(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 50.0,
		UsageLimit:  100.0,
	}
	overage := &models.OverageUsage{
		Used:      25.50,
		Limit:     100.00,
		Currency:  "USD",
		IsEnabled: true,
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, overage)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage == nil {
		t.Fatal("expected overage to be present")
	}
	if snapshot.Overage.Used != 25.50 {
		t.Errorf("overage used = %v, want 25.50", snapshot.Overage.Used)
	}
}

func TestParseWebUsageResponse_ZeroLimit(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 0,
		UsageLimit:  0,
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	// With zero limit, no period should be created
	if len(snapshot.Periods) != 0 {
		t.Errorf("len(periods) = %d, want 0", len(snapshot.Periods))
	}
}

func TestParseWebUsageResponse_NoIdentity(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 10.0,
		UsageLimit:  50.0,
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected identity to be nil when no email/org/plan")
	}
}

func TestParseWebUsageResponse_InvalidPeriodEnd(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 50.0,
		UsageLimit:  100.0,
		PeriodEnd:   "not-a-date",
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	// Period should exist but resets_at should be nil
	if snapshot.Periods[0].ResetsAt != nil {
		t.Error("expected resets_at to be nil for invalid date")
	}
}

func TestParseWebUsageResponse_EmptyPeriodEnd(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 50.0,
		UsageLimit:  100.0,
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].ResetsAt != nil {
		t.Error("expected resets_at to be nil when period_end is empty")
	}
}

func TestParseWebUsageResponse_HighUtilization(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 95.0,
		UsageLimit:  100.0,
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Utilization != 95 {
		t.Errorf("utilization = %d, want 95", snapshot.Periods[0].Utilization)
	}
}

func TestParseWebUsageResponse_PartialIdentity(t *testing.T) {
	resp := WebUsageResponse{
		UsageAmount: 50.0,
		UsageLimit:  100.0,
		Email:       "user@example.com",
	}

	s := WebStrategy{}
	snapshot := s.parseWebUsageResponse(resp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity == nil {
		t.Fatal("expected identity when email is present")
	}
	if snapshot.Identity.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", snapshot.Identity.Email, "user@example.com")
	}
	if snapshot.Identity.Organization != "" {
		t.Errorf("organization = %q, want empty", snapshot.Identity.Organization)
	}
}

func TestGetOrgFromList_ChatCapability(t *testing.T) {
	orgs := []WebOrganization{
		{UUID: "org-1", Capabilities: []string{"billing"}},
		{UUID: "org-2", Capabilities: []string{"chat", "billing"}},
		{UUID: "org-3", Capabilities: []string{"chat"}},
	}

	s := WebStrategy{}
	orgID := s.findChatOrgID(orgs)

	if orgID != "org-2" {
		t.Errorf("findChatOrgID() = %q, want %q (first org with chat)", orgID, "org-2")
	}
}

func TestGetOrgFromList_NoChatCapability(t *testing.T) {
	orgs := []WebOrganization{
		{UUID: "org-1", Capabilities: []string{"billing"}},
		{UUID: "org-2", Capabilities: []string{"billing"}},
	}

	s := WebStrategy{}
	orgID := s.findChatOrgID(orgs)

	// Fallback to first org
	if orgID != "org-1" {
		t.Errorf("findChatOrgID() = %q, want %q (fallback to first)", orgID, "org-1")
	}
}

func TestGetOrgFromList_Empty(t *testing.T) {
	s := WebStrategy{}
	orgID := s.findChatOrgID(nil)

	if orgID != "" {
		t.Errorf("findChatOrgID() = %q, want empty", orgID)
	}
}

func TestGetOrgFromList_UsesIDFallback(t *testing.T) {
	orgs := []WebOrganization{
		{ID: "org-legacy", Capabilities: []string{"chat"}},
	}

	s := WebStrategy{}
	orgID := s.findChatOrgID(orgs)

	if orgID != "org-legacy" {
		t.Errorf("findChatOrgID() = %q, want %q", orgID, "org-legacy")
	}
}
