package antigravity

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestMeta(t *testing.T) {
	a := Antigravity{}
	meta := a.Meta()

	if meta.ID != "antigravity" {
		t.Errorf("ID = %q, want %q", meta.ID, "antigravity")
	}
	if meta.Name != "Antigravity" {
		t.Errorf("Name = %q, want %q", meta.Name, "Antigravity")
	}
}

func TestFetchStrategies(t *testing.T) {
	a := Antigravity{}
	strategies := a.FetchStrategies()

	if len(strategies) != 1 {
		t.Fatalf("len(strategies) = %d, want 1", len(strategies))
	}
	if strategies[0].Name() != "oauth" {
		t.Errorf("strategy name = %q, want %q", strategies[0].Name(), "oauth")
	}
}

func TestParseQuotaResponse_FullResponse(t *testing.T) {
	quota := QuotaResponse{
		QuotaBuckets: []QuotaBucket{
			{ModelID: "models/gemini-2.5-pro", RemainingFraction: ptrFloat64(0.75), ResetTime: "2026-02-20T05:00:00Z"},
			{ModelID: "models/gemini-2.0-flash", RemainingFraction: ptrFloat64(0.5), ResetTime: "2026-02-20T05:00:00Z"},
		},
	}
	codeAssist := &CodeAssistResponse{UserTier: "premium"}

	s := OAuthStrategy{}
	snapshot := s.parseQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "antigravity" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "antigravity")
	}
	if snapshot.Source != "oauth" {
		t.Errorf("source = %q, want %q", snapshot.Source, "oauth")
	}

	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2", len(snapshot.Periods))
	}

	p0 := snapshot.Periods[0]
	if p0.Utilization != 25 {
		t.Errorf("period[0] utilization = %d, want 25", p0.Utilization)
	}
	if p0.PeriodType != models.PeriodSession {
		t.Errorf("period[0] period_type = %q, want %q (premium tier)", p0.PeriodType, models.PeriodSession)
	}
	if p0.Model != "gemini-2.5-pro" {
		t.Errorf("period[0] model = %q, want %q", p0.Model, "gemini-2.5-pro")
	}
	if p0.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}

	p1 := snapshot.Periods[1]
	if p1.Utilization != 50 {
		t.Errorf("period[1] utilization = %d, want 50", p1.Utilization)
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "premium" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "premium")
	}
}

func TestParseQuotaResponse_FreeTier(t *testing.T) {
	quota := QuotaResponse{
		QuotaBuckets: []QuotaBucket{
			{ModelID: "models/gemini-2.0-flash", RemainingFraction: ptrFloat64(0.9)},
		},
	}
	codeAssist := &CodeAssistResponse{UserTier: "free"}

	s := OAuthStrategy{}
	snapshot := s.parseQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (free tier)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseQuotaResponse_NoTier(t *testing.T) {
	quota := QuotaResponse{
		QuotaBuckets: []QuotaBucket{
			{ModelID: "models/gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no code assist response")
	}

	// No tier info defaults to weekly (free tier assumption)
	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (no tier defaults to weekly)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseQuotaResponse_EmptyBuckets(t *testing.T) {
	quota := QuotaResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot (fallback period)")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Name != "Usage" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Usage")
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0", snapshot.Periods[0].Utilization)
	}
}

func TestParseQuotaResponse_EmptyUserTier(t *testing.T) {
	quota := QuotaResponse{
		QuotaBuckets: []QuotaBucket{
			{ModelID: "models/gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}
	codeAssist := &CodeAssistResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when user_tier is empty")
	}
}

func TestPeriodTypeForTier(t *testing.T) {
	tests := []struct {
		tier string
		want models.PeriodType
	}{
		{"free", models.PeriodWeekly},
		{"", models.PeriodWeekly},
		{"premium", models.PeriodSession},
		{"pro", models.PeriodSession},
		{"ultra", models.PeriodSession},
		{"FREE", models.PeriodWeekly},
		{"Premium", models.PeriodSession},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := periodTypeForTier(tt.tier)
			if got != tt.want {
				t.Errorf("periodTypeForTier(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}
