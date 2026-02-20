package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	boldStyle      = lipgloss.NewStyle().Bold(true)
	greenStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

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
	filled := utilization * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return colorStyle(color).Render(bar)
}

func FormatPeriodLine(period models.UsagePeriod, nameWidth int) string {
	paceRatio := period.PaceRatio()
	color := models.PaceToColor(paceRatio, period.Utilization)

	bar := RenderBar(period.Utilization, 20, color)
	pct := colorStyle(color).Render(fmt.Sprintf("%d%%", period.Utilization))

	name := period.Name
	if len(name) > nameWidth {
		name = name[:nameWidth]
	}

	reset := ""
	if d := period.TimeUntilReset(); d != nil {
		reset = dimStyle.Render("resets in " + models.FormatResetCountdown(d))
	}

	return fmt.Sprintf("%-*s  %s %4s    %s", nameWidth, boldStyle.Render(name), bar, pct, reset)
}

// RenderSingleProvider renders a single provider in expanded format.
func RenderSingleProvider(snapshot models.UsageSnapshot, cached bool, verbose bool) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(strutil.TitleCase(snapshot.Provider)))
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
			pct := colorStyle(color).Render(fmt.Sprintf("%d%%", p.Utilization))
			reset := ""
			if d := p.TimeUntilReset(); d != nil {
				reset = dimStyle.Render("resets in " + models.FormatResetCountdown(d))
			}

			b.WriteString(fmt.Sprintf("%-18s  %s %4s    %s\n", boldStyle.Render(name), bar, pct, reset))
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

// RenderProviderPanel renders a provider in compact panel format for multi-provider view.
func RenderProviderPanel(snapshot models.UsageSnapshot, cached bool) string {
	var b strings.Builder

	session, weekly, daily, monthly := groupPeriods(snapshot.Periods)

	// Only general periods (no model-specific) in compact view
	nameWidth := 22
	for _, p := range session {
		if p.Model == "" {
			b.WriteString(FormatPeriodLine(p, nameWidth))
			b.WriteByte('\n')
		}
	}
	for _, p := range weekly {
		if p.Model == "" {
			name := p.Name
			if !strings.Contains(name, "(") {
				name = "Weekly"
			}
			pp := p
			pp.Name = name
			b.WriteString(FormatPeriodLine(pp, nameWidth))
			b.WriteByte('\n')
		}
	}
	for _, p := range daily {
		if p.Model == "" {
			name := p.Name
			if !strings.Contains(name, "(") {
				name = "Daily"
			}
			pp := p
			pp.Name = name
			b.WriteString(FormatPeriodLine(pp, nameWidth))
			b.WriteByte('\n')
		}
	}
	for _, p := range monthly {
		if p.Model == "" {
			b.WriteString(FormatPeriodLine(p, nameWidth))
			b.WriteByte('\n')
		}
	}

	// Overage
	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		o := snapshot.Overage
		sym := ""
		if o.Currency == "USD" {
			sym = "$"
		}
		b.WriteString(fmt.Sprintf("Extra: %s%.2f / %s%.2f %s\n", sym, o.Used, sym, o.Limit, o.Currency))
	}

	content := strings.TrimRight(b.String(), "\n")
	title := strutil.TitleCase(snapshot.Provider)
	return panelBorder.Render(title + "\n" + content)
}

func RenderStaleWarning(snapshot models.UsageSnapshot, maxAgeMinutes int) string {
	age := time.Since(snapshot.FetchedAt)
	ageMinutes := int(age.Minutes())
	if ageMinutes < maxAgeMinutes {
		return ""
	}

	var ageStr string
	if ageMinutes < 60 {
		ageStr = fmt.Sprintf("%d minute", ageMinutes)
		if ageMinutes != 1 {
			ageStr += "s"
		}
	} else {
		hours := ageMinutes / 60
		ageStr = fmt.Sprintf("%d hour", hours)
		if hours != 1 {
			ageStr += "s"
		}
	}

	return yellowStyle.Render(fmt.Sprintf("⚠ Showing cached data from %s ago", ageStr)) + "\n" +
		dimStyle.Render("Run with --refresh to fetch fresh data")
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
