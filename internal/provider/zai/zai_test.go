package zai

import (
	"context"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestAPIKeyStrategy_Name(t *testing.T) {
	s := &APIKeyStrategy{}
	if s.Name() != "api_key" {
		t.Errorf("Name() = %q, want %q", s.Name(), "api_key")
	}
}

func TestAPIKeyStrategy_IsAvailable_EnvVar(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "test-key")
	s := &APIKeyStrategy{}
	if !s.IsAvailable() {
		t.Error("IsAvailable() = false, want true when ZAI_API_KEY is set")
	}
}

func TestAPIKeyStrategy_IsAvailable_NoEnvVar(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	s := &APIKeyStrategy{}
	// Without the env var and no credential files, should be false.
	// (credential file absence is guaranteed by the empty env var path)
	got := s.IsAvailable()
	// We only assert false when loadToken returns ""
	if s.loadToken() == "" && got {
		t.Error("IsAvailable() = true, want false when no token available")
	}
}

func TestAPIKeyStrategy_Fetch_NoToken(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	s := &APIKeyStrategy{}
	if s.loadToken() != "" {
		t.Skip("credential file present — skipping no-token test")
	}

	result, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected non-successful result when no token available")
	}
}

func TestParseQuotaResponse_Source(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Success: true,
		Data: &QuotaData{
			Level: "pro",
			Limits: []QuotaLimit{
				{Type: "TOKENS_LIMIT", Unit: 3, Number: 5, Percentage: 10, NextResetTime: 1771661559241},
			},
		},
	}

	snapshot := parseQuotaResponse(resp)
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Source != "api_key" {
		t.Errorf("Source = %q, want %q", snapshot.Source, "api_key")
	}
	if snapshot.Provider != "zai" {
		t.Errorf("Provider = %q, want %q", snapshot.Provider, "zai")
	}
}

func TestParseQuotaResponse_PeriodFields(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Success: true,
		Data: &QuotaData{
			Limits: []QuotaLimit{
				{Type: "TOKENS_LIMIT", Unit: 3, Number: 5, Percentage: 75, NextResetTime: 1771661559241},
			},
		},
	}

	snapshot := parseQuotaResponse(resp)
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(snapshot.Periods))
	}

	p := snapshot.Periods[0]
	if p.Name != "Token Quota" {
		t.Errorf("Name = %q, want %q", p.Name, "Token Quota")
	}
	if p.Utilization != 75 {
		t.Errorf("Utilization = %d, want 75", p.Utilization)
	}
	if p.PeriodType != models.PeriodSession {
		t.Errorf("PeriodType = %q, want %q", p.PeriodType, models.PeriodSession)
	}
}

func TestZai_Meta(t *testing.T) {
	z := Zai{}
	meta := z.Meta()
	if meta.ID != "zai" {
		t.Errorf("ID = %q, want %q", meta.ID, "zai")
	}
	if meta.Name != "Z.ai" {
		t.Errorf("Name = %q, want %q", meta.Name, "Z.ai")
	}
}
