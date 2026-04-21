package display

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func RenderUpdateCheckHeader(currentVersion, latestVersion string, noColor bool) string {
	text := fmt.Sprintf("Update available: %s → %s", currentVersion, latestVersion)
	if noColor {
		return "↑ " + text
	}

	arrow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).Render("↑")
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(text)
	return arrow + " " + body
}
