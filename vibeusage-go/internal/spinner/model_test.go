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
	if len(m.completed) != 0 {
		t.Errorf("expected 0 completed, got %d", len(m.completed))
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
		Source:     "device_flow",
		DurationMs: 189,
		Success:    true,
	})
	m = updated.(model)

	if len(m.inflight) != 2 {
		t.Errorf("expected 2 inflight, got %d", len(m.inflight))
	}
	if len(m.completed) != 1 {
		t.Errorf("expected 1 completed, got %d", len(m.completed))
	}
	if m.completed[0].ProviderID != "copilot" {
		t.Errorf("expected completed[0].ProviderID=copilot, got %s", m.completed[0].ProviderID)
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
		Source:     "oauth",
		DurationMs: 100,
		Success:    true,
	})
	m = updated.(model)

	// Complete second — should trigger quit
	updated, cmd := m.Update(completionMsg{
		ProviderID: "copilot",
		Source:     "device_flow",
		DurationMs: 200,
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
		Source:     "oauth",
		DurationMs: 100,
		Success:    true,
	})
	m = updated.(model)

	// Complete claude again (duplicate) — should be ignored
	updated, _ = m.Update(completionMsg{
		ProviderID: "claude",
		Source:     "oauth",
		DurationMs: 200,
		Success:    true,
	})
	m = updated.(model)

	if len(m.completed) != 1 {
		t.Errorf("expected 1 completed (duplicate ignored), got %d", len(m.completed))
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

	if !strings.Contains(view, "Fetching claude, copilot, gemini...") {
		t.Errorf("expected inflight title in view, got %q", view)
	}
}

func TestModelViewPartialCompletion(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newModel(providers)

	updated, _ := m.Update(completionMsg{
		ProviderID: "copilot",
		Source:     "device_flow",
		DurationMs: 189,
		Success:    true,
	})
	m = updated.(model)

	view := m.View()

	// Should contain the completed provider line
	if !strings.Contains(view, "copilot") {
		t.Errorf("expected completed copilot in view, got %q", view)
	}
	// Should contain the inflight title with remaining providers
	if !strings.Contains(view, "Fetching claude, gemini...") {
		t.Errorf("expected remaining inflight in view, got %q", view)
	}
}

func TestModelViewAllDone(t *testing.T) {
	providers := []string{"claude"}
	m := newModel(providers)

	updated, _ := m.Update(completionMsg{
		ProviderID: "claude",
		Source:     "oauth",
		DurationMs: 342,
		Success:    true,
	})
	m = updated.(model)

	view := m.View()

	// Should show the completed line
	if !strings.Contains(view, "claude") {
		t.Errorf("expected completed claude in view, got %q", view)
	}
	// Should NOT contain "Fetching" since all are done
	if strings.Contains(view, "Fetching") {
		t.Errorf("expected no Fetching in final view, got %q", view)
	}
}
