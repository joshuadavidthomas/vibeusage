package spinner

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newModel(providers)

	if len(m.inflight) != 3 {
		t.Errorf("expected 3 inflight, got %d", len(m.inflight))
	}
	if len(m.completions) != 0 {
		t.Errorf("expected 0 completions, got %d", len(m.completions))
	}
	if m.quitting {
		t.Error("expected quitting=false")
	}
}

func TestModelUpdateCompletion(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newModel(providers)

	// Complete one provider
	updated, cmd := m.Update(completionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(model)

	if len(m.inflight) != 2 {
		t.Errorf("expected 2 inflight, got %d", len(m.inflight))
	}
	if len(m.completions) != 1 {
		t.Errorf("expected 1 completion, got %d", len(m.completions))
	}
	if c, ok := m.completions["copilot"]; !ok || c.ProviderID != "copilot" {
		t.Errorf("expected completions[copilot] to exist")
	}
	if m.quitting {
		t.Error("should not be quitting yet")
	}
	if cmd != nil {
		t.Error("expected no command after non-final completion")
	}
}

func TestModelUpdateAllComplete(t *testing.T) {
	providers := []string{"claude", "copilot"}
	m := newModel(providers)

	// Complete first
	updated, _ := m.Update(completionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(model)

	// Complete second — should trigger quit
	updated, cmd := m.Update(completionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(model)

	if len(m.inflight) != 0 {
		t.Errorf("expected 0 inflight, got %d", len(m.inflight))
	}
	if !m.quitting {
		t.Error("expected quitting=true when all complete")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestModelUpdateDuplicateCompletion(t *testing.T) {
	providers := []string{"claude", "copilot"}
	m := newModel(providers)

	// Complete claude
	updated, _ := m.Update(completionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(model)

	// Complete claude again (duplicate) — should be ignored
	updated, _ = m.Update(completionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(model)

	if len(m.completions) != 1 {
		t.Errorf("expected 1 completion (duplicate ignored), got %d", len(m.completions))
	}
}

func TestModelUpdateCtrlC(t *testing.T) {
	providers := []string{"claude"}
	m := newModel(providers)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)

	if !m.quitting {
		t.Error("expected quitting=true on ctrl+c")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestModelViewInflight(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newModel(providers)

	view := m.View()

	// Each provider should have its own line with a spinner
	for _, p := range providers {
		if !strings.Contains(view, p) {
			t.Errorf("expected provider %q in view, got %q", p, view)
		}
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (one per provider), got %d: %q", len(lines), view)
	}
}

func TestModelViewPartialCompletion(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newModel(providers)

	updated, _ := m.Update(completionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(model)

	view := m.View()

	// All providers should still be visible, each on its own line
	for _, p := range providers {
		if !strings.Contains(view, p) {
			t.Errorf("expected provider %q in view, got %q", p, view)
		}
	}
	// Copilot's line should have a checkmark
	if !strings.Contains(view, "✓") {
		t.Errorf("expected checkmark for completed copilot, got %q", view)
	}
}

func TestModelViewAllDone(t *testing.T) {
	providers := []string{"claude"}
	m := newModel(providers)

	updated, _ := m.Update(completionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(model)

	view := m.View()

	// Final view should be empty — spinner is transient progress UI
	if view != "" {
		t.Errorf("expected empty final view, got %q", view)
	}
}

func TestModelViewShowsFailures(t *testing.T) {
	providers := []string{"claude", "cursor"}
	m := newModel(providers)

	// Complete cursor with failure
	updated, _ := m.Update(completionMsg{
		ProviderID: "cursor",
		Success:    false,
		Error:      "not configured",
	})
	m = updated.(model)

	view := m.View()

	// Failed provider should show with ✗
	if !strings.Contains(view, "✗") {
		t.Errorf("expected failure indicator ✗ in view, got %q", view)
	}
	if !strings.Contains(view, "cursor") {
		t.Errorf("expected failed provider cursor in view, got %q", view)
	}
	// Should still show claude as inflight
	if !strings.Contains(view, "claude") {
		t.Errorf("expected inflight provider claude in view, got %q", view)
	}
}
