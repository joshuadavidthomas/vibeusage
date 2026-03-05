package pace

import (
	"math"
	"testing"
)

func pf(v float64) *float64 { return &v }

func TestAssess(t *testing.T) {
	tests := []struct {
		name         string
		paceRatio    *float64
		utilization  int
		elapsedRatio *float64
		want         Level
	}{
		// nil pace ratio — fall back to utilization thresholds
		{"nil pace, low util", nil, 20, nil, OK},
		{"nil pace, mid util", nil, 50, nil, Warning},
		{"nil pace, high util", nil, 79, nil, Warning},
		{"nil pace, very high util", nil, 80, nil, Critical},
		{"nil pace, full util", nil, 100, nil, Critical},

		// with pace ratio, no elapsed (headroom check skipped)
		{"pace 0.5 ok", pf(0.5), 25, nil, OK},
		{"pace 1.0 ok", pf(1.0), 50, nil, OK},
		{"pace 1.15 ok boundary", pf(1.15), 60, nil, OK},
		{"pace 1.16 warning", pf(1.16), 60, nil, Warning},
		{"pace 1.30 warning boundary", pf(1.30), 70, nil, Warning},
		{"pace 1.31 critical", pf(1.31), 70, nil, Critical},
		{"pace 2.0 critical", pf(2.0), 80, nil, Critical},

		// exhausted quota — always critical regardless of pace
		{"pace 1.0 util 100 critical", pf(1.0), 100, nil, Critical},
		{"pace 1.05 util 100 critical", pf(1.05), 100, nil, Critical},
		{"pace 0.5 util 100 critical", pf(0.5), 100, nil, Critical},

		// near-exhaustion floor — pace can escalate but not rescue
		{"pace 1.0 util 90 warning no elapsed", pf(1.0), 90, nil, Warning},
		{"pace 0.5 util 95 warning no elapsed", pf(0.5), 95, nil, Warning},
		{"pace 1.16 util 92 critical", pf(1.16), 92, nil, Critical},
		{"pace 2.0 util 90 critical", pf(2.0), 90, nil, Critical},

		// headroom stress — remaining budget dangerously low for remaining time

		// Issue #144: 99% util with significant time remaining → critical
		{"99% util, 86.3% elapsed, pace 1.147", pf(1.147), 99, pf(0.863), Critical},
		{"99% util, 91.7% elapsed, pace 1.080", pf(1.080), 99, pf(0.917), Critical},

		// 95% util, 80% elapsed → headroom 5/20=0.25, < 0.50 and util >= 80 → critical
		{"95% util, 80% elapsed", pf(1.1875), 95, pf(0.80), Critical},

		// 90% util, 80% elapsed → headroom 10/20=0.50, not triggered → warning (90% floor)
		{"90% util, 80% elapsed", pf(1.125), 90, pf(0.80), Warning},

		// 85% util, 50% elapsed → headroom 15/50=0.30, < 0.50 and util >= 80 → critical
		{"85% util, 50% elapsed", pf(1.70), 85, pf(0.50), Critical},

		// 80% util, 75% elapsed → headroom 20/25=0.80 → defers to pace 1.067 → ok
		{"80% util, 75% elapsed, pace on track", pf(1.067), 80, pf(0.75), OK},

		// 50% util, 40% elapsed → headroom 50/60=0.83 → defers to pace 1.25 → warning
		{"50% util, 40% elapsed, pace 1.25", pf(1.25), 50, pf(0.40), Warning},

		// 30% util, 20% elapsed → headroom 70/80=0.875 → defers to pace 1.5 → critical
		{"30% util, 20% elapsed, pace 1.5", pf(1.5), 30, pf(0.20), Critical},

		// 10% util, 90% elapsed → headroom 90/10=9.0 → defers to pace 0.11 → ok
		{"10% util, 90% elapsed, well under pace", pf(0.11), 10, pf(0.90), OK},

		// near period end: elapsed=1.0 → headroom skipped → 90% floor → warning
		{"99% util, period ending", pf(1.0), 99, pf(1.0), Warning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Assess(tt.paceRatio, tt.utilization, tt.elapsedRatio)
			if got != tt.want {
				t.Errorf("Assess(%v, %d, %v) = %v, want %v", tt.paceRatio, tt.utilization, tt.elapsedRatio, got, tt.want)
			}
		})
	}
}

func TestLevel_Color(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{OK, "green"},
		{Warning, "yellow"},
		{Critical, "red"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.Color(); got != tt.want {
				t.Errorf("%v.Color() = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{OK, "ok"},
		{Warning, "warning"},
		{Critical, "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("%v.String() = %q, want %q", tt.level, got, tt.want)
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
