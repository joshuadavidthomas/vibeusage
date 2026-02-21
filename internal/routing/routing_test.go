package routing

import (
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func makeSnapshot(providerID string, utilization int, periodType models.PeriodType, plan string) *models.UsageSnapshot {
	reset := time.Now().Add(2 * time.Hour)
	return &models.UsageSnapshot{
		Provider:  providerID,
		FetchedAt: time.Now().UTC(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Usage",
				Utilization: utilization,
				PeriodType:  periodType,
				ResetsAt:    &reset,
			},
		},
		Identity: &models.ProviderIdentity{Plan: plan},
	}
}

func TestRank_SortsByHeadroom(t *testing.T) {
	providerIDs := []string{"claude", "copilot", "cursor"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 80, models.PeriodSession, "Pro")},
		"copilot": {Snapshot: makeSnapshot("copilot", 20, models.PeriodMonthly, "Business")},
		"cursor":  {Snapshot: makeSnapshot("cursor", 50, models.PeriodMonthly, "Pro")},
	}

	candidates, unavailable := Rank(providerIDs, snapshots)

	if len(unavailable) != 0 {
		t.Errorf("expected 0 unavailable, got %d", len(unavailable))
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// copilot (80 headroom) > cursor (50) > claude (20)
	if candidates[0].ProviderID != "copilot" {
		t.Errorf("first = %q, want copilot", candidates[0].ProviderID)
	}
	if candidates[0].Headroom != 80 {
		t.Errorf("first headroom = %d, want 80", candidates[0].Headroom)
	}
	if candidates[1].ProviderID != "cursor" {
		t.Errorf("second = %q, want cursor", candidates[1].ProviderID)
	}
	if candidates[2].ProviderID != "claude" {
		t.Errorf("third = %q, want claude", candidates[2].ProviderID)
	}
}

func TestRank_TiebreaksByProviderID(t *testing.T) {
	providerIDs := []string{"cursor", "claude"}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 50, models.PeriodSession, "")},
		"cursor": {Snapshot: makeSnapshot("cursor", 50, models.PeriodMonthly, "")},
	}

	candidates, _ := Rank(providerIDs, snapshots)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	// Same headroom → alphabetical: claude before cursor
	if candidates[0].ProviderID != "claude" {
		t.Errorf("first = %q, want claude (alphabetical tiebreak)", candidates[0].ProviderID)
	}
}

func TestRank_MissingProviders(t *testing.T) {
	providerIDs := []string{"claude", "copilot", "cursor"}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 30, models.PeriodSession, "")},
		// copilot and cursor missing
	}

	candidates, unavailable := Rank(providerIDs, snapshots)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].ProviderID != "claude" {
		t.Errorf("candidate = %q, want claude", candidates[0].ProviderID)
	}
	if len(unavailable) != 2 {
		t.Fatalf("expected 2 unavailable, got %d", len(unavailable))
	}
	// Sorted alphabetically
	if unavailable[0] != "copilot" || unavailable[1] != "cursor" {
		t.Errorf("unavailable = %v, want [copilot cursor]", unavailable)
	}
}

func TestRank_NilSnapshot(t *testing.T) {
	providerIDs := []string{"claude"}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: nil},
	}

	candidates, unavailable := Rank(providerIDs, snapshots)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 1 {
		t.Fatalf("expected 1 unavailable, got %d", len(unavailable))
	}
}

func TestRank_EmptyPeriods(t *testing.T) {
	providerIDs := []string{"claude"}
	snapshots := map[string]ProviderData{
		"claude": {
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: time.Now().UTC(),
				Periods:   []models.UsagePeriod{},
			},
		},
	}

	candidates, unavailable := Rank(providerIDs, snapshots)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 1 {
		t.Fatalf("expected 1 unavailable, got %d", len(unavailable))
	}
}

func TestRank_CachedFlag(t *testing.T) {
	providerIDs := []string{"claude"}
	snapshots := map[string]ProviderData{
		"claude": {
			Snapshot: makeSnapshot("claude", 40, models.PeriodSession, ""),
			Cached:   true,
		},
	}

	candidates, _ := Rank(providerIDs, snapshots)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if !candidates[0].Cached {
		t.Error("expected cached = true")
	}
}

func TestRank_PlanPropagated(t *testing.T) {
	providerIDs := []string{"claude"}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 10, models.PeriodSession, "Max")},
	}

	candidates, _ := Rank(providerIDs, snapshots)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Plan != "Max" {
		t.Errorf("plan = %q, want %q", candidates[0].Plan, "Max")
	}
}

func TestRank_AllProvidersMissing(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{}

	candidates, unavailable := Rank(providerIDs, snapshots)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 2 {
		t.Errorf("expected 2 unavailable, got %d", len(unavailable))
	}
}

func TestRank_FullyUsedProvider(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 100, models.PeriodSession, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 60, models.PeriodMonthly, "")},
	}

	candidates, _ := Rank(providerIDs, snapshots)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	// copilot has more headroom
	if candidates[0].ProviderID != "copilot" {
		t.Errorf("first = %q, want copilot", candidates[0].ProviderID)
	}
	// claude is fully used but still a candidate (headroom 0)
	if candidates[1].ProviderID != "claude" {
		t.Errorf("second = %q, want claude", candidates[1].ProviderID)
	}
	if candidates[1].Headroom != 0 {
		t.Errorf("claude headroom = %d, want 0", candidates[1].Headroom)
	}
}
