package display

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

var (
	boldDescStyle  = lipgloss.NewStyle().Bold(true)
	mutedDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimDescStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// FormatStatusRows builds table headers and rows from a provider status map.
// Rows are sorted by provider ID. Descriptions are truncated to 30 characters.
// Operational and unknown descriptions are dimmed so degraded/outage
// descriptions stand out.
func FormatStatusRows(statuses map[string]models.ProviderStatus, noColor bool) (headers []string, rows [][]string) {
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	headers = []string{"Provider", "Status", "Description", "Updated"}

	for _, pid := range ids {
		s := statuses[pid]
		desc := s.DisplayDescription()
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		if !noColor {
			switch s.Level {
			case models.StatusMajorOutage:
				desc = boldDescStyle.Render(desc)
			case models.StatusOperational:
				desc = mutedDescStyle.Render(desc)
			case models.StatusUnknown:
				desc = dimDescStyle.Render(desc)
			}
		}
		rows = append(rows, []string{
			pid,
			StatusSymbol(s.Level, noColor),
			desc,
			FormatStatusUpdated(s.UpdatedAt),
		})
	}

	return headers, rows
}
