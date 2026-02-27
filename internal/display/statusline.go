package display

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

// StatuslineMode determines the output format for statusline display.
type StatuslineMode string

const (
	StatuslineModePretty StatuslineMode = "pretty"
	StatuslineModeShort  StatuslineMode = "short"
	StatuslineModeJSON   StatuslineMode = "json"
)

// StatuslineOptions configures statusline rendering.
type StatuslineOptions struct {
	Mode    StatuslineMode
	Limit   int
	NoColor bool
}

// RenderStatusline outputs statusline-formatted data for the given outcomes.
func RenderStatusline(w io.Writer, outcomes map[string]fetch.FetchOutcome, opts StatuslineOptions) error {
	if opts.Mode == StatuslineModeJSON {
		return renderStatuslineJSON(w, outcomes)
	}
	showBar := opts.Mode == StatuslineModePretty
	showProviderLabel := len(outcomes) != 1
	return renderStatuslineTable(w, outcomes, showProviderLabel, showBar, opts.NoColor, opts.Limit)
}

// periodColumn holds the pre-rendered data for one period in a statusline row.
type periodColumn struct {
	qualifier string
	duration  string
	bar       string // empty when showBar is false
	pct       string
	timer     string
}

// renderStatuslineTable renders a compact table for both pretty and short modes.
// When showBar is true, a visual bar column is included between duration and pct.
func renderStatuslineTable(w io.Writer, outcomes map[string]fetch.FetchOutcome, showProviderLabel, showBar, noColor bool, limit int) error {
	ids := sortedOutcomeIDs(outcomes)

	type row struct {
		provider string
		periods  []periodColumn
	}
	var rows []row
	maxProviderWidth := 0

	for _, pid := range ids {
		outcome := outcomes[pid]
		if !outcome.Success || outcome.Snapshot == nil {
			continue
		}

		r := row{provider: provider.DisplayName(pid)}
		if len(r.provider) > maxProviderWidth {
			maxProviderWidth = len(r.provider)
		}

		periods := statuslinePeriods(*outcome.Snapshot)
		if limit > 0 && len(periods) > limit {
			periods = periods[:limit]
		}

		for _, p := range periods {
			utilization := min(p.Utilization, 100)
			color := PaceToColor(p.PaceRatio(), p.Utilization)
			qual, dur := periodNameParts(p)
			timer := formatDurationCompact(p.TimeUntilReset())
			if timer == "" {
				timer = "-"
			}

			col := periodColumn{
				qualifier: qual,
				duration:  dur,
				pct:       fmt.Sprintf("%d%%", p.Utilization),
				timer:     timer,
			}

			if showBar {
				filled := utilization * 10 / 100
				bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
				col.bar = colorStyle(color).Render(bar)
				col.pct = colorStyle(color).Render(col.pct)
			} else if !noColor {
				col.pct = colorStyle(color).Render(col.pct)
			}

			r.periods = append(r.periods, col)
		}

		rows = append(rows, r)
	}

	// Determine column layout
	hasQualifier := false
	maxPeriods := 1
	for _, r := range rows {
		if len(r.periods) > maxPeriods {
			maxPeriods = len(r.periods)
		}
		for _, p := range r.periods {
			if p.qualifier != "" {
				hasQualifier = true
			}
		}
	}

	colsPerPeriod := countPeriodCols(showBar, hasQualifier)
	providerCols := 0
	if showProviderLabel {
		providerCols = 1
	}
	totalCols := providerCols + maxPeriods*colsPerPeriod

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(statuslineStyleFunc(showProviderLabel, hasQualifier, showBar, providerCols, colsPerPeriod, maxProviderWidth))

	for _, r := range rows {
		cells := make([]string, totalCols)
		if showProviderLabel {
			cells[0] = r.provider
		}
		for i, p := range r.periods {
			if i >= maxPeriods {
				break
			}
			fillPeriodCells(cells, providerCols+i*colsPerPeriod, p, hasQualifier, showBar)
		}
		t.Row(cells...)
	}

	rendered := t.Render()
	_, err := fmt.Fprintln(w, cleanTableOutput(rendered, !showProviderLabel))
	return err
}

// statuslineStyleFunc returns a StyleFunc for the statusline table.
func statuslineStyleFunc(showProviderLabel, hasQualifier, showBar bool, providerCols, colsPerPeriod, maxProviderWidth int) func(int, int) lipgloss.Style {
	return func(_, col int) lipgloss.Style {
		if showProviderLabel && col == 0 {
			return lipgloss.NewStyle().Bold(true).Align(lipgloss.Right).Width(maxProviderWidth)
		}
		periodCol := (col - providerCols) % colsPerPeriod
		return periodCellStyle(periodCol, hasQualifier, showBar)
	}
}

// periodCellStyle returns the lipgloss style for a given sub-column within a period group.
func periodCellStyle(periodCol int, hasQualifier, showBar bool) lipgloss.Style {
	// Normalize to a canonical field index regardless of which optional columns are present.
	// Fields in order: [qualifier?] [duration] [bar?] [pct] [timer]
	field := periodCol
	if !hasQualifier {
		field++ // skip qualifier slot
	}
	if !showBar && field >= 2 {
		field++ // skip bar slot
	}

	switch field {
	case 0: // qualifier
		return lipgloss.NewStyle().Align(lipgloss.Right).Foreground(lipgloss.Color("240"))
	case 1: // duration
		return lipgloss.NewStyle().Align(lipgloss.Right).Foreground(lipgloss.Color("245"))
	case 2: // bar
		return lipgloss.NewStyle().Align(lipgloss.Left)
	case 3: // pct
		return lipgloss.NewStyle().Align(lipgloss.Right)
	case 4: // timer
		return lipgloss.NewStyle().Align(lipgloss.Right).Foreground(lipgloss.Color("240"))
	}
	return lipgloss.NewStyle()
}

