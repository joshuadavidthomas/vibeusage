package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
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

// periodTableRow represents one period entry for the table builder.
// The displayName allows overriding the name shown (e.g. indented sub-periods).
type periodTableRow struct {
	displayName string
	period      models.UsagePeriod
}

// buildPeriodTable renders period rows as borderless lipgloss tables.
// Each period is its own single-row table. When a period has detail
// (e.g. dollar amounts), it's placed in the same cell as the percentage
// separated by a newline so alignment is handled by the table.
func buildPeriodTable(rows []periodTableRow) string {
	if len(rows) == 0 {
		return ""
	}

	nameWidth := 0
	for _, r := range rows {
		nameWidth = max(nameWidth, len(r.displayName))
	}

	styleFunc := func(_ int, col int) lipgloss.Style {
		switch col {
		case 0:
			return lipgloss.NewStyle().Width(nameWidth)
		case 2:
			return lipgloss.NewStyle().Align(lipgloss.Right)
		case 3:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		}
		return lipgloss.NewStyle()
	}

	var lines []string
	for _, r := range rows {
		p := r.period
		color := PaceToColor(p.PaceRatio(), p.Utilization)
		pct := colorStyle(color).Render(fmt.Sprintf("%d%%", p.Utilization))
		bar := RenderBar(p.Utilization, 20, color)

		reset := ""
		if d := p.TimeUntilReset(); d != nil {
			reset = "resets in " + FormatResetCountdown(d)
		}

		t := table.New().
			Border(lipgloss.HiddenBorder()).
			StyleFunc(styleFunc).
			Row(r.displayName, bar, pct, reset)
		lines = append(lines, cleanPeriodTableOutput(t.Render()))
	}

	return strings.Join(lines, "\n")
}

