package claude

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseUsageResponse_WebSource(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour:       &UsagePeriodResponse{Utilization: 25.0, ResetsAt: "2025-02-19T22:00:00Z"},
		SevenDay:       &UsagePeriodResponse{Utilization: 10.0, ResetsAt: "2025-02-26T00:00:00Z"},
		SevenDaySonnet: &UsagePeriodResponse{Utilization: 15.0, ResetsAt: "2025-02-26T00:00:00Z"},
	}

	snapshot := parseUsageResponse(resp, "web", nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "claude" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "claude")
	}
	if snapshot.Source != "web" {
		t.Errorf("source = %q, want %q", snapshot.Source, "web")
	}

	// Should have 3 periods: Session (5h), All Models, Sonnet
	if len(snapshot.Periods) != 3 {
		t.Fatalf("len(periods) = %d, want 3", len(snapshot.Periods))
	}

	p := snapshot.Periods[0]
	if p.Name != "Session (5h)" {
		t.Errorf("period[0] name = %q, want %q", p.Name, "Session (5h)")
	}
	if p.Utilization != 25 {
		t.Errorf("period[0] utilization = %d, want 25", p.Utilization)
	}
	if p.PeriodType != models.PeriodSession {
		t.Errorf("period[0] type = %q, want %q", p.PeriodType, models.PeriodSession)
	}

	p = snapshot.Periods[2]
	if p.Name != "Sonnet" {
		t.Errorf("period[2] name = %q, want %q", p.Name, "Sonnet")
	}
	if p.Model != "sonnet" {
		t.Errorf("period[2] model = %q, want %q", p.Model, "sonnet")
	}
}

func TestParseUsageResponse_WebWithOverageOverride(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{Utilization: 50.0},
	}
	overage := &models.OverageUsage{
		Used:      25.50,
		Limit:     100.00,
		Currency:  "USD",
		IsEnabled: true,
	}

	snapshot := parseUsageResponse(resp, "web", overage)

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

func TestParseUsageResponse_InlineExtraUsageTakesPrecedence(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{Utilization: 50.0},
		ExtraUsage: &ExtraUsageResponse{
			IsEnabled:    true,
			UsedCredits:  1000,
			MonthlyLimit: 5000,
		},
	}
	overrideOverage := &models.OverageUsage{
		Used:      99.99,
		Limit:     200.00,
		Currency:  "USD",
		IsEnabled: true,
	}

	snapshot := parseUsageResponse(resp, "web", overrideOverage)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage == nil {
		t.Fatal("expected overage to be present")
	}
	// Inline extra_usage should win over the override
	if snapshot.Overage.Used != 10.0 { // 1000 / 100.0
		t.Errorf("overage used = %v, want 10.0 (from inline extra_usage)", snapshot.Overage.Used)
	}
}

func TestParseUsageResponse_EmptyResponse(t *testing.T) {
	resp := OAuthUsageResponse{}

	snapshot := parseUsageResponse(resp, "web", nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 0 {
		t.Errorf("len(periods) = %d, want 0", len(snapshot.Periods))
	}
	if snapshot.Identity != nil {
		t.Error("expected identity to be nil")
	}
	if snapshot.Overage != nil {
		t.Error("expected overage to be nil")
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
