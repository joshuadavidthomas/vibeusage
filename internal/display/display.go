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
	color := PaceToColor(paceRatio, period.Utilization)

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
		reset = dimStyle.Render("resets in " + FormatResetCountdown(d))
	}

	return boldStyle.Render(name) + namePad + "  " + bar + " " + pctPad + pctStyled + "    " + reset
}

// renderPeriodRow renders a single period as an aligned row using the provided
// column widths. The displayName allows the caller to override the name shown
// (e.g. for indented sub-period names).
func renderPeriodRow(p models.UsagePeriod, displayName string, cw PeriodColWidths) string {
	color := PaceToColor(p.PaceRatio(), p.Utilization)
	pctRaw := fmt.Sprintf("%d%%", p.Utilization)

	resetRaw := ""
	if d := p.TimeUntilReset(); d != nil {
		resetRaw = "resets in " + FormatResetCountdown(d)
	}

	namePad := strings.Repeat(" ", max(0, cw.Name-len(displayName)))
	pctPad := strings.Repeat(" ", max(0, cw.Pct-len(pctRaw)))

	return boldStyle.Render(displayName) + namePad +
		"  " + RenderBar(p.Utilization, 20, color) +
		" " + pctPad + colorStyle(color).Render(pctRaw) +
		"    " + dimStyle.Render(resetRaw)
}

// formatSubPeriodName returns the display name for a period within a section
// group (e.g. "  All Models", "  Sonnet"). Names are indented with two spaces.
func formatSubPeriodName(p *models.UsagePeriod, sectionHeader string) string {
	name := p.Name
	if p.Model == "" {
		if strings.Contains(name, "(") && strings.Contains(name, ")") {
			start := strings.Index(name, "(") + 1
			end := strings.Index(name, ")")
			return "  " + name[start:end]
		}
		if name == sectionHeader {
			return "  All Models"
		}
		return "  " + name
	}
	return "  " + name
}

// formatOverageLine formats an overage usage line with the given label prefix.
// When the limit is zero (no hard limit set), the limit portion is omitted to
// avoid confusing output like "$73.72 / $0.00".
func formatOverageLine(o *models.OverageUsage, label string) string {
	sym := ""
	if o.Currency == "USD" {
		sym = "$"
	}
	if o.Limit > 0 {
		return fmt.Sprintf("%s: %s%.2f / %s%.2f %s", label, sym, o.Used, sym, o.Limit, o.Currency)
	}
	return fmt.Sprintf("%s: %s%.2f %s", label, sym, o.Used, o.Currency)
}

// DetailOptions configures the single-provider detail view.
type DetailOptions struct {
	// Status is the provider's health status, fetched separately.
	Status *models.ProviderStatus
}

// RenderSingleProvider renders a single provider in expanded detail format
// with a provider title above a "Usage" panel, plus optional status info.
func RenderSingleProvider(snapshot models.UsageSnapshot, cached bool, opts DetailOptions) string {
	var out strings.Builder

	// Provider title above the card
	providerTitle := titleStyle.Render(provider.DisplayName(snapshot.Provider))
	if cached {
		providerTitle += dimStyle.Render(" (" + formatAge(time.Since(snapshot.FetchedAt)) + " ago)")
	}
	if snapshot.Identity != nil {
		parts := identitySummary(snapshot.Identity)
		if parts != "" {
			providerTitle += dimStyle.Render("  " + parts)
		}
	}
	out.WriteString(providerTitle)
	out.WriteByte('\n')

	// Status line between title and usage card
	if opts.Status != nil {
		out.WriteString(renderStatusLine(*opts.Status))
		out.WriteByte('\n')
	}

	// Usage panel
	out.WriteString(renderUsagePanel(snapshot))

	return out.String()
}

// identitySummary returns a compact string from provider identity fields.
func identitySummary(id *models.ProviderIdentity) string {
	var parts []string
	if id.Plan != "" {
		parts = append(parts, id.Plan)
	}
	if id.Organization != "" {
		parts = append(parts, id.Organization)
	}
	if id.Email != "" {
		parts = append(parts, id.Email)
	}
	return strings.Join(parts, " · ")
}

