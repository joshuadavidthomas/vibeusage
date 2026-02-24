package display

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CompletionInfo describes a completed provider fetch.
type CompletionInfo struct {
	ProviderID string
	Success    bool
	Error      string
}

// SpinnerShouldShow returns true if the spinner should be displayed.
// The spinner is hidden for quiet mode, JSON output, or non-TTY (piped) output.
func SpinnerShouldShow(quiet, json, nonTTY bool) bool {
	return !quiet && !json && !nonTTY
}

// SpinnerRun starts a spinner tracking the given provider IDs.
// It calls fetchFn, passing a callback that fetchFn should invoke
// when each provider completes. SpinnerRun blocks until all providers finish.
func SpinnerRun(providerIDs []string, fetchFn func(onComplete func(CompletionInfo))) error {
	if len(providerIDs) == 0 {
		fetchFn(func(CompletionInfo) {})
		return nil
	}

	m := newSpinnerModel(providerIDs)
	p := tea.NewProgram(m)

	done := make(chan struct{})
	go func() {
		fetchFn(func(info CompletionInfo) {
			p.Send(spinnerCompletionMsg(info))
		})
		close(done)
	}()

	_, err := p.Run()
	<-done
	if err != nil {
		return fmt.Errorf("running spinner: %w", err)
	}
	return nil
}

// spinnerCompletionMsg is sent to the model when a provider fetch completes.
type spinnerCompletionMsg CompletionInfo

type spinnerModel struct {
	spinner      spinner.Model
	allProviders []string
	inflight     []string
	completions  map[string]CompletionInfo
	quitting     bool
}

var (
	spinnerCheckStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	spinnerErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func newSpinnerModel(providerIDs []string) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	allProviders := make([]string, len(providerIDs))
	copy(allProviders, providerIDs)

	inflight := make([]string, len(providerIDs))
	copy(inflight, providerIDs)

	return spinnerModel{
		spinner:      s,
		allProviders: allProviders,
		inflight:     inflight,
		completions:  make(map[string]CompletionInfo),
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerCompletionMsg:
		info := CompletionInfo(msg)

		// Ignore duplicates
		if !m.isInflight(info.ProviderID) {
			return m, nil
		}

		m.completions[info.ProviderID] = info
		m.inflight = removeStringFromSlice(m.inflight, info.ProviderID)

		if len(m.inflight) == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m spinnerModel) View() string {
	// When done, return empty — the spinner is transient progress UI
	if m.quitting {
		return ""
	}

	var b strings.Builder

	for i, id := range m.allProviders {
		if i > 0 {
			b.WriteString("\n")
		}

		if c, done := m.completions[id]; done {
			if c.Success {
				b.WriteString(spinnerCheckStyle.Render("✓"))
			} else {
				b.WriteString(spinnerErrStyle.Render("✗"))
			}
			b.WriteString(" ")
			b.WriteString(c.ProviderID)
		} else {
			b.WriteString(m.spinner.View())
			b.WriteString(" ")
			b.WriteString(id)
		}
	}

	return b.String()
}

func (m spinnerModel) isInflight(providerID string) bool {
	for _, id := range m.inflight {
		if id == providerID {
			return true
		}
	}
	return false
}

func removeStringFromSlice(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
