package gemini

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func ptrFloat64(f float64) *float64 { return &f }

func TestParseTypedQuotaResponse_FullResponse(t *testing.T) {
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.0-flash", RemainingFraction: ptrFloat64(0.75), ResetTime: "2025-02-20T00:00:00Z", TokenType: "REQUESTS"},
			{ModelID: "gemini-1.5-pro", RemainingFraction: ptrFloat64(0.5), ResetTime: "2025-02-20T00:00:00Z", TokenType: "REQUESTS"},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &CodeAssistTier{
			ID:   "standard-tier",
			Name: "Gemini Code Assist",
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "gemini" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "gemini")
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
	if p0.PeriodType != models.PeriodDaily {
		t.Errorf("period[0] period_type = %q, want %q", p0.PeriodType, models.PeriodDaily)
	}
	if p0.Model != "gemini-2.0-flash" {
		t.Errorf("period[0] model = %q, want %q", p0.Model, "gemini-2.0-flash")
	}
	if p0.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}

	p1 := snapshot.Periods[1]
	if p1.Utilization != 50 {
		t.Errorf("period[1] utilization = %d, want 50", p1.Utilization)
	}
	if p1.Model != "gemini-1.5-pro" {
		t.Errorf("period[1] model = %q, want %q", p1.Model, "gemini-1.5-pro")
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Gemini Code Assist" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Gemini Code Assist")
	}
}

func TestParseTypedQuotaResponse_EmptyBuckets(t *testing.T) {
	quota := QuotaResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot (fallback daily period)")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Name != "Daily" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Daily")
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0", snapshot.Periods[0].Utilization)
	}
}

func TestParseTypedQuotaResponse_NoCodeAssist(t *testing.T) {
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no code assist response")
	}
}

func TestParseTypedQuotaResponse_EmptyCurrentTier(t *testing.T) {
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}
	codeAssist := &CodeAssistResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when currentTier is nil")
	}
}

func TestParseTypedQuotaResponse_EmptyTierName(t *testing.T) {
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &CodeAssistTier{ID: "standard-tier"},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when tier name is empty")
	}
}

func TestParseTypedQuotaResponse_ModelNameParsing(t *testing.T) {
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	p := snapshot.Periods[0]
	if p.Model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want %q", p.Model, "gemini-2.0-flash")
	}
	if p.Name == "" {
		t.Error("expected non-empty name")
	}
}

func TestParseTypedQuotaResponse_ModelNameWithPrefix(t *testing.T) {
	// The live API returns model IDs without the "models/" prefix,
	// but the code still handles it gracefully if present.
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "models/gemini-2.0-flash", RemainingFraction: ptrFloat64(1.0)},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	p := snapshot.Periods[0]
	if p.Model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want %q", p.Model, "gemini-2.0-flash")
	}
}

func TestParseTypedQuotaResponse_VertexSuffixModels(t *testing.T) {
	// The live API returns vertex variants with _vertex suffix
	quota := QuotaResponse{
		Buckets: []QuotaBucket{
			{ModelID: "gemini-2.5-flash", RemainingFraction: ptrFloat64(0.8), TokenType: "REQUESTS"},
			{ModelID: "gemini-2.5-flash_vertex", RemainingFraction: ptrFloat64(0.9), TokenType: "REQUESTS"},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseTypedQuotaResponse(quota, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2", len(snapshot.Periods))
	}

	if snapshot.Periods[0].Model != "gemini-2.5-flash" {
		t.Errorf("period[0] model = %q, want %q", snapshot.Periods[0].Model, "gemini-2.5-flash")
	}
	if snapshot.Periods[1].Model != "gemini-2.5-flash_vertex" {
		t.Errorf("period[1] model = %q, want %q", snapshot.Periods[1].Model, "gemini-2.5-flash_vertex")
	}
}
