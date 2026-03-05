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
// the pace ratio, current utilization percentage, and elapsed time in the
// period. The elapsedRatio (0.0–1.0) enables headroom-aware color decisions:
// pace ratio alone can mask dangerously low remaining budget when utilization
// is high (e.g. 99% at 86% elapsed has pace ~1.15 but only 1% headroom).
func PaceToColor(paceRatio *float64, utilization int, elapsedRatio *float64) string {
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
	// Headroom check: evaluate remaining budget relative to remaining time.
	// Pace ratio captures burn rate but can mask how little budget actually
	// remains. Example: 99% utilization at 86% elapsed → pace 1.15 looks
	// borderline, but only 1% budget for 14% of the period is clearly
	// unsustainable.
	//
	// headroom = remaining_budget% / remaining_time%. A value of 1.0 means
	// budget and time are proportional; below 1.0 means you're running out
	// of budget faster than time.
	if elapsedRatio != nil && *elapsedRatio < 1.0 {
		remainingBudget := float64(100 - utilization)
		remainingTime := (1.0 - *elapsedRatio) * 100.0
		headroom := remainingBudget / remainingTime
		if headroom < 0.25 {
			return "red"
		}
		if headroom < 0.50 && utilization >= 80 {
			return "red"
		}
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
