package pace

import (
	"math"
	"testing"
)

func pf(v float64) *float64 { return &v }

func TestColor(t *testing.T) {
	tests := []struct {
		name         string
		paceRatio    *float64
		utilization  int
		elapsedRatio *float64
		want         string
	}{
		// nil pace ratio — fall back to utilization thresholds
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
		{"pace 1.0 util 100 red", pf(1.0), 100, nil, "red"},
		{"pace 1.05 util 100 red", pf(1.05), 100, nil, "red"},
		{"pace 0.5 util 100 red", pf(0.5), 100, nil, "red"},

		// near-exhaustion (≥90%) floor — pace can escalate to red but not rescue to green
		{"pace 1.0 util 90 yellow no elapsed", pf(1.0), 90, nil, "yellow"},
		{"pace 0.5 util 95 yellow no elapsed", pf(0.5), 95, nil, "yellow"},
		{"pace 1.16 util 92 red", pf(1.16), 92, nil, "red"},
		{"pace 2.0 util 90 red", pf(2.0), 90, nil, "red"},

		// headroom stress — remaining budget dangerously low relative to remaining time

		// Issue #144 examples: 99% util with significant time remaining → red
		// 99% util, 86.3% elapsed → headroom ratio = 1/13.7 = 0.073 → red
		{"99% util, 86.3% elapsed, pace 1.147", pf(1.147), 99, pf(0.863), "red"},
		// 99% util, 91.7% elapsed → headroom ratio = 1/8.3 = 0.12 → red
		{"99% util, 91.7% elapsed, pace 1.080", pf(1.080), 99, pf(0.917), "red"},

		// 95% util, 80% elapsed → headroom = 5/20 = 0.25 → not < 0.25,
		// but < 0.50 and util >= 80 → red
		{"95% util, 80% elapsed", pf(1.1875), 95, pf(0.80), "red"},

		// 90% util, 80% elapsed → headroom = 10/20 = 0.50 → not triggered,
		// falls through to 90% floor → yellow
		{"90% util, 80% elapsed", pf(1.125), 90, pf(0.80), "yellow"},

		// 85% util, 50% elapsed → headroom = 15/50 = 0.30 → < 0.50 and util >= 80 → red
		{"85% util, 50% elapsed", pf(1.70), 85, pf(0.50), "red"},

		// 80% util, 75% elapsed → headroom = 20/25 = 0.80 → no trigger,
		// falls through to pace check: pace 1.067 ≤ 1.15 → green
		{"80% util, 75% elapsed, pace on track", pf(1.067), 80, pf(0.75), "green"},

		// 50% util, 40% elapsed → headroom = 50/60 = 0.83 → no trigger,
		// pace = 1.25, ≤ 1.30 → yellow
		{"50% util, 40% elapsed, pace 1.25", pf(1.25), 50, pf(0.40), "yellow"},

		// 30% util, 20% elapsed → headroom = 70/80 = 0.875 → no trigger,
		// pace = 1.5, > 1.30 → red
		{"30% util, 20% elapsed, pace 1.5", pf(1.5), 30, pf(0.20), "red"},

		// 10% util, 90% elapsed → headroom = 90/10 = 9.0 → well above threshold,
		// pace = 0.11, ≤ 1.15 → green
		{"10% util, 90% elapsed, well under pace", pf(0.11), 10, pf(0.90), "green"},

		// near period end: elapsed = 1.0 → headroom check skipped (period resetting),
		// falls through to 90% floor → yellow
		{"99% util, period ending", pf(1.0), 99, pf(1.0), "yellow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Color(tt.paceRatio, tt.utilization, tt.elapsedRatio)
			if got != tt.want {
				t.Errorf("Color(%v, %d, %v) = %q, want %q", tt.paceRatio, tt.utilization, tt.elapsedRatio, got, tt.want)
			}
		})
	}
}

func TestHeadroomRatio(t *testing.T) {
	tests := []struct {
		name         string
		utilization  int
		elapsedRatio float64
		want         float64
	}{
		{"on track", 50, 0.50, 1.0},
		{"under pace", 25, 0.50, 1.5},
		{"over pace", 75, 0.50, 0.5},
		{"critical — 99% at 86%", 99, 0.863, 0.073},
		{"lots of headroom early", 10, 0.10, 1.0},
		{"zero remaining time, zero budget", 100, 1.0, 0},
		{"zero remaining time, some budget", 90, 1.0, math.Inf(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HeadroomRatio(tt.utilization, tt.elapsedRatio)
			if math.IsInf(tt.want, 1) {
				if !math.IsInf(got, 1) {
					t.Errorf("HeadroomRatio(%d, %f) = %f, want +Inf", tt.utilization, tt.elapsedRatio, got)
				}
				return
			}
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("HeadroomRatio(%d, %f) = %f, want ~%f", tt.utilization, tt.elapsedRatio, got, tt.want)
			}
		})
	}
}

func TestEffectiveHeadroom(t *testing.T) {
	tests := []struct {
		name       string
		headroom   int
		multiplier *float64
		want       int
	}{
		{"nil multiplier", 80, nil, 80},
		{"zero multiplier (free)", 50, pf(0), 100},
		{"1x multiplier", 90, pf(1), 90},
		{"3x multiplier", 90, pf(3), 30},
		{"0.33x multiplier", 30, pf(0.33), 90},
		{"0.33x capped at 100", 50, pf(0.33), 100},
		{"zero headroom", 0, pf(3), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveHeadroom(tt.headroom, tt.multiplier)
			if got != tt.want {
				t.Errorf("EffectiveHeadroom(%d, %v) = %d, want %d", tt.headroom, tt.multiplier, got, tt.want)
			}
		})
	}
}
