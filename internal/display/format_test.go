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

func TestPaceToColor(t *testing.T) {
	pf := func(v float64) *float64 { return &v }

	tests := []struct {
		name        string
		paceRatio   *float64
		utilization int
		want        string
	}{
		// nil pace ratio - fall back to utilization thresholds
		{"nil pace, low util", nil, 20, "green"},
		{"nil pace, mid util", nil, 50, "yellow"},
		{"nil pace, high util", nil, 79, "yellow"},
		{"nil pace, very high util", nil, 80, "red"},
		{"nil pace, full util", nil, 100, "red"},

		// with pace ratio
		{"pace 0.5 green", pf(0.5), 25, "green"},
		{"pace 1.0 green", pf(1.0), 50, "green"},
		{"pace 1.15 green boundary", pf(1.15), 60, "green"},
		{"pace 1.16 yellow", pf(1.16), 60, "yellow"},
		{"pace 1.30 yellow boundary", pf(1.30), 70, "yellow"},
		{"pace 1.31 red", pf(1.31), 70, "red"},
		{"pace 2.0 red", pf(2.0), 80, "red"},

		// exhausted quota — always red regardless of pace
		// at 100% you're blocked; "on pace" is irrelevant
		{"pace 1.0 util 100 red", pf(1.0), 100, "red"},
		{"pace 1.05 util 100 red", pf(1.05), 100, "red"},
		{"pace 0.5 util 100 red", pf(0.5), 100, "red"},

		// near-exhaustion (≥90%) floor — pace can escalate to red but not rescue to green
		// e.g. 99% at 86% elapsed = paceRatio 1.149 (just under 1.15), must be yellow not green
		{"pace 1.0 util 90 yellow", pf(1.0), 90, "yellow"},
		{"pace 1.0 util 99 yellow", pf(1.0), 99, "yellow"},
		{"pace 0.5 util 95 yellow", pf(0.5), 95, "yellow"},
		{"pace 1.149 util 99 yellow", pf(1.149), 99, "yellow"},
		{"pace 1.16 util 92 red", pf(1.16), 92, "red"},
		{"pace 2.0 util 90 red", pf(2.0), 90, "red"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaceToColor(tt.paceRatio, tt.utilization)
			if got != tt.want {
				t.Errorf("PaceToColor(%v, %d) = %q, want %q", tt.paceRatio, tt.utilization, got, tt.want)
			}
		})
	}
}
