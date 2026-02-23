package routing

import (
	"context"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestBuildProviderData(t *testing.T) {
	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot:   makeSnapshot("claude", 30, models.PeriodWeekly, "Pro"),
			Cached:     false,
		},
		"copilot": {
			ProviderID: "copilot",
			Success:    false,
			Error:      "timeout",
		},
		"cursor": {
			ProviderID: "cursor",
			Success:    true,
			Snapshot:   makeSnapshot("cursor", 50, models.PeriodMonthly, ""),
			Cached:     true,
		},
	}

	data := BuildProviderData(outcomes)

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (claude, cursor), got %d", len(data))
	}
	if _, ok := data["claude"]; !ok {
		t.Error("missing claude")
	}
	if _, ok := data["cursor"]; !ok {
		t.Error("missing cursor")
	}
	if _, ok := data["copilot"]; ok {
		t.Error("copilot should not be in provider data (failed fetch)")
	}
	if data["cursor"].Cached != true {
		t.Error("cursor should be marked as cached")
	}
}

func TestBuildStrategyMap(t *testing.T) {
	called := false
	lookupFn := func(id string) []fetch.Strategy {
		called = true
		return []fetch.Strategy{}
	}

	result := BuildStrategyMap([]string{"claude", "cursor"}, lookupFn)

	if !called {
		t.Error("lookup function was not called")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestRouteModel_UnknownModel(t *testing.T) {
	svc := &Service{
		LookupModel:  func(q string) *ModelInfo { return nil },
		SearchModels: func(q string) []ModelInfo { return nil },
	}

	_, err := svc.RouteModel(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestRouteModel_NoConfiguredProviders(t *testing.T) {
	svc := &Service{
		LookupModel: func(q string) *ModelInfo {
			return &ModelInfo{ID: "gpt-5", Name: "GPT-5", Providers: []string{"codex"}}
		},
		ConfiguredProviders: func(ids []string) []string { return nil },
	}

	_, err := svc.RouteModel(context.Background(), "gpt-5")
	if err == nil {
		t.Fatal("expected error when no providers configured")
	}
}

func TestRouteModel_Success(t *testing.T) {
	snap := makeSnapshot("claude", 30, models.PeriodWeekly, "Pro")

	svc := &Service{
		LookupModel: func(q string) *ModelInfo {
			return &ModelInfo{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Providers: []string{"claude"}}
		},
		ConfiguredProviders: func(ids []string) []string { return ids },
		ProviderStrategies:  func(id string) []fetch.Strategy { return nil },
		FetchAll: func(ctx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome {
			return map[string]fetch.FetchOutcome{
				"claude": {ProviderID: "claude", Success: true, Snapshot: snap},
			}
		},
		LookupMultiplier: func(modelName string, providerID string) *float64 { return nil },
	}

	rec, err := svc.RouteModel(context.Background(), "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Best == nil {
		t.Fatal("expected best candidate")
	}
	if rec.Best.ProviderID != "claude" {
		t.Errorf("best = %q, want claude", rec.Best.ProviderID)
	}
	if rec.ModelName != "Claude Sonnet 4.6" {
		t.Errorf("model name = %q, want Claude Sonnet 4.6", rec.ModelName)
	}
}

func TestRouteByRole_UnknownRole(t *testing.T) {
	svc := &Service{
		GetRole:   func(name string) (*RoleConfig, bool) { return nil, false },
		RoleNames: func() []string { return nil },
	}

	_, err := svc.RouteByRole(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestRouteByRole_Success(t *testing.T) {
	snap := makeSnapshot("claude", 30, models.PeriodWeekly, "Pro")

	svc := &Service{
		GetRole: func(name string) (*RoleConfig, bool) {
			return &RoleConfig{Models: []string{"claude-sonnet-4-6"}}, true
		},
		MatchPrefix: func(id string) []ModelInfo {
			return []ModelInfo{{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Providers: []string{"claude"}}}
		},
		LookupModel:         func(q string) *ModelInfo { return nil },
		ConfiguredProviders: func(ids []string) []string { return ids },
		ProviderStrategies:  func(id string) []fetch.Strategy { return nil },
		FetchAll: func(ctx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome {
			return map[string]fetch.FetchOutcome{
				"claude": {ProviderID: "claude", Success: true, Snapshot: snap},
			}
		},
		LookupMultiplier: func(modelName string, providerID string) *float64 { return nil },
	}

	rec, err := svc.RouteByRole(context.Background(), "thinking")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Best == nil {
		t.Fatal("expected best candidate")
	}
	if rec.Best.ProviderID != "claude" {
		t.Errorf("best = %q, want claude", rec.Best.ProviderID)
	}
	if rec.Role != "thinking" {
		t.Errorf("role = %q, want thinking", rec.Role)
	}
}

func TestBuildModelRolesMap(t *testing.T) {
	roles := map[string]RoleConfig{
		"thinking": {Models: []string{"claude-opus-4-6"}},
		"coding":   {Models: []string{"claude-opus-4-6", "gpt-5"}},
	}

	matchPrefix := func(id string) []ModelInfo {
		switch id {
		case "claude-opus-4-6":
			return []ModelInfo{{ID: "claude-opus-4-6", Name: "Claude Opus 4.6"}}
		case "gpt-5":
			return []ModelInfo{{ID: "gpt-5", Name: "GPT-5"}}
		}
		return nil
	}

	lookupModel := func(q string) *ModelInfo { return nil }

	result := BuildModelRolesMap(roles, matchPrefix, lookupModel)

	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d: %v", len(result), result)
	}

	opusRoles := result["claude-opus-4-6"]
	if len(opusRoles) != 2 {
		t.Fatalf("expected 2 roles for opus, got %d: %v", len(opusRoles), opusRoles)
	}
	// Should be sorted
	if opusRoles[0] != "coding" || opusRoles[1] != "thinking" {
		t.Errorf("opus roles = %v, want [coding thinking]", opusRoles)
	}

	gptRoles := result["gpt-5"]
	if len(gptRoles) != 1 || gptRoles[0] != "coding" {
		t.Errorf("gpt-5 roles = %v, want [coding]", gptRoles)
	}
}
