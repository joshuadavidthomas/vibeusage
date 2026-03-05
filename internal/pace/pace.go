package pace

import "math"

// Headroom thresholds: remaining budget relative to remaining time.
const (
	HeadroomCritical = 0.25 // below this, color is always red
	HeadroomLow      = 0.50 // below this + high utilization, color is red
)

// Pace ratio thresholds: burn rate relative to elapsed time.
const (
	PaceOnTrack  = 1.15 // at or below is considered on-track (green)
	PaceElevated = 1.30 // at or below is considered elevated (yellow)
)

// Utilization thresholds.
const (
	UtilModerate  = 50  // utilization below which color is green (no pace data)
	UtilHigh      = 80  // utilization above which low headroom triggers red
	UtilCautious  = 90  // utilization above which color is always at least yellow
	UtilExhausted = 100 // utilization at which color is always red
)

// Color returns a color name ("green", "yellow", or "red") based on the
// pace ratio, current utilization percentage, and elapsed time in the period.
//
// The elapsedRatio (0.0–1.0) enables headroom-aware color decisions: pace
// ratio alone can mask dangerously low remaining budget when utilization is
// high (e.g. 99% at 86% elapsed has pace ~1.15 but only 1% headroom).
func Color(paceRatio *float64, utilization int, elapsedRatio *float64) string {
	// Exhausted quota is always red — you're blocked regardless of pace.
	if utilization >= UtilExhausted {
		return "red"
	}
	if paceRatio == nil {
		if utilization < UtilModerate {
			return "green"
		}
		if utilization < UtilHigh {
			return "yellow"
		}
		return "red"
	}
	// Headroom check: evaluate remaining budget relative to remaining time.
	// Pace ratio captures burn rate but can mask how little budget actually
	// remains. Example: 99% utilization at 86% elapsed → pace 1.15 looks
	// borderline, but only 1% budget for 14% of the period is clearly
	// unsustainable.
	if elapsedRatio != nil && *elapsedRatio < 1.0 {
		h := HeadroomRatio(utilization, *elapsedRatio)
		if h < HeadroomCritical {
			return "red"
		}
		if h < HeadroomLow && utilization >= UtilHigh {
			return "red"
		}
	}
	// Near-exhaustion floor: ≥90% utilization is always at least yellow.
	// At 90%+ you have ≤10% left regardless of pace, which warrants a
	// caution signal. Pace can still escalate to red, but not rescue to green.
	if utilization >= UtilCautious {
		if *paceRatio > PaceOnTrack {
			return "red"
		}
		return "yellow"
	}
	if *paceRatio <= PaceOnTrack {
		return "green"
	}
	if *paceRatio <= PaceElevated {
		return "yellow"
	}
	return "red"
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