// cleanPeriodTableOutput strips the single leading border space and trailing
// whitespace from each line of rendered table output, and removes empty lines.
func cleanPeriodTableOutput(rendered string) string {
	lines := strings.Split(rendered, "\n")
	var cleaned []string
	for _, line := range lines {
		// Remove exactly one leading space (the hidden border left edge).
		line = strings.TrimPrefix(line, " ")
		line = strings.TrimRight(line, " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
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
// When the limit is zero (no hard limit set), shows "Unlimited" to match
// the provider's web UI.
func formatOverageLine(o *models.OverageUsage, label string) string {
	sym := ""
	if o.Currency == "USD" {
		sym = "$"
	}
	if o.Limit > 0 {
		return fmt.Sprintf("%s: %s%.2f / %s%.2f %s", label, sym, o.Used, sym, o.Limit, o.Currency)
	}
	return fmt.Sprintf("%s: %s%.2f %s (Unlimited)", label, sym, o.Used, o.Currency)
}

// formatBalance renders a standalone balance line from billing detail.
// Returns empty string when no balance is available.
func formatBalance(billing *models.BillingDetail) string {
	if billing == nil || billing.Balance == nil {
		return ""
	}
	bal := *billing.Balance
	if bal < 0 {
		return fmt.Sprintf("Balance: -$%.2f", -bal)
	}
	return fmt.Sprintf("Balance: $%.2f", bal)
}

// formatBillingDetail renders a compact sub-line with supplemental billing
// info (reset date, prepaid balance, auto-reload). Returns empty string when
// no billing details are available.
func formatBillingDetail(snapshot models.UsageSnapshot) string {
	var parts []string

	// Reset date from the monthly period (billing cycle reset)
	for _, p := range snapshot.Periods {
		if p.PeriodType == models.PeriodMonthly && p.ResetsAt != nil {
			parts = append(parts, "Resets "+p.ResetsAt.Format("Jan 2"))
			break
		}
	}

	if snapshot.Billing != nil {
		if snapshot.Billing.Balance != nil {
			sym := "$"
			bal := *snapshot.Billing.Balance
			if bal < 0 {
				parts = append(parts, fmt.Sprintf("Balance: -%s%.2f", sym, -bal))
			} else {
				parts = append(parts, fmt.Sprintf("Balance: %s%.2f", sym, bal))
			}
		}
		if snapshot.Billing.AutoReload != nil {
			status := "Off"
			if *snapshot.Billing.AutoReload {
				status = "On"
			}
			parts = append(parts, "Auto-reload: "+status)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return dimStyle.Render("  " + strings.Join(parts, " · "))
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

	// Provider title
	providerTitle := titleStyle.Render(provider.DisplayName(snapshot.Provider))
	if cached {
		providerTitle += dimStyle.Render(" (" + formatAge(time.Since(snapshot.FetchedAt)) + " ago)")
	}
	out.WriteString(providerTitle)
	out.WriteByte('\n')

	// Labeled metadata
	if meta := renderMetaLine(snapshot); meta != "" {
		out.WriteString(meta)
		out.WriteByte('\n')
	}

	// Status line
	if opts.Status != nil {
		out.WriteByte('\n')
		out.WriteString(renderStatusLine(*opts.Status))
		out.WriteByte('\n')
	}

	// Usage panel
	out.WriteByte('\n')
	out.WriteString(renderUsagePanel(snapshot))

	return out.String()
}

// renderMetaLine builds a labeled metadata line from identity and source fields.
func renderMetaLine(snapshot models.UsageSnapshot) string {
	type labeledField struct {
		label string
		value string
	}

	var fields []labeledField

	if snapshot.Identity != nil {
		if snapshot.Identity.Plan != "" {
			fields = append(fields, labeledField{"Plan", snapshot.Identity.Plan})
		}
		if snapshot.Identity.Organization != "" {
			fields = append(fields, labeledField{"Org", snapshot.Identity.Organization})
		}
		if snapshot.Identity.Email != "" {
			fields = append(fields, labeledField{"Account", snapshot.Identity.Email})
		}
	}

	if snapshot.Source != "" {
		fields = append(fields, labeledField{"Auth", formatSourceName(snapshot.Source)})
	}

	if len(fields) == 0 {
		return ""
	}

	maxLabel := 0
	for _, f := range fields {
		maxLabel = max(maxLabel, len(f.label))
	}

	lines := make([]string, len(fields))
	for i, f := range fields {
		pad := strings.Repeat(" ", maxLabel-len(f.label))
		lines[i] = dimStyle.Render(f.label) + pad + "  " + f.value
	}
	return strings.Join(lines, "\n")
}

// formatSourceName returns a human-readable name for a fetch source.
func formatSourceName(source string) string {
	switch source {
	case "oauth":
		return "OAuth"
	case "web":
		return "Web Session"
	case "api_key":
		return "API Key"
	case "device_flow":
		return "Device Flow"
	case "provider_cli":
		return "CLI"
	default:
		return source
	}
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

	// Session periods
	if len(session) > 0 {
		var rows []periodTableRow
		for _, p := range session {
			rows = append(rows, periodTableRow{p.Name, p})
		}
		b.WriteString(buildPeriodTable(rows))
	}

	// Longer periods with per-model breakdowns
	if len(session) > 0 && len(longer.periods) > 0 {
		b.WriteString("\n\n")
	}

	if len(longer.periods) > 0 {
		b.WriteString(longer.header)

		var rows []periodTableRow
		for _, p := range longer.periods {
			name := formatSubPeriodName(&p, longer.header)
			rows = append(rows, periodTableRow{name, p})
		}
		b.WriteByte('\n')
		b.WriteString(buildPeriodTable(rows))
	}

	// Overage and billing details
	if snapshot.Overage != nil && snapshot.Overage.IsEnabled {
		b.WriteString("\n\n")
		b.WriteString(formatOverageLine(snapshot.Overage, "Extra Usage"))
		if detail := formatBillingDetail(snapshot); detail != "" {
			b.WriteByte('\n')
			b.WriteString(detail)
		}
	} else if bal := formatBalance(snapshot.Billing); bal != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(bal)
	}

	return renderTitledPanel(titleStyle.Render("Usage"), b.String(), 0)
}

// PeriodColWidths holds pre-computed column widths used to ensure consistent
// panel sizing across all providers in the dashboard view.
type PeriodColWidths struct{ Name, Pct, Reset int }

// RowWidth returns the total visible width of a fully-populated period row
// as rendered by a borderless lipgloss table. Each column is separated by
// one hidden-border space, plus left and right border spaces.
func (cw PeriodColWidths) RowWidth() int {
	// 4 columns: name, bar(20), pct, reset + 3 separators + 2 outer borders
	return cw.Name + 20 + cw.Pct + cw.Reset + 5
}

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
//
// Aggregate (model-free) periods are preferred. When a provider only returns
// per-model periods (e.g. Gemini), all periods are included so that the
// compact panel is not empty.
func collectDisplayPeriods(snapshot models.UsageSnapshot) []models.UsagePeriod {
	session, weekly, daily, monthly := groupPeriods(snapshot.Periods)

	// First pass: collect only aggregate (non-model) periods.
	var out []models.UsagePeriod
	for _, p := range session {
		if p.Model == "" {
			out = append(out, p)
		}
	}
	for _, p := range weekly {
		if p.Model == "" {
			if isGenericPeriodName(p.Name) {
				p.Name = "Weekly"
			}
			out = append(out, p)
		}
	}
	for _, p := range daily {
		if p.Model == "" {
			if isGenericPeriodName(p.Name) {
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

	// Fallback: if no aggregate periods exist but there are model-specific
	// periods, condense them into model-family summary lines (e.g. "Pro",
	// "Flash") using the highest utilization from each family.
	if len(out) == 0 && len(snapshot.Periods) > 0 {
		out = condenseModelPeriods(snapshot.Periods)
	}

	return out
}

// condenseModelPeriods groups model-specific periods by family (e.g. "pro",
// "flash") and returns one summary period per family using the highest
// utilization. Models that don't match a known family are grouped as "Other".
func condenseModelPeriods(periods []models.UsagePeriod) []models.UsagePeriod {
	type bucket struct {
		best models.UsagePeriod
		set  bool
	}

	// Ordered families to check; first match wins.
	families := []struct {
		label   string
		matches func(string) bool
	}{
		{"Pro", func(m string) bool {
			lower := strings.ToLower(m)
			return strings.Contains(lower, "pro") && !strings.Contains(lower, "flash")
		}},
		{"Flash", func(m string) bool {
			return strings.Contains(strings.ToLower(m), "flash")
		}},
	}

	buckets := make([]bucket, len(families))
	var other bucket

	for _, p := range periods {
		matched := false
		for i, f := range families {
			if f.matches(p.Model) {
				if !buckets[i].set || p.Utilization > buckets[i].best.Utilization {
					buckets[i].best = p
					buckets[i].set = true
				}
				matched = true
				break
			}
		}
		if !matched {
			if !other.set || p.Utilization > other.best.Utilization {
				other.best = p
				other.set = true
			}
		}
	}

	var out []models.UsagePeriod
	for i, b := range buckets {
		if b.set {
			summary := b.best
			summary.Name = families[i].label
			out = append(out, summary)
		}
	}
	if other.set {
		summary := other.best
		summary.Name = "Other"
		out = append(out, summary)
	}
	return out
}

// isGenericPeriodName reports whether a period name looks like a raw/generic
// label that should be normalised to "Daily", "Weekly", etc. in the compact
// panel view. Names with parenthesized qualifiers (e.g. "Monthly (Premium)")
// or provider-branded names (e.g. "Amp Free") are preserved as-is.
func isGenericPeriodName(name string) bool {
	if strings.Contains(name, "(") {
		return false
	}
	lower := strings.ToLower(name)
	for _, generic := range []string{"daily", "weekly", "monthly", "session"} {
		if strings.Contains(lower, generic) {
			return true
		}
	}
	// Names that are purely snake_case or single words look generic
	if !strings.Contains(name, " ") {
		return true
	}
	return false
}

// renderPeriodTable renders a slice of periods as a borderless table,
// using shared column widths for consistent cross-panel alignment.
func renderPeriodTable(periods []models.UsagePeriod, cw PeriodColWidths) string {
	var rows []periodTableRow
	for _, p := range periods {
		rows = append(rows, periodTableRow{p.Name, p})
	}
	return buildPeriodTableWithWidths(rows, cw)
}

// buildPeriodTableWithWidths builds a period table with explicit column widths
// for cross-panel alignment in the dashboard view. Each period is rendered as
// a one-row table so detail sub-lines can be inserted between periods without
// inflating the pct column width for all panels.
func buildPeriodTableWithWidths(rows []periodTableRow, cw PeriodColWidths) string {
	if len(rows) == 0 {
		return ""
	}

	styleFunc := func(_ int, col int) lipgloss.Style {
		switch col {
		case 0:
			return lipgloss.NewStyle().Width(cw.Name)
		case 1:
			return lipgloss.NewStyle().Width(20)
		case 2:
			return lipgloss.NewStyle().Align(lipgloss.Right).Width(cw.Pct)
		case 3:
			return lipgloss.NewStyle().Width(cw.Reset).Foreground(lipgloss.Color("240"))
		}
		return lipgloss.NewStyle()
	}

	var lines []string
	for _, r := range rows {
		p := r.period
		color := PaceToColor(p.PaceRatio(), p.Utilization)
		pct := colorStyle(color).Render(fmt.Sprintf("%d%%", p.Utilization))
		bar := RenderBar(p.Utilization, 20, color)

		reset := ""
		if d := p.TimeUntilReset(); d != nil {
			reset = "resets in " + FormatResetCountdown(d)
		}

		t := table.New().
			Border(lipgloss.HiddenBorder()).
			StyleFunc(styleFunc).
			Row(r.displayName, bar, pct, reset)
		lines = append(lines, cleanPeriodTableOutput(t.Render()))
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
	} else if bal := formatBalance(snapshot.Billing); bal != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(bal)
	}

	title := titleStyle.Render(provider.DisplayName(snapshot.Provider))
	if cached {
		title += dimStyle.Render(" (" + formatAge(time.Since(snapshot.FetchedAt)) + " ago)")
	}
	return renderTitledPanel(title, b.String(), cw.RowWidth())
}

func renderTitledPanel(title string, body string, minWidth int) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	bodyWidth := minWidth
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