// renderStatusLine renders a compact status indicator line.
func renderStatusLine(status models.ProviderStatus) string {
	sym := StatusSymbol(status.Level, false)
	desc := string(status.Level)
	if status.Description != "" {
		desc = status.Description
	}
	line := sym + " " + desc
	if status.UpdatedAt != nil {
		line += dimStyle.Render("  " + FormatStatusUpdated(status.UpdatedAt))
	}
	return line
}

// renderUsagePanel renders the usage data inside a titled "Usage" panel.
func renderUsagePanel(snapshot models.UsageSnapshot) string {
	var b strings.Builder

	// Group periods
	session, weekly, daily, monthly := groupPeriods(snapshot.Periods)
	longer := pickLonger(weekly, daily, monthly)

	// Compute column widths across all displayed periods
	cw := detailColWidths(session, longer)

	// Session periods
	for i, p := range session {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderPeriodRow(p, p.Name, cw))
	}

	// Longer periods with per-model breakdowns
	if len(session) > 0 && len(longer.periods) > 0 {
		b.WriteString("\n\n")
	}

	if len(longer.periods) > 0 {
		b.WriteString(boldStyle.Render(longer.header))

		for _, p := range longer.periods {
			name := formatSubPeriodName(&p, longer.header)
			b.WriteByte('\n')
			b.WriteString(renderPeriodRow(p, name, cw))
		}
	}

	// Overage
	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		b.WriteString("\n\n")
		b.WriteString(formatOverageLine(snapshot.Overage, "Extra Usage"))
	}

	return renderTitledPanel(titleStyle.Render("Usage"), b.String())
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
				cw.Reset = max(cw.Reset, len("resets in "+FormatResetCountdown(d)))
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

// detailColWidths computes column widths for the single-provider detail view
// across both session periods and the longer period group (including sub-periods).
func detailColWidths(session []models.UsagePeriod, longer longerPeriods) PeriodColWidths {
	var cw PeriodColWidths

	for _, p := range session {
		cw.Name = max(cw.Name, len(p.Name))
		cw.Pct = max(cw.Pct, len(fmt.Sprintf("%d%%", p.Utilization)))
		if d := p.TimeUntilReset(); d != nil {
			cw.Reset = max(cw.Reset, len("resets in "+FormatResetCountdown(d)))
		}
	}

	for i := range longer.periods {
		p := &longer.periods[i]
		name := formatSubPeriodName(p, longer.header)
		cw.Name = max(cw.Name, len(name))
		cw.Pct = max(cw.Pct, len(fmt.Sprintf("%d%%", p.Utilization)))
		if d := p.TimeUntilReset(); d != nil {
			cw.Reset = max(cw.Reset, len("resets in "+FormatResetCountdown(d)))
		}
	}

	return cw
}

// renderPeriodTable renders a slice of periods as aligned rows using the
// provided column widths.  No characters are hand-counted: the caller supplies
// widths computed from the full dataset so every panel lines up identically.
func renderPeriodTable(periods []models.UsagePeriod, cw PeriodColWidths) string {
	lines := make([]string, 0, len(periods))
	for _, p := range periods {
		lines = append(lines, renderPeriodRow(p, p.Name, cw))
	}
	return strings.Join(lines, "\n")
}

// RenderProviderPanel renders a provider in compact panel format for multi-provider view.
// Pass column widths from GlobalPeriodColWidths so all panels share identical column sizing.
func RenderProviderPanel(snapshot models.UsageSnapshot, cached bool, cw PeriodColWidths) string {
	var b strings.Builder

	b.WriteString(renderPeriodTable(collectDisplayPeriods(snapshot), cw))

	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		b.WriteByte('\n')
		b.WriteString(formatOverageLine(snapshot.Overage, "Extra"))
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
