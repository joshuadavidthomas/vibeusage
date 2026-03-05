package display

import (
	"testing"
	"time"
)

func TestFormatResetCountdown(t *testing.T) {
	dur := func(d time.Duration) *time.Duration { return &d }

	tests := []struct {
		name string
		d    *time.Duration
		want string
	}{
		{"nil duration", nil, ""},
		{"zero", dur(0), "now"},
		{"negative", dur(-5 * time.Minute), "now"},
		{"30 minutes", dur(30 * time.Minute), "30m"},
		{"1 minute", dur(1 * time.Minute), "1m"},
		{"0 minutes (59s)", dur(59 * time.Second), "0m"},
		{"2 hours 15 min", dur(2*time.Hour + 15*time.Minute), "2h 15m"},
		{"1 hour 0 min", dur(1 * time.Hour), "1h 0m"},
		{"1 day 3 hours", dur(27 * time.Hour), "1d 3h"},
		{"2 days 0 hours", dur(48 * time.Hour), "2d 0h"},
		{"7 days 12 hours", dur(180 * time.Hour), "7d 12h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResetCountdown(tt.d)
			if got != tt.want {
				t.Errorf("FormatResetCountdown(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
