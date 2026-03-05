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
		name         string
		paceRatio    *float64
		utilization  int
		elapsedRatio *float64
		want         string
	}{
		// nil pace ratio - fall back to utilization thresholds
		{"nil pace, low util", nil, 20, nil, "green"},
		{"nil pace, mid util", nil, 50, nil, "yellow"},
		{"nil pace, high util", nil, 79, nil, "yellow"},
		{"nil pace, very high util", nil, 80, nil, "red"},
		{"nil pace, full util", nil, 100, nil, "red"},

		// with pace ratio, no elapsed (headroom check skipped)
		{"pace 0.5 green", pf(0.5), 25, nil, "green"},
		{"pace 1.0 green", pf(1.0), 50, nil, "green"},
		{"pace 1.15 green boundary", pf(1.15), 60, nil, "green"},
		{"pace 1.16 yellow", pf(1.16), 60, nil, "yellow"},
		{"pace 1.30 yellow boundary", pf(1.30), 70, nil, "yellow"},
		{"pace 1.31 red", pf(1.31), 70, nil, "red"},
		{"pace 2.0 red", pf(2.0), 80, nil, "red"},

		// exhausted quota — always red regardless of pace
		// at 100% you're blocked; "on pace" is irrelevant
		{"pace 1.0 util 100 red", pf(1.0), 100, nil, "red"},
		{"pace 1.05 util 100 red", pf(1.05), 100, nil, "red"},
		{"pace 0.5 util 100 red", pf(0.5), 100, nil, "red"},

		// near-exhaustion (≥90%) floor — pace can escalate to red but not rescue to green
		// e.g. 99% at 86% elapsed = paceRatio 1.149 (just under 1.15), must be yellow not green
		{"pace 1.0 util 90 yellow no elapsed", pf(1.0), 90, nil, "yellow"},
		{"pace 0.5 util 95 yellow no elapsed", pf(0.5), 95, nil, "yellow"},
		{"pace 1.16 util 92 red", pf(1.16), 92, nil, "red"},
		{"pace 2.0 util 90 red", pf(2.0), 90, nil, "red"},

		// headroom stress — remaining budget dangerously low relative to remaining time
		// adequacy = remaining_budget% / remaining_time%
		// < 0.25 → red regardless of pace

		// Issue #144 examples: 99% util with significant time remaining → red
		// 99% util, 86.3% elapsed → adequacy = 1/13.7 = 0.073 → red
		{"99% util, 86.3% elapsed, pace 1.147", pf(1.147), 99, pf(0.863), "red"},
		// 99% util, 91.7% elapsed → adequacy = 1/8.3 = 0.12 → red
		{"99% util, 91.7% elapsed, pace 1.080", pf(1.080), 99, pf(0.917), "red"},

		// 95% util, 80% elapsed → adequacy = 5/20 = 0.25 → not < 0.25,
		// but < 0.50 and util >= 80 → red
		{"95% util, 80% elapsed", pf(1.1875), 95, pf(0.80), "red"},

		// 90% util, 80% elapsed → adequacy = 10/20 = 0.50 → not triggered,
		// falls through to 90% floor → yellow
		{"90% util, 80% elapsed", pf(1.125), 90, pf(0.80), "yellow"},

		// 85% util, 50% elapsed → adequacy = 15/50 = 0.30 → < 0.50 and util >= 80 → red
		{"85% util, 50% elapsed", pf(1.70), 85, pf(0.50), "red"},

		// 80% util, 75% elapsed → adequacy = 20/25 = 0.80 → no headroom trigger,
		// falls through to pace check: pace 1.067 ≤ 1.15 → green
		{"80% util, 75% elapsed, pace on track", pf(1.067), 80, pf(0.75), "green"},

		// 50% util, 40% elapsed → adequacy = 50/60 = 0.83 → no trigger,
		// pace = 1.25, ≤ 1.30 → yellow
		{"50% util, 40% elapsed, pace 1.25", pf(1.25), 50, pf(0.40), "yellow"},

		// 30% util, 20% elapsed → adequacy = 70/80 = 0.875 → no trigger,
		// pace = 1.5, > 1.30 → red
		{"30% util, 20% elapsed, pace 1.5", pf(1.5), 30, pf(0.20), "red"},

		// 10% util, 90% elapsed → adequacy = 90/10 = 9.0 → well above threshold,
		// pace = 0.11, ≤ 1.15 → green
		{"10% util, 90% elapsed, well under pace", pf(0.11), 10, pf(0.90), "green"},

		// near period end: elapsed = 1.0 → headroom check skipped (period resetting),
		// falls through to 90% floor → yellow
		{"99% util, period ending", pf(1.0), 99, pf(1.0), "yellow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaceToColor(tt.paceRatio, tt.utilization, tt.elapsedRatio)
			if got != tt.want {
				t.Errorf("PaceToColor(%v, %d, %v) = %q, want %q", tt.paceRatio, tt.utilization, tt.elapsedRatio, got, tt.want)
			}
		})
	}
}
