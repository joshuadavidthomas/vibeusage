package display

import (
	"fmt"
	"sort"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/routing"
)

// RenderBarFunc renders a utilization bar for the given percentage.
type RenderBarFunc func(utilization int) string

// FormatResetFunc formats a duration until reset for display.
type FormatResetFunc func(d *time.Duration) string

// FormattedTable holds the pre-formatted rows, headers, and styles
// for a routing recommendation table.
type FormattedTable struct {
	Headers []string
	Rows    [][]string
	Styles  []RowStyle
}

// FormatRecommendationRows builds formatted table rows for a single-model
// recommendation. The renderBar and formatReset callbacks allow the caller to
// inject presentation-layer rendering.
func FormatRecommendationRows(rec routing.Recommendation, renderBar RenderBarFunc, formatReset FormatResetFunc) FormattedTable {
	hasMultiplier := false
	for _, c := range rec.Candidates {
		if c.Multiplier != nil {
			hasMultiplier = true
			break
		}
	}

	var rows [][]string
	var styles []RowStyle

	for i, c := range rec.Candidates {
		row := formatCandidateRow(c, hasMultiplier, false, "", renderBar, formatReset)
		rows = append(rows, row)

		if i == 0 {
			styles = append(styles, RowBold)
		} else {
			styles = append(styles, RowNormal)
		}
	}

	// Unavailable providers as dim rows.
	sort.Strings(rec.Unavailable)
	for _, pid := range rec.Unavailable {
		row := unavailableRow(provider.DisplayName(pid), "", hasMultiplier, false)
		rows = append(rows, row)
		styles = append(styles, RowDim)
	}

	headers := buildRouteHeaders(hasMultiplier, false)

	return FormattedTable{Headers: headers, Rows: rows, Styles: styles}
}

// FormatRoleRecommendationRows builds formatted table rows for a role-based
// recommendation, which includes a "Model" column.
func FormatRoleRecommendationRows(rec routing.RoleRecommendation, renderBar RenderBarFunc, formatReset FormatResetFunc) FormattedTable {
	hasMultiplier := false
	for _, c := range rec.Candidates {
		if c.Multiplier != nil {
			hasMultiplier = true
			break
		}
	}

	var rows [][]string
	var styles []RowStyle

	for i, c := range rec.Candidates {
		row := formatCandidateRow(c.Candidate, hasMultiplier, true, c.ModelID, renderBar, formatReset)
		rows = append(rows, row)

		if i == 0 {
			styles = append(styles, RowBold)
		} else {
			styles = append(styles, RowNormal)
		}
	}

	// Unavailable as dim rows.
	for _, u := range rec.Unavailable {
		row := unavailableRow(provider.DisplayName(u.ProviderID), u.ModelID, hasMultiplier, true)
		rows = append(rows, row)
		styles = append(styles, RowDim)
	}

	headers := buildRouteHeaders(hasMultiplier, true)

	return FormattedTable{Headers: headers, Rows: rows, Styles: styles}
}

// FormatMultiplier formats a cost multiplier for display.
// nil → "—", 0 → "free", integer → "3x", fractional → "0.33x".
func FormatMultiplier(m *float64) string {
	if m == nil {
		return "—"
	}
	v := *m
	if v == 0 {
		return "free"
	}
	if v == float64(int(v)) {
		return fmt.Sprintf("%dx", int(v))
	}
	return fmt.Sprintf("%.2gx", v)
}

func formatCandidateRow(c routing.Candidate, hasMultiplier, includeModel bool, modelID string, renderBar RenderBarFunc, formatReset FormatResetFunc) []string {
	name := provider.DisplayName(c.ProviderID)
	bar := renderBar(c.Utilization)
	util := fmt.Sprintf("%d%%", c.Utilization)
	headroom := fmt.Sprintf("%d%%", c.EffectiveHeadroom)
	cost := FormatMultiplier(c.Multiplier)

	reset := ""
	if c.ResetsAt != nil {
		d := time.Until(*c.ResetsAt)
		if d < 0 {
			d = 0
		}
		reset = formatReset(&d)
	}

	plan := c.Plan
	if plan == "" {
		plan = "—"
	}

	var row []string
	if includeModel {
		row = append(row, modelID)
	}
	row = append(row, name, bar+" "+util, headroom)
	if hasMultiplier {
		row = append(row, cost)
	}
	row = append(row, string(c.PeriodType), reset, plan)

	return row
}

func unavailableRow(providerName, modelID string, hasMultiplier, includeModel bool) []string {
	var row []string
	if includeModel {
		row = append(row, modelID)
	}
	row = append(row, providerName, "—", "—")
	if hasMultiplier {
		row = append(row, "—")
	}
	row = append(row, "—", "—", "—")
	return row
}

func buildRouteHeaders(hasMultiplier, includeModel bool) []string {
	var headers []string
	if includeModel {
		headers = append(headers, "Model")
	}
	headers = append(headers, "Provider", "Usage", "Headroom")
	if hasMultiplier {
		headers = append(headers, "Cost")
	}
	headers = append(headers, "Period", "Resets In", "Plan")
	return headers
}
