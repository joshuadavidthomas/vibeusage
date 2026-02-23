package routing

import "fmt"

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