// countPeriodCols returns the number of table columns per period.
func countPeriodCols(showBar, hasQualifier bool) int {
	n := 3 // duration, pct, timer
	if showBar {
		n++
	}
	if hasQualifier {
		n++
	}
	return n
}

// fillPeriodCells populates the table cells for one period starting at base.
func fillPeriodCells(cells []string, base int, p periodColumn, hasQualifier, showBar bool) {
	i := base
	if hasQualifier {
		cells[i] = p.qualifier
		i++
	}
	cells[i] = p.duration
	i++
	if showBar {
		cells[i] = p.bar
		i++
	}
	cells[i] = p.pct
	i++
	cells[i] = p.timer
}

// renderStatuslineJSON renders machine-readable JSON.
func renderStatuslineJSON(w io.Writer, outcomes map[string]fetch.FetchOutcome) error {
	entries := make([]StatuslineJSON, 0, len(outcomes))

	for _, pid := range sortedOutcomeIDs(outcomes) {
		outcome := outcomes[pid]
		entry := StatuslineJSON{Provider: pid}

		if !outcome.Success || outcome.Snapshot == nil {
			entry.Error = outcome.Error
			if entry.Error == "" {
				entry.Error = "unavailable"
			}
		} else {
			snap := *outcome.Snapshot
			for _, p := range statuslinePeriods(snap) {
				entry.Periods = append(entry.Periods, StatuslinePeriodJSON{
					Name:        p.Name,
					Utilization: p.Utilization,
					PeriodType:  string(p.PeriodType),
				})
			}

			if snap.Overage != nil && snap.Overage.IsEnabled {
				o := snap.Overage
				entry.Overage = &StatuslineOverageJSON{
					Used:        o.Used,
					Limit:       o.Limit,
					Currency:    o.Currency,
					Utilization: o.UtilizationPct(),
				}
			}
		}

		entries = append(entries, entry)
	}

	return OutputJSON(w, entries)
}

// statuslinePeriods extracts the most relevant periods for statusline display.
// Picks the highest-utilization period per type (skipping model-specific ones),
// and returns them in urgency order (shortest window first).
func statuslinePeriods(snap models.UsageSnapshot) []models.UsagePeriod {
	best := make(map[models.PeriodType]*models.UsagePeriod)

	for _, p := range snap.Periods {
		if p.Model != "" {
			continue
		}
		curr, ok := best[p.PeriodType]
		if !ok || p.Utilization > curr.Utilization {
			cp := p
			best[p.PeriodType] = &cp
		}
	}

	order := []models.PeriodType{
		models.PeriodSession,
		models.PeriodDaily,
		models.PeriodWeekly,
		models.PeriodMonthly,
	}
	var result []models.UsagePeriod
	for _, pt := range order {
		if p, ok := best[pt]; ok {
			result = append(result, *p)
		}
	}
	return result
}

// sortedOutcomeIDs returns sorted provider IDs from an outcomes map.
func sortedOutcomeIDs(outcomes map[string]fetch.FetchOutcome) []string {
	ids := make([]string, 0, len(outcomes))
	for id := range outcomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// periodDurationLabel returns a compact duration label for a period type.
func periodDurationLabel(pt models.PeriodType) string {
	switch pt {
	case models.PeriodSession:
		return "5h"
	case models.PeriodDaily:
		return "24h"
	case models.PeriodWeekly:
		return "7d"
	case models.PeriodMonthly:
		return "30d"
	default:
		return strings.ToLower(string(pt))
	}
}

// periodNameParts returns the qualifier and duration label for a period.
// The qualifier is non-empty only when the name has a parenthesized
// distinction (e.g., "Monthly (Premium)" → "Prem", "30d").
func periodNameParts(p models.UsagePeriod) (qualifier, duration string) {
	duration = periodDurationLabel(p.PeriodType)

	if i := strings.Index(p.Name, "("); i >= 0 {
		if j := strings.Index(p.Name[i:], ")"); j >= 0 {
			qual := strings.TrimSpace(p.Name[i+1 : i+j])
			if qual != "5h" {
				if len(qual) > 4 {
					qual = qual[:4]
				}
				qualifier = qual
			}
		}
	}

	return
}

// formatDurationCompact formats a duration in compact form (e.g., "7h", "6d9h").
func formatDurationCompact(d *time.Duration) string {
	if d == nil {
		return ""
	}

	hours := int(d.Hours())
	days := hours / 24
	remainingHours := hours % 24

	if days > 0 {
		if remainingHours > 0 {
			return fmt.Sprintf("%dd%dh", days, remainingHours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return "<1m"
}

// cleanTableOutput strips blank lines and trailing spaces from rendered table
// output. When trimLeft is true, it also strips leading whitespace per line
// (used when there's no provider column to preserve alignment for).
func cleanTableOutput(rendered string, trimLeft bool) string {
	lines := strings.Split(rendered, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if trimLeft {
			trimmed = strings.TrimLeft(trimmed, " ")
		}
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}
