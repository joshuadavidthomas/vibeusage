package display

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// TableOptions configures table rendering.
type TableOptions struct {
	Title   string
	NoColor bool
}

// NewTable creates a styled table with headers and rows using lipgloss/table.
func NewTable(headers []string, rows [][]string) string {
	return NewTableWithOptions(headers, rows, TableOptions{})
}

// NewTableWithOptions creates a styled table with the given options.
func NewTableWithOptions(headers []string, rows [][]string, opts TableOptions) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	if opts.NoColor {
		headerStyle = lipgloss.NewStyle().Padding(0, 1)
		borderStyle = lipgloss.NewStyle()
	}

	t := table.New().
		Headers(headers...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	for _, row := range rows {
		t.Row(row...)
	}

	rendered := t.String()

	if opts.Title != "" {
		title := lipgloss.NewStyle().Bold(true).Render(opts.Title)
		if opts.NoColor {
			title = opts.Title
		}
		return title + "\n" + rendered
	}

	return rendered
}
