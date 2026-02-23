package routing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// stubRenderBar is a test double for the bar renderer.
func stubRenderBar(utilization int) string {
	return fmt.Sprintf("[bar:%d%%]", utilization)
}

// stubFormatReset is a test double for reset countdown formatting.
func stubFormatReset(d *time.Duration) string {
	if d == nil {
		return ""
	}
	return "2h"
}

func TestFormatRecommendationRows_SingleModel(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour)
	rec := Recommendation{
		ModelID:   "claude-sonnet-4-6",
		ModelName: "Claude Sonnet 4.6",
		Best: &Candidate{
			ProviderID:        "claude",
			Headroom:          80,
			Utilization:       20,
			EffectiveHeadroom: 80,
			PeriodType:        models.PeriodWeekly,
			ResetsAt:          &reset,
			Plan:              "Pro",
		},
		Candidates: []Candidate{
			{
				ProviderID:        "claude",
				Headroom:          80,
				Utilization:       20,
				EffectiveHeadroom: 80,
				PeriodType:        models.PeriodWeekly,
				ResetsAt:          &reset,
				Plan:              "Pro",
			},
			{
				ProviderID:        "copilot",
				Headroom:          50,
				Utilization:       50,
				Multiplier:        floatPtr(3),
				EffectiveHeadroom: 16,
				PeriodType:        models.PeriodMonthly,
				ResetsAt:          &reset,
				Plan:              "Business",
			},
		},
		Unavailable: []string{"cursor"},
	}

	result := FormatRecommendationRows(rec, stubRenderBar, stubFormatReset)

	// Has multiplier → should include Cost column
	if !containsString(result.Headers, "Cost") {
		t.Errorf("headers = %v, want Cost column (has multiplier)", result.Headers)
	}

	// Should not include Model column (single-model mode)
	if containsString(result.Headers, "Model") {
		t.Errorf("headers = %v, should not have Model column", result.Headers)
	}

	// 2 candidates + 1 unavailable = 3 rows
	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}

	// First row should be the best candidate (claude)
	if !containsString(result.Rows[0], "Claude") {
		t.Errorf("first row = %v, want Claude", result.Rows[0])
	}

	// Last row should be the unavailable provider
	if !containsString(result.Rows[2], "Cursor") {
		t.Errorf("last row = %v, want Cursor (unavailable)", result.Rows[2])
	}

	// Row styles
	if len(result.Styles) != 3 {
		t.Fatalf("expected 3 styles, got %d", len(result.Styles))
	}
	if result.Styles[0] != RowBold {
		t.Errorf("first style = %v, want bold", result.Styles[0])
	}
	if result.Styles[1] != RowNormal {
		t.Errorf("second style = %v, want normal", result.Styles[1])
	}
	if result.Styles[2] != RowDim {
		t.Errorf("third style = %v, want dim", result.Styles[2])
	}
}

func TestFormatRecommendationRows_NoMultiplier(t *testing.T) {
	reset := time.Now().Add(1 * time.Hour)
	rec := Recommendation{
		ModelID:   "gpt-5",
		ModelName: "GPT-5",
		Best: &Candidate{
			ProviderID:        "codex",
			Headroom:          90,
			Utilization:       10,
			EffectiveHeadroom: 90,
			PeriodType:        models.PeriodMonthly,
			ResetsAt:          &reset,
			Plan:              "Plus",
		},
		Candidates: []Candidate{
			{
				ProviderID:        "codex",
				Headroom:          90,
				Utilization:       10,
				EffectiveHeadroom: 90,
				PeriodType:        models.PeriodMonthly,
				ResetsAt:          &reset,
				Plan:              "Plus",
			},
		},
	}

	result := FormatRecommendationRows(rec, stubRenderBar, stubFormatReset)

	// No multiplier → no Cost column
	if containsString(result.Headers, "Cost") {
		t.Errorf("headers = %v, should not have Cost column (no multipliers)", result.Headers)
	}
}

func TestFormatRoleRecommendationRows(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour)
	rec := RoleRecommendation{
		Role: "thinking",
		Best: &RoleCandidate{
			Candidate: Candidate{
				ProviderID:        "claude",
				Headroom:          80,
				Utilization:       20,
				EffectiveHeadroom: 80,
				PeriodType:        models.PeriodWeekly,
				ResetsAt:          &reset,
				Plan:              "Pro",
			},
			ModelID:   "claude-opus-4-6",
			ModelName: "Claude Opus 4.6",
		},
		Candidates: []RoleCandidate{
			{
				Candidate: Candidate{
					ProviderID:        "claude",
					Headroom:          80,
					Utilization:       20,
					EffectiveHeadroom: 80,
					PeriodType:        models.PeriodWeekly,
					ResetsAt:          &reset,
					Plan:              "Pro",
				},
				ModelID:   "claude-opus-4-6",
				ModelName: "Claude Opus 4.6",
			},
		},
		Unavailable: []RoleUnavailable{
			{ModelID: "o4", ProviderID: "codex"},
		},
	}

	result := FormatRoleRecommendationRows(rec, stubRenderBar, stubFormatReset)

	// Should include Model column
	if !containsString(result.Headers, "Model") {
		t.Errorf("headers = %v, want Model column", result.Headers)
	}

	// 1 candidate + 1 unavailable = 2 rows
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}

	// First row includes model ID
	if !containsString(result.Rows[0], "claude-opus-4-6") {
		t.Errorf("first row = %v, want to contain model ID", result.Rows[0])
	}
}

func TestFormatRecommendationRows_NoBest(t *testing.T) {
	rec := Recommendation{
		ModelID:     "gpt-5",
		ModelName:   "GPT-5",
		Unavailable: []string{"codex"},
	}

	result := FormatRecommendationRows(rec, stubRenderBar, stubFormatReset)

	// With no candidates, we get only unavailable rows
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row (unavailable), got %d", len(result.Rows))
	}
	if result.Styles[0] != RowDim {
		t.Errorf("style = %v, want dim", result.Styles[0])
	}
}

func TestFormatRecommendationRows_PlanFallback(t *testing.T) {
	reset := time.Now().Add(1 * time.Hour)
	rec := Recommendation{
		ModelID:   "gpt-5",
		ModelName: "GPT-5",
		Best: &Candidate{
			ProviderID:        "codex",
			Headroom:          90,
			Utilization:       10,
			EffectiveHeadroom: 90,
			PeriodType:        models.PeriodMonthly,
			ResetsAt:          &reset,
			Plan:              "",
		},
		Candidates: []Candidate{
			{
				ProviderID:        "codex",
				Headroom:          90,
				Utilization:       10,
				EffectiveHeadroom: 90,
				PeriodType:        models.PeriodMonthly,
				ResetsAt:          &reset,
				Plan:              "",
			},
		},
	}

	result := FormatRecommendationRows(rec, stubRenderBar, stubFormatReset)

	// Empty plan → "—"
	lastCol := result.Rows[0][len(result.Rows[0])-1]
	if lastCol != "—" {
		t.Errorf("plan column = %q, want %q", lastCol, "—")
	}
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}
