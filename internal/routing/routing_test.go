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

	candidates, unavailable := Rank(providerIDs, snapshots, nil)

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

	candidates, _ := Rank(providerIDs, snapshots, nil)

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

	candidates, unavailable := Rank(providerIDs, snapshots, nil)

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

	candidates, unavailable := Rank(providerIDs, snapshots, nil)

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

	candidates, unavailable := Rank(providerIDs, snapshots, nil)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 1 {
		t.Fatalf("expected 1 unavailable, got %d", len(unavailable))
	}
}

func TestRank_UsesBottleneckPeriod(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour)
	weeklyReset := time.Now().Add(72 * time.Hour)

	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude": {
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: time.Now().UTC(),
				Periods: []models.UsagePeriod{
					{Name: "Session", Utilization: 2, PeriodType: models.PeriodSession, ResetsAt: &reset},
					{Name: "Weekly", Utilization: 62, PeriodType: models.PeriodWeekly, ResetsAt: &weeklyReset},
				},
				Identity: &models.ProviderIdentity{Plan: ""},
			},
		},
		"copilot": {Snapshot: makeSnapshot("copilot", 10, models.PeriodMonthly, "individual")},
	}

	candidates, _ := Rank(providerIDs, snapshots, nil)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Copilot (90% headroom) should beat Claude (38% headroom from weekly bottleneck)
	if candidates[0].ProviderID != "copilot" {
		t.Errorf("first = %q, want copilot (more headroom)", candidates[0].ProviderID)
	}
	if candidates[0].Headroom != 90 {
		t.Errorf("copilot headroom = %d, want 90", candidates[0].Headroom)
	}

	// Claude's headroom should reflect the weekly bottleneck (38%), not session (98%)
	if candidates[1].ProviderID != "claude" {
		t.Errorf("second = %q, want claude", candidates[1].ProviderID)
	}
	if candidates[1].Headroom != 38 {
		t.Errorf("claude headroom = %d, want 38 (weekly bottleneck)", candidates[1].Headroom)
	}
	if candidates[1].PeriodType != models.PeriodWeekly {
		t.Errorf("claude period = %q, want weekly (bottleneck)", candidates[1].PeriodType)
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

	candidates, _ := Rank(providerIDs, snapshots, nil)

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

	candidates, _ := Rank(providerIDs, snapshots, nil)

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

	candidates, unavailable := Rank(providerIDs, snapshots, nil)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 2 {
		t.Errorf("expected 2 unavailable, got %d", len(unavailable))
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestRank_MultiplierFreeModelWins(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 20, models.PeriodSession, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 50, models.PeriodMonthly, "")},
	}
	// Copilot model is free (0x multiplier).
	multipliers := map[string]*float64{
		"copilot": floatPtr(0),
	}

	candidates, _ := Rank(providerIDs, snapshots, multipliers)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Copilot should win: 0x multiplier → effective headroom 100%
	if candidates[0].ProviderID != "copilot" {
		t.Errorf("first = %q, want copilot (free model)", candidates[0].ProviderID)
	}
	if candidates[0].EffectiveHeadroom != 100 {
		t.Errorf("copilot effective headroom = %d, want 100 (free)", candidates[0].EffectiveHeadroom)
	}
}

func TestRank_ExpensiveMultiplierReducesEffectiveHeadroom(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 50, models.PeriodWeekly, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 10, models.PeriodMonthly, "")},
	}
	// Copilot model costs 3x premium requests.
	multipliers := map[string]*float64{
		"copilot": floatPtr(3),
	}

	candidates, _ := Rank(providerIDs, snapshots, multipliers)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Claude: 50% headroom, no multiplier → effective 50
	// Copilot: 90% headroom, 3x multiplier → effective 30
	if candidates[0].ProviderID != "claude" {
		t.Errorf("first = %q, want claude (better effective headroom)", candidates[0].ProviderID)
	}
	if candidates[0].EffectiveHeadroom != 50 {
		t.Errorf("claude effective headroom = %d, want 50", candidates[0].EffectiveHeadroom)
	}
	if candidates[1].ProviderID != "copilot" {
		t.Errorf("second = %q, want copilot", candidates[1].ProviderID)
	}
	if candidates[1].EffectiveHeadroom != 30 {
		t.Errorf("copilot effective headroom = %d, want 30 (90/3)", candidates[1].EffectiveHeadroom)
	}
	if candidates[1].Multiplier == nil || *candidates[1].Multiplier != 3 {
		t.Errorf("copilot multiplier = %v, want 3", candidates[1].Multiplier)
	}
}

