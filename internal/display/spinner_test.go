package display

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSpinnerShouldShow(t *testing.T) {
	tests := []struct {
		name   string
		quiet  bool
		json   bool
		nonTTY bool
		want   bool
	}{
		{"interactive", false, false, false, true},
		{"quiet mode", true, false, false, false},
		{"json mode", false, true, false, false},
		{"both quiet and json", true, true, false, false},
		{"non-TTY (piped)", false, false, true, false},
		{"quiet and non-TTY", true, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpinnerShouldShow(tt.quiet, tt.json, tt.nonTTY)
			if got != tt.want {
				t.Errorf("SpinnerShouldShow(quiet=%v, json=%v, nonTTY=%v) = %v, want %v",
					tt.quiet, tt.json, tt.nonTTY, got, tt.want)
			}
		})
	}
}

func TestSpinnerRunEmptyProviders(t *testing.T) {
	called := false
	err := SpinnerRun([]string{}, func(onComplete func(CompletionInfo)) {
		called = true
	})
	if err != nil {
		t.Errorf("SpinnerRun with empty providers returned error: %v", err)
	}
	if !called {
		t.Error("expected fetchFn to be called even with empty providers")
	}
}

func TestNewSpinnerModel(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newSpinnerModel(providers)

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

func TestSpinnerModelUpdateCompletion(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newSpinnerModel(providers)

	updated, cmd := m.Update(spinnerCompletionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(spinnerModel)

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

func TestSpinnerModelUpdateAllComplete(t *testing.T) {
	providers := []string{"claude", "copilot"}
	m := newSpinnerModel(providers)

	updated, _ := m.Update(spinnerCompletionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(spinnerModel)

	updated, cmd := m.Update(spinnerCompletionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(spinnerModel)

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

func TestSpinnerModelUpdateDuplicateCompletion(t *testing.T) {
	providers := []string{"claude", "copilot"}
	m := newSpinnerModel(providers)

	updated, _ := m.Update(spinnerCompletionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(spinnerModel)

	updated, _ = m.Update(spinnerCompletionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(spinnerModel)

	if len(m.completions) != 1 {
		t.Errorf("expected 1 completion (duplicate ignored), got %d", len(m.completions))
	}
}

func TestSpinnerModelUpdateCtrlC(t *testing.T) {
	providers := []string{"claude"}
	m := newSpinnerModel(providers)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(spinnerModel)

	if !m.quitting {
		t.Error("expected quitting=true on ctrl+c")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestSpinnerModelViewInflight(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newSpinnerModel(providers)

	view := m.View()

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

func TestSpinnerModelViewPartialCompletion(t *testing.T) {
	providers := []string{"claude", "copilot", "gemini"}
	m := newSpinnerModel(providers)

	updated, _ := m.Update(spinnerCompletionMsg{
		ProviderID: "copilot",
		Success:    true,
	})
	m = updated.(spinnerModel)

	view := m.View()

	for _, p := range providers {
		if !strings.Contains(view, p) {
			t.Errorf("expected provider %q in view, got %q", p, view)
		}
	}
	if !strings.Contains(view, "✓") {
		t.Errorf("expected checkmark for completed copilot, got %q", view)
	}
}

func TestSpinnerModelViewAllDone(t *testing.T) {
	providers := []string{"claude"}
	m := newSpinnerModel(providers)

	updated, _ := m.Update(spinnerCompletionMsg{
		ProviderID: "claude",
		Success:    true,
	})
	m = updated.(spinnerModel)

	view := m.View()

	if view != "" {
		t.Errorf("expected empty final view, got %q", view)
	}
}

func TestSpinnerModelViewShowsFailures(t *testing.T) {
	providers := []string{"claude", "cursor"}
	m := newSpinnerModel(providers)

	updated, _ := m.Update(spinnerCompletionMsg{
		ProviderID: "cursor",
		Success:    false,
		Error:      "not configured",
	})
	m = updated.(spinnerModel)

	view := m.View()

	if !strings.Contains(view, "✗") {
		t.Errorf("expected failure indicator ✗ in view, got %q", view)
	}
	if !strings.Contains(view, "cursor") {
		t.Errorf("expected failed provider cursor in view, got %q", view)
	}
	if !strings.Contains(view, "claude") {
		t.Errorf("expected inflight provider claude in view, got %q", view)
	}
}
