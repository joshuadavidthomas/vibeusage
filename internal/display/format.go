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
