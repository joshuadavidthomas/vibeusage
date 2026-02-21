package routing

import (
	"sort"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// Candidate represents a provider that can serve a given model,
// along with its current usage headroom.
type Candidate struct {
	// ProviderID is the provider identifier (e.g. "claude", "copilot").
	ProviderID string `json:"provider_id"`
	// Headroom is remaining capacity (100 - utilization). Higher is better.
	Headroom int `json:"headroom"`
	// Utilization is the current usage percentage (0-100).
	Utilization int `json:"utilization"`
	// PeriodType indicates the quota window type.
	PeriodType models.PeriodType `json:"period_type"`
	// ResetsAt is when the quota window resets, if known.
	ResetsAt *time.Time `json:"resets_at,omitempty"`
	// Plan is the subscription tier, if known.
	Plan string `json:"plan,omitempty"`
	// Cached indicates this data came from cache rather than a live fetch.
	Cached bool `json:"cached"`
}

// Recommendation is the result of routing a model query across providers.
type Recommendation struct {
	// ModelID is the canonical model identifier that was resolved.
	ModelID string `json:"model_id"`
	// ModelName is the human-readable model name.
	ModelName string `json:"model_name"`
	// Best is the top-ranked candidate (most headroom). Nil if no providers have data.
	Best *Candidate `json:"best,omitempty"`
	// Candidates is all providers ranked by headroom (descending).
	Candidates []Candidate `json:"candidates"`
	// Unavailable lists provider IDs that offer the model but had no usage data.
	Unavailable []string `json:"unavailable,omitempty"`
}

// Rank takes a set of provider snapshots and the list of provider IDs that
// offer a model, and returns candidates sorted by headroom (most first).
// Providers without successful snapshots are returned in the unavailable list.
func Rank(providerIDs []string, snapshots map[string]ProviderData) (candidates []Candidate, unavailable []string) {
	for _, pid := range providerIDs {
		data, ok := snapshots[pid]
		if !ok || data.Snapshot == nil {
			unavailable = append(unavailable, pid)
			continue
		}

		snap := data.Snapshot
		primary := snap.PrimaryPeriod()
		if primary == nil {
			unavailable = append(unavailable, pid)
			continue
		}

		var plan string
		if snap.Identity != nil {
			plan = snap.Identity.Plan
		}

		candidates = append(candidates, Candidate{
			ProviderID:  pid,
			Headroom:    primary.Remaining(),
			Utilization: primary.Utilization,
			PeriodType:  primary.PeriodType,
			ResetsAt:    primary.ResetsAt,
			Plan:        plan,
			Cached:      data.Cached,
		})
	}

	// Sort by headroom descending, then provider ID ascending for stability.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Headroom != candidates[j].Headroom {
			return candidates[i].Headroom > candidates[j].Headroom
		}
		return candidates[i].ProviderID < candidates[j].ProviderID
	})

	sort.Strings(unavailable)
	return candidates, unavailable
}

// ProviderData bundles a snapshot with its cache status.
type ProviderData struct {
	Snapshot *models.UsageSnapshot
	Cached   bool
}
