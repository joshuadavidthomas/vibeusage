package routing

import (
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
