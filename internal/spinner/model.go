package spinner

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// completionMsg is sent to the model when a provider fetch completes.
type completionMsg CompletionInfo

type model struct {
	spinner   spinner.Model
	inflight  []string
	completed []CompletionInfo
	quitting  bool
}

var (
	checkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	crossStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func newModel(providerIDs []string) model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	inflight := make([]string, len(providerIDs))
	copy(inflight, providerIDs)

	return model{
		spinner:  s,
		inflight: inflight,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case completionMsg:
		info := CompletionInfo(msg)

		// Ignore duplicates
		if !m.isInflight(info.ProviderID) {
			return m, nil
		}

		m.completed = append(m.completed, info)
		m.inflight = removeString(m.inflight, info.ProviderID)

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

func (m model) View() string {
	// When done, return empty — the spinner is transient progress UI
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Show completed providers (only successes)
	for _, c := range m.completed {
		if !c.Success {
			continue
		}
		b.WriteString(checkStyle.Render("✓"))
		b.WriteString(" ")
		b.WriteString(FormatCompletionText(c))
		b.WriteString("\n")
	}

	// Show spinner with in-flight providers
	if len(m.inflight) > 0 {
		b.WriteString(m.spinner.View())
		b.WriteString(" ")
		b.WriteString(FormatTitle(m.inflight))
	}

	return b.String()
}

func (m model) isInflight(providerID string) bool {
	for _, id := range m.inflight {
		if id == providerID {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
