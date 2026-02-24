package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	boldStyle      = lipgloss.NewStyle().Bold(true)
	greenStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	overageBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 1)
)

func colorStyle(color string) lipgloss.Style {
	switch color {
	case "green":
		return greenStyle
	case "yellow":
		return yellowStyle
	case "red":
		return redStyle
	default:
		return lipgloss.NewStyle()
	}
}

func RenderBar(utilization int, width int, color string) string {
	filled := max(0, min(utilization*width/100, width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return colorStyle(color).Render(bar)
}

func FormatPeriodLine(period models.UsagePeriod, nameWidth int) string {
	paceRatio := period.PaceRatio()
	color := models.PaceToColor(paceRatio, period.Utilization)

	bar := RenderBar(period.Utilization, 20, color)

	pctText := fmt.Sprintf("%d%%", period.Utilization)
	pctStyled := colorStyle(color).Render(pctText)
	pctPad := strings.Repeat(" ", max(0, 4-len(pctText)))

	name := period.Name
	if len(name) > nameWidth {
		name = name[:nameWidth]
	}
	namePad := strings.Repeat(" ", max(0, nameWidth-len(name)))

	reset := ""
	if d := period.TimeUntilReset(); d != nil {
		reset = dimStyle.Render("resets in " + models.FormatResetCountdown(d))
	}

	return boldStyle.Render(name) + namePad + "  " + bar + " " + pctPad + pctStyled + "    " + reset
}

// RenderSingleProvider renders a single provider in expanded format.
func RenderSingleProvider(snapshot models.UsageSnapshot, cached bool) string {
	var b strings.Builder

	// Title
	title := provider.DisplayName(snapshot.Provider)
	if cached {
		title += dimStyle.Render(" (" + formatAge(time.Since(snapshot.FetchedAt)) + " ago)")
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(separatorStyle.Render(strings.Repeat("━", 60)))
	b.WriteByte('\n')

	// Group periods
	session, weekly, daily, monthly := groupPeriods(snapshot.Periods)

	// Session periods
	for _, p := range session {
		b.WriteString(FormatPeriodLine(p, 16))
		b.WriteByte('\n')
	}

	// Longer periods
	longer := pickLonger(weekly, daily, monthly)
	if len(session) > 0 && len(longer.periods) > 0 {
		b.WriteByte('\n')
	}

	if len(longer.periods) > 0 {
		b.WriteString(boldStyle.Render(longer.header))
		b.WriteByte('\n')

		for _, p := range longer.periods {
			name := p.Name
			if p.Model == "" {
				if strings.Contains(name, "(") && strings.Contains(name, ")") {
					start := strings.Index(name, "(") + 1
					end := strings.Index(name, ")")
					name = "  " + name[start:end]
				} else if name == longer.header {
					name = "  All Models"
				} else {
					name = "  " + name
				}
			} else {
				name = "  " + name
			}

			paceRatio := p.PaceRatio()
			color := models.PaceToColor(paceRatio, p.Utilization)
			bar := RenderBar(p.Utilization, 20, color)

			pctText := fmt.Sprintf("%d%%", p.Utilization)
			pctStyled := colorStyle(color).Render(pctText)
			pctPad := strings.Repeat(" ", max(0, 4-len(pctText)))

			const subNameWidth = 18
			namePad := strings.Repeat(" ", max(0, subNameWidth-len(name)))

			reset := ""
			if d := p.TimeUntilReset(); d != nil {
				reset = dimStyle.Render("resets in " + models.FormatResetCountdown(d))
			}

			b.WriteString(boldStyle.Render(name) + namePad + "  " + bar + " " + pctPad + pctStyled + "    " + reset + "\n")
		}
	}

	// Overage
	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		o := snapshot.Overage
		sym := ""
		if o.Currency == "USD" {
			sym = "$"
		}
		overageText := fmt.Sprintf("Extra Usage: %s%.2f / %s%.2f %s", sym, o.Used, sym, o.Limit, o.Currency)
		b.WriteByte('\n')
		b.WriteString(overageBorder.Render(overageText))
		b.WriteByte('\n')
	}

	return b.String()
}

// PeriodColWidths holds pre-computed column widths for renderPeriodTable.
// Compute once across all panels with GlobalPeriodColWidths so every provider
// box shares the same column widths and renders at the same total width.
type PeriodColWidths struct{ Name, Pct, Reset int }

// GlobalPeriodColWidths computes the widest values for each column across all
// provided snapshots, using the same name normalisations as RenderProviderPanel.
func GlobalPeriodColWidths(snapshots []models.UsageSnapshot) PeriodColWidths {
	var cw PeriodColWidths
	for _, s := range snapshots {
		for _, p := range collectDisplayPeriods(s) {
			cw.Name = max(cw.Name, len(p.Name))
			cw.Pct = max(cw.Pct, len(fmt.Sprintf("%d%%", p.Utilization)))
			if d := p.TimeUntilReset(); d != nil {
				cw.Reset = max(cw.Reset, len("resets in "+models.FormatResetCountdown(d)))
			}
		}
	}
	return cw
}

// collectDisplayPeriods returns the periods shown in the compact panel view,
// with period-type name normalisation already applied ("Weekly", "Daily", etc.).
func collectDisplayPeriods(snapshot models.UsageSnapshot) []models.UsagePeriod {
	session, weekly, daily, monthly := groupPeriods(snapshot.Periods)
	var out []models.UsagePeriod
	for _, p := range session {
		if p.Model == "" {
			out = append(out, p)
		}
	}
	for _, p := range weekly {
		if p.Model == "" {
			if !strings.Contains(p.Name, "(") {
				p.Name = "Weekly"
			}
			out = append(out, p)
		}
	}
	for _, p := range daily {
		if p.Model == "" {
			if !strings.Contains(p.Name, "(") {
				p.Name = "Daily"
			}
			out = append(out, p)
		}
	}
	for _, p := range monthly {
		if p.Model == "" {
			out = append(out, p)
		}
	}
	return out
}

// renderPeriodTable renders a slice of periods as aligned rows using the
// provided column widths.  No characters are hand-counted: the caller supplies
// widths computed from the full dataset so every panel lines up identically.
func renderPeriodTable(periods []models.UsagePeriod, cw PeriodColWidths) string {
	lines := make([]string, 0, len(periods))
	for _, p := range periods {
		color := models.PaceToColor(p.PaceRatio(), p.Utilization)
		pctRaw := fmt.Sprintf("%d%%", p.Utilization)

		resetRaw := ""
		if d := p.TimeUntilReset(); d != nil {
			resetRaw = "resets in " + models.FormatResetCountdown(d)
		}

		namePad := strings.Repeat(" ", max(0, cw.Name-len(p.Name)))
		pctPad := strings.Repeat(" ", max(0, cw.Pct-len(pctRaw)))
		resetPad := strings.Repeat(" ", max(0, cw.Reset-len(resetRaw)))

		lines = append(lines,
			p.Name+namePad+
				"  "+RenderBar(p.Utilization, 20, color)+
				" "+pctPad+colorStyle(color).Render(pctRaw)+
				"    "+dimStyle.Render(resetRaw)+resetPad,
		)
	}
	return strings.Join(lines, "\n")
}

// RenderProviderPanel renders a provider in compact panel format for multi-provider view.
// Pass column widths from GlobalPeriodColWidths so all panels share identical column sizing.
func RenderProviderPanel(snapshot models.UsageSnapshot, cached bool, cw PeriodColWidths) string {
	var b strings.Builder

	b.WriteString(renderPeriodTable(collectDisplayPeriods(snapshot), cw))

	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		o := snapshot.Overage
		sym := ""
		if o.Currency == "USD" {
			sym = "$"
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "Extra: %s%.2f / %s%.2f %s", sym, o.Used, sym, o.Limit, o.Currency)
	}

	title := titleStyle.Render(provider.DisplayName(snapshot.Provider))
	if cached {
		title += dimStyle.Render(" (" + formatAge(time.Since(snapshot.FetchedAt)) + " ago)")
	}
	return renderTitledPanel(title, b.String())
}

func renderTitledPanel(title string, body string) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	bodyWidth := 0
	for _, line := range lines {
		bodyWidth = max(bodyWidth, lipgloss.Width(line))
	}

	innerWidth := max(bodyWidth+2, lipgloss.Width(title)+1)
	top := separatorStyle.Render("╭─") + title + separatorStyle.Render(strings.Repeat("─", max(0, innerWidth-lipgloss.Width(title)-1))+"╮")
	bottom := separatorStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	rows := make([]string, 0, len(lines)+2)
	rows = append(rows, top)
	for _, line := range lines {
		pad := strings.Repeat(" ", max(0, bodyWidth-lipgloss.Width(line)))
		rows = append(rows, separatorStyle.Render("│")+" "+line+pad+" "+separatorStyle.Render("│"))
	}
	rows = append(rows, bottom)

	return strings.Join(rows, "\n")
}

