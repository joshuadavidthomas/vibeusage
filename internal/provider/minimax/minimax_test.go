package minimax

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
	t.Setenv("MINIMAX_API_KEY", "sk-cp-test")
	s := &APIKeyStrategy{}
	if !s.IsAvailable() {
		t.Error("IsAvailable() = false, want true when MINIMAX_API_KEY is set")
	}
}

func TestAPIKeyStrategy_IsAvailable_NoEnvVar(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "")
	s := &APIKeyStrategy{}
	if s.loadToken() == "" && s.IsAvailable() {
		t.Error("IsAvailable() = true, want false when no token available")
	}
}

func TestAPIKeyStrategy_Fetch_NoToken(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "")
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

func TestParseResponse_Source(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{
			{
				EndTime:                   1771668000000,
				CurrentIntervalTotalCount: 500,
				CurrentIntervalUsageCount: 100,
				ModelName:                 "MiniMax-M2.5",
			},
		},
		BaseResp: BaseResp{StatusCode: 0},
	}

	snapshot := parseResponse(resp)
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Source != "api_key" {
		t.Errorf("Source = %q, want %q", snapshot.Source, "api_key")
	}
	if snapshot.Provider != "minimax" {
		t.Errorf("Provider = %q, want %q", snapshot.Provider, "minimax")
	}
}

func TestParseResponse_PeriodFields(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{
			{
				EndTime:                   1771668000000,
				CurrentIntervalTotalCount: 1500,
				CurrentIntervalUsageCount: 750,
				ModelName:                 "MiniMax-M2",
			},
		},
		BaseResp: BaseResp{StatusCode: 0},
	}

	snapshot := parseResponse(resp)
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(snapshot.Periods))
	}

	p := snapshot.Periods[0]
	if p.Name != "Coding Plan" {
		t.Errorf("Name = %q, want %q", p.Name, "Coding Plan")
	}
	if p.Utilization != 50 {
		t.Errorf("Utilization = %d, want 50", p.Utilization)
	}
	if p.PeriodType != models.PeriodSession {
		t.Errorf("PeriodType = %q, want %q", p.PeriodType, models.PeriodSession)
	}
}

func TestMinimax_Meta(t *testing.T) {
	m := Minimax{}
	meta := m.Meta()
	if meta.ID != "minimax" {
		t.Errorf("ID = %q, want %q", meta.ID, "minimax")
	}
	if meta.Name != "Minimax" {
		t.Errorf("Name = %q, want %q", meta.Name, "Minimax")
	}
}
