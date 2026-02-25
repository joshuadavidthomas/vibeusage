package display

import (
	"strconv"
	"time"
)

// FormatResetCountdown formats a duration as a compact human-readable
// countdown string (e.g. "2d 3h", "5h 42m", "15m").
func FormatResetCountdown(d *time.Duration) string {
	if d == nil {
		return ""
	}
	total := int(d.Seconds())
	if total <= 0 {
		return "now"
	}
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	if days > 0 {
		return formatDH(days, hours)
	}
	if hours > 0 {
		return formatHM(hours, minutes)
	}
	return formatM(minutes)
}

func formatDH(d, h int) string { return strconv.Itoa(d) + "d " + strconv.Itoa(h) + "h" }
func formatHM(h, m int) string { return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m" }
func formatM(m int) string     { return strconv.Itoa(m) + "m" }

// PaceToColor returns a color name ("green", "yellow", or "red") based on
// the pace ratio and current utilization percentage.
func PaceToColor(paceRatio *float64, utilization int) string {
	// Exhausted quota is always red — you're blocked regardless of pace.
	if utilization >= 100 {
		return "red"
	}
	if paceRatio == nil {
		if utilization < 50 {
			return "green"
		}
		if utilization < 80 {
			return "yellow"
		}
		return "red"
	}
	// Near-exhaustion floor: ≥90% utilization is always at least yellow.
	// Pace ratio captures burn rate, not how much budget remains. At 90%+
	// you have ≤10% left regardless of pace, which warrants a caution signal.
	// Pace can still escalate near-exhaustion to red, but not rescue it to green.
	if utilization >= 90 {
		if *paceRatio > 1.15 {
			return "red"
		}
		return "yellow"
	}
	if *paceRatio <= 1.15 {
		return "green"
	}
	if *paceRatio <= 1.30 {
		return "yellow"
	}
	return "red"
}
