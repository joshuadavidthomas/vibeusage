package opencode

// SSR usage window parsed from the Go page's embedded JavaScript.
// Matches: rollingUsage:{status:"ok",resetInSec:13553,usagePercent:1}
type ssrUsageWindow struct {
	Status       string
	ResetInSec   float64
	UsagePercent float64
}