func TestRank_CheapMultiplierBoostsEffectiveHeadroom(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 60, models.PeriodWeekly, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 70, models.PeriodMonthly, "")},
	}
	// Copilot model costs 0.33x (cheap).
	multipliers := map[string]*float64{
		"copilot": floatPtr(0.33),
	}

	candidates, _ := Rank(providerIDs, snapshots, multipliers)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Copilot: 30% headroom, 0.33x → effective 90 (30/0.33 ≈ 90.9, capped at 100)
	// Claude: 40% headroom, no multiplier → effective 40
	if candidates[0].ProviderID != "copilot" {
		t.Errorf("first = %q, want copilot (cheap model boosts headroom)", candidates[0].ProviderID)
	}
	if candidates[0].EffectiveHeadroom != 90 {
		t.Errorf("copilot effective headroom = %d, want 90", candidates[0].EffectiveHeadroom)
	}
}

func TestRank_NilMultipliersEqualsNoMultipliers(t *testing.T) {
	providerIDs := []string{"claude"}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 30, models.PeriodSession, "")},
	}

	candidates, _ := Rank(providerIDs, snapshots, nil)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].EffectiveHeadroom != 70 {
		t.Errorf("effective headroom = %d, want 70 (same as raw headroom)", candidates[0].EffectiveHeadroom)
	}
	if candidates[0].Multiplier != nil {
		t.Errorf("multiplier = %v, want nil", candidates[0].Multiplier)
	}
}

func TestComputeEffectiveHeadroom(t *testing.T) {
	tests := []struct {
		name       string
		headroom   int
		multiplier *float64
		want       int
	}{
		{"nil multiplier", 80, nil, 80},
		{"zero multiplier (free)", 50, floatPtr(0), 100},
		{"1x multiplier", 90, floatPtr(1), 90},
		{"3x multiplier", 90, floatPtr(3), 30},
		{"0.33x multiplier", 30, floatPtr(0.33), 90},
		{"0.33x capped at 100", 50, floatPtr(0.33), 100},
		{"zero headroom", 0, floatPtr(3), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEffectiveHeadroom(tt.headroom, tt.multiplier)
			if got != tt.want {
				t.Errorf("computeEffectiveHeadroom(%d, %v) = %d, want %d", tt.headroom, tt.multiplier, got, tt.want)
			}
		})
	}
}

// RankByRole tests

func TestRankByRole_RanksAcrossModelsAndProviders(t *testing.T) {
	entries := []RoleModelEntry{
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", ProviderIDs: []string{"claude"}},
		{ModelID: "o4", ModelName: "o4", ProviderIDs: []string{"codex"}},
	}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 80, models.PeriodSession, "Max")},
		"codex":  {Snapshot: makeSnapshot("codex", 30, models.PeriodMonthly, "Plus")},
	}

	candidates, unavailable := RankByRole(entries, snapshots, nil)

	if len(unavailable) != 0 {
		t.Errorf("expected 0 unavailable, got %d", len(unavailable))
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// codex (70 headroom) > claude (20 headroom)
	if candidates[0].ProviderID != "codex" {
		t.Errorf("first = %q, want codex", candidates[0].ProviderID)
	}
	if candidates[0].ModelID != "o4" {
		t.Errorf("first model = %q, want o4", candidates[0].ModelID)
	}
	if candidates[0].ModelName != "o4" {
		t.Errorf("first model name = %q, want o4", candidates[0].ModelName)
	}
	if candidates[1].ProviderID != "claude" {
		t.Errorf("second = %q, want claude", candidates[1].ProviderID)
	}
	if candidates[1].ModelID != "claude-opus-4-6" {
		t.Errorf("second model = %q, want claude-opus-4-6", candidates[1].ModelID)
	}
}

