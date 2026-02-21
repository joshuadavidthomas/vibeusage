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

func TestParseModelsResponse_FullResponse(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-2.5-pro": {
				DisplayName: "Gemini 2.5 Pro",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.75),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.5),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &TierInfo{ID: "pro-tier", Name: "Google AI Pro"},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, codeAssist)

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

	// Periods are sorted by model name
	p0 := snapshot.Periods[0]
	if p0.Model != "gemini-2.5-pro" {
		t.Errorf("period[0] model = %q, want %q", p0.Model, "gemini-2.5-pro")
	}
	if p0.Utilization != 25 {
		t.Errorf("period[0] utilization = %d, want 25", p0.Utilization)
	}
	if p0.PeriodType != models.PeriodSession {
		t.Errorf("period[0] period_type = %q, want %q (paid tier)", p0.PeriodType, models.PeriodSession)
	}
	if p0.Name != "Gemini 2.5 Pro" {
		t.Errorf("period[0] name = %q, want %q", p0.Name, "Gemini 2.5 Pro")
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
	if snapshot.Identity.Plan != "Google AI Pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Google AI Pro")
	}
}

func TestParseModelsResponse_FreeTier(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.9),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &TierInfo{ID: "free-tier", Name: "Antigravity"},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (free tier)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseModelsResponse_SkipsModelsWithoutResetTime(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.5),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
			"tab_flash_lite_preview": {
				// No display name, no reset time — tab completion model
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(1.0),
				},
			},
			"chat_20706": {
				// No quota info at all
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1 (should skip models without reset time)", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Model != "gemini-3-flash" {
		t.Errorf("period model = %q, want %q", snapshot.Periods[0].Model, "gemini-3-flash")
	}
}

func TestParseModelsResponse_NoTier(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(1.0),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no code assist response")
	}

	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (no tier defaults to weekly)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseModelsResponse_EmptyModels(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot (fallback period)")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Name != "Usage" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Usage")
	}
}

func TestParseModelsResponse_SortedByModelName(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
			"claude-sonnet-4-6": {
				DisplayName: "Claude Sonnet 4.6",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
			"gemini-2.5-pro": {
				DisplayName: "Gemini 2.5 Pro",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if len(snapshot.Periods) != 3 {
		t.Fatalf("len(periods) = %d, want 3", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Model != "claude-sonnet-4-6" {
		t.Errorf("period[0] model = %q, want %q", snapshot.Periods[0].Model, "claude-sonnet-4-6")
	}
	if snapshot.Periods[1].Model != "gemini-2.5-pro" {
		t.Errorf("period[1] model = %q, want %q", snapshot.Periods[1].Model, "gemini-2.5-pro")
	}
	if snapshot.Periods[2].Model != "gemini-3-flash" {
		t.Errorf("period[2] model = %q, want %q", snapshot.Periods[2].Model, "gemini-3-flash")
	}
}

func TestPeriodTypeForTier(t *testing.T) {
	tests := []struct {
		tier string
		want models.PeriodType
	}{
		{"free", models.PeriodWeekly},
		{"", models.PeriodWeekly},
		{"Antigravity", models.PeriodWeekly},
		{"antigravity", models.PeriodWeekly},
		{"free-tier", models.PeriodWeekly},
		{"premium", models.PeriodSession},
		{"Google AI Pro", models.PeriodSession},
		{"Google AI Ultra", models.PeriodSession},
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