// formatAge formats a duration as a compact human-readable age string.
func formatAge(d time.Duration) string {
	if d.Hours() >= 24 {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if d.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return "<1m"
}

type longerPeriods struct {
	header  string
	periods []models.UsagePeriod
}

func pickLonger(weekly, daily, monthly []models.UsagePeriod) longerPeriods {
	if len(weekly) > 0 {
		return longerPeriods{"Weekly", weekly}
	}
	if len(daily) > 0 {
		return longerPeriods{"Daily", daily}
	}
	if len(monthly) > 0 {
		return longerPeriods{"Monthly", monthly}
	}
	return longerPeriods{}
}

func groupPeriods(periods []models.UsagePeriod) (session, weekly, daily, monthly []models.UsagePeriod) {
	for _, p := range periods {
		switch p.PeriodType {
		case models.PeriodSession:
			session = append(session, p)
		case models.PeriodWeekly:
			weekly = append(weekly, p)
		case models.PeriodDaily:
			daily = append(daily, p)
		case models.PeriodMonthly:
			monthly = append(monthly, p)
		}
	}
	return
}

// RenderProviderError renders a compact error line for a failed provider.
// Only suggests auth when the error is actually about missing credentials.
func RenderProviderError(providerID string, errMsg string) string {
	name := provider.DisplayName(providerID)
	line := dimStyle.Render(name + ": " + errMsg)
	if isCredentialError(errMsg) {
		line += dimStyle.Render("  (vibeusage auth " + providerID + ")")
	}
	return line
}

func isCredentialError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	for _, s := range []string{"not configured", "no credentials", "no oauth", "no strategies"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// StatusSymbol returns a colored status indicator symbol.
// When noColor is true, the plain symbol is returned without ANSI styling.
func StatusSymbol(level models.StatusLevel, noColor bool) string {
	sym := "?"
	var style lipgloss.Style

	switch level {
	case models.StatusOperational:
		sym = "●"
		style = greenStyle
	case models.StatusDegraded:
		sym = "◐"
		style = yellowStyle
	case models.StatusPartialOutage:
		sym = "◑"
		style = yellowStyle
	case models.StatusMajorOutage:
		sym = "○"
		style = redStyle
	default:
		style = dimStyle
	}

	if noColor {
		return sym
	}
	return style.Render(sym)
}

func FormatStatusUpdated(t *time.Time) string {
	if t == nil {
		return "unknown"
	}
	d := time.Since(*t)
	if d.Hours() >= 24 {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	if d.Hours() >= 1 {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d.Minutes() >= 1 {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return "just now"
}