func TestRankByRole_DeduplicatesProviders(t *testing.T) {
	// Two models both available on the same provider — should only appear once.
	entries := []RoleModelEntry{
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", ProviderIDs: []string{"claude"}},
		{ModelID: "claude-sonnet-4-6", ModelName: "Claude Sonnet 4.6", ProviderIDs: []string{"claude"}},
	}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 40, models.PeriodSession, "")},
	}

	candidates, _ := RankByRole(entries, snapshots, nil)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (deduplicated), got %d", len(candidates))
	}
	// First model in the list wins for the provider.
	if candidates[0].ModelID != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6 (first listed)", candidates[0].ModelID)
	}
}

func TestRankByRole_MissingProviderIsUnavailable(t *testing.T) {
	entries := []RoleModelEntry{
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", ProviderIDs: []string{"claude"}},
		{ModelID: "o4", ModelName: "o4", ProviderIDs: []string{"codex"}},
	}
	snapshots := map[string]ProviderData{
		"claude": {Snapshot: makeSnapshot("claude", 30, models.PeriodSession, "")},
		// codex missing
	}

	candidates, unavailable := RankByRole(entries, snapshots, nil)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if len(unavailable) != 1 {
		t.Fatalf("expected 1 unavailable, got %d", len(unavailable))
	}
	if unavailable[0].ModelID != "o4" {
		t.Errorf("unavailable model = %q, want o4", unavailable[0].ModelID)
	}
	if unavailable[0].ProviderID != "codex" {
		t.Errorf("unavailable provider = %q, want codex", unavailable[0].ProviderID)
	}
}

func TestRankByRole_WithMultipliers(t *testing.T) {
	entries := []RoleModelEntry{
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", ProviderIDs: []string{"claude"}},
		{ModelID: "claude-opus-4-6", ModelName: "Claude Opus 4.6", ProviderIDs: []string{"copilot"}},
	}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 50, models.PeriodSession, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 10, models.PeriodMonthly, "")},
	}

	multiplierFn := func(modelName string, providerID string) *float64 {
		if providerID == "copilot" {
			v := 3.0
			return &v
		}
		return nil
	}

	candidates, _ := RankByRole(entries, snapshots, multiplierFn)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Claude: 50 headroom, no multiplier → effective 50
	// Copilot: 90 headroom, 3x → effective 30
	if candidates[0].ProviderID != "claude" {
		t.Errorf("first = %q, want claude (better effective headroom)", candidates[0].ProviderID)
	}
	if candidates[1].EffectiveHeadroom != 30 {
		t.Errorf("copilot effective headroom = %d, want 30", candidates[1].EffectiveHeadroom)
	}
}

func TestRankByRole_AllMissing(t *testing.T) {
	entries := []RoleModelEntry{
		{ModelID: "o4", ModelName: "o4", ProviderIDs: []string{"codex"}},
	}
	snapshots := map[string]ProviderData{}

	candidates, unavailable := RankByRole(entries, snapshots, nil)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 1 {
		t.Errorf("expected 1 unavailable, got %d", len(unavailable))
	}
}

func TestRankByRole_EmptyEntries(t *testing.T) {
	candidates, unavailable := RankByRole(nil, nil, nil)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(unavailable) != 0 {
		t.Errorf("expected 0 unavailable, got %d", len(unavailable))
	}
}

func TestRank_FullyUsedProvider(t *testing.T) {
	providerIDs := []string{"claude", "copilot"}
	snapshots := map[string]ProviderData{
		"claude":  {Snapshot: makeSnapshot("claude", 100, models.PeriodSession, "")},
		"copilot": {Snapshot: makeSnapshot("copilot", 60, models.PeriodMonthly, "")},
	}

	candidates, _ := Rank(providerIDs, snapshots, nil)

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
