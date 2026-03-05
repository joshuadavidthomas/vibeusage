package pace

import "math"

// Level represents the health of a usage budget — how urgently the user
// should be aware of their consumption.
type Level int

const (
	OK       Level = iota // on track, no concern
	Warning               // elevated usage, worth watching
	Critical              // at or near limit, action needed
)

func (l Level) String() string {
	switch l {
	case Critical:
		return "critical"
	case Warning:
		return "warning"
	default:
		return "ok"
	}
}

// Color maps a Level to a traffic-light color name for display.
func (l Level) Color() string {
	switch l {
	case Critical:
		return "red"
	case Warning:
		return "yellow"
	default:
		return "green"
	}
}

// Headroom thresholds: remaining budget relative to remaining time.
const (
	HeadroomCritical = 0.25 // below this, level is always Critical
	HeadroomLow      = 0.50 // below this + high utilization, level is Critical
)

// Pace ratio thresholds: burn rate relative to elapsed time.
const (
	PaceOnTrack  = 1.15 // at or below is considered on-track
	PaceElevated = 1.30 // at or below is considered elevated
)

// Utilization thresholds.
const (
	UtilModerate  = 50  // utilization below which level is OK (no pace data)
	UtilHigh      = 80  // utilization above which low headroom escalates to Critical
	UtilCautious  = 90  // utilization above which level is always at least Warning
	UtilExhausted = 100 // utilization at which level is always Critical
)

// Assess evaluates the health of a usage period and returns a Level.
//
// It considers three signals in priority order:
//  1. Utilization — is the quota exhausted or nearly so?
//  2. Headroom — is remaining budget sustainable for the remaining time?
//  3. Pace — is the current burn rate on track to finish within budget?
//
// The elapsedRatio (0.0–1.0) enables headroom-aware assessment. Without it,
// only pace and utilization are used.
func Assess(paceRatio *float64, utilization int, elapsedRatio *float64) Level {
	if utilization >= UtilExhausted {
		return Critical
	}
	if paceRatio == nil {
		return fromUtilization(utilization)
	}
	if elapsedRatio != nil && *elapsedRatio < 1.0 {
		if level, escalated := fromHeadroom(utilization, *elapsedRatio); escalated {
			return level
		}
	}
	return fromPace(*paceRatio, utilization)
}

// fromUtilization assesses level from utilization alone, used when no pace
// data is available (e.g. too early in the period or no reset time known).
func fromUtilization(utilization int) Level {
	if utilization < UtilModerate {
		return OK
	}
	if utilization < UtilHigh {
		return Warning
	}
	return Critical
}

// fromHeadroom checks if remaining budget is dangerously low relative to
// remaining time. Returns the escalated level and true if headroom is
// insufficient, or (_, false) to defer to pace-based assessment.
func fromHeadroom(utilization int, elapsedRatio float64) (Level, bool) {
	h := HeadroomRatio(utilization, elapsedRatio)
	if h < HeadroomCritical {
		return Critical, true
	}
	if h < HeadroomLow && utilization >= UtilHigh {
		return Critical, true
	}
	return OK, false
}

// fromPace assesses level from burn rate, with a near-exhaustion floor
// that prevents high utilization from ever appearing OK.
func fromPace(paceRatio float64, utilization int) Level {
	if utilization >= UtilCautious {
		if paceRatio > PaceOnTrack {
			return Critical
		}
		return Warning
	}
	if paceRatio <= PaceOnTrack {
		return OK
	}
	if paceRatio <= PaceElevated {
		return Warning
	}
	return Critical
}

// HeadroomRatio computes remaining budget relative to remaining time.
// A value of 1.0 means budget and time are proportional (on track);
// below 1.0 means budget is running out faster than time.
func HeadroomRatio(utilization int, elapsedRatio float64) float64 {
	remainingBudget := float64(100 - utilization)
	remainingTime := (1.0 - elapsedRatio) * 100.0
	if remainingTime <= 0 {
		if remainingBudget <= 0 {
			return 0
		}
		return math.Inf(1)
	}
	return remainingBudget / remainingTime
}

// EffectiveHeadroom adjusts raw headroom for multiplier cost.
//   - nil multiplier: headroom is used as-is
//   - multiplier == 0: model is free, effective headroom is 100
//   - multiplier > 0: headroom / multiplier, capped at 100
func EffectiveHeadroom(headroom int, multiplier *float64) int {
	if multiplier == nil {
		return headroom
	}
	if *multiplier == 0 {
		return 100
	}
	eff := float64(headroom) / *multiplier
	return int(math.Min(eff, 100))
}
