package routing

import (
	"math"
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
	// Multiplier is the premium request cost multiplier (e.g. 3.0 for expensive
	// models on Copilot). Nil means no multiplier applies.
	Multiplier *float64 `json:"multiplier,omitempty"`
	// EffectiveHeadroom is headroom adjusted for multiplier cost. For providers
	// without multipliers, this equals Headroom. For multiplier 0 (free models),
	// this is 100. For multiplier > 0, this is headroom / multiplier, capped at 100.
	EffectiveHeadroom int `json:"effective_headroom"`
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
	// Best is the top-ranked candidate (most effective headroom). Nil if no providers have data.
	Best *Candidate `json:"best,omitempty"`
	// Candidates is all providers ranked by effective headroom (descending).
	Candidates []Candidate `json:"candidates"`
	// Unavailable lists provider IDs that offer the model but had no usage data.
	Unavailable []string `json:"unavailable,omitempty"`
}

// Rank takes a set of provider snapshots and the list of provider IDs that
// offer a model, and returns candidates sorted by effective headroom (most first).
// The multipliers map provides per-provider premium request cost multipliers
// (e.g. from Copilot). Providers without successful snapshots are returned
// in the unavailable list.
func Rank(providerIDs []string, snapshots map[string]ProviderData, multipliers map[string]*float64) (candidates []Candidate, unavailable []string) {
	for _, pid := range providerIDs {
		data, ok := snapshots[pid]
		if !ok || data.Snapshot == nil {
			unavailable = append(unavailable, pid)
			continue
		}

		snap := data.Snapshot
		primary := snap.BottleneckPeriod()
		if primary == nil {
			unavailable = append(unavailable, pid)
			continue
		}

		var plan string
		if snap.Identity != nil {
			plan = snap.Identity.Plan
		}

		headroom := primary.Remaining()
		mult := multipliers[pid]
		effectiveHeadroom := computeEffectiveHeadroom(headroom, mult)

		candidates = append(candidates, Candidate{
			ProviderID:        pid,
			Headroom:          headroom,
			Utilization:       primary.Utilization,
			Multiplier:        mult,
			EffectiveHeadroom: effectiveHeadroom,
			PeriodType:        primary.PeriodType,
			ResetsAt:          primary.ResetsAt,
			Plan:              plan,
			Cached:            data.Cached,
		})
	}

	// Sort by effective headroom descending, then provider ID ascending for stability.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].EffectiveHeadroom != candidates[j].EffectiveHeadroom {
			return candidates[i].EffectiveHeadroom > candidates[j].EffectiveHeadroom
		}
		return candidates[i].ProviderID < candidates[j].ProviderID
	})

	sort.Strings(unavailable)
	return candidates, unavailable
}

// computeEffectiveHeadroom adjusts raw headroom for multiplier cost.
//   - nil multiplier (non-copilot): headroom is used as-is
//   - multiplier == 0: model is free, effective headroom is 100
//   - multiplier > 0: headroom / multiplier, capped at 100
func computeEffectiveHeadroom(headroom int, multiplier *float64) int {
	if multiplier == nil {
		return headroom
	}
	if *multiplier == 0 {
		return 100
	}
	eff := float64(headroom) / *multiplier
	return int(math.Min(eff, 100))
}

// ProviderData bundles a snapshot with its cache status.
type ProviderData struct {
	Snapshot *models.UsageSnapshot
	Cached   bool
}

// RoleCandidate extends Candidate with the model that was matched,
// used when routing by role (multiple models across providers).
type RoleCandidate struct {
	Candidate
	// ModelID is the canonical model identifier.
	ModelID string `json:"model_id"`
	// ModelName is the human-readable model name.
	ModelName string `json:"model_name"`
}

// RoleRecommendation is the result of routing a role across models and providers.
type RoleRecommendation struct {
	// Role is the user-defined role name that was queried.
	Role string `json:"role"`
	// Best is the top-ranked candidate. Nil if no providers have data.
	Best *RoleCandidate `json:"best,omitempty"`
	// Candidates is all model+provider pairs ranked by effective headroom (descending).
	Candidates []RoleCandidate `json:"candidates"`
	// Unavailable lists model+provider pairs that had no usage data.
	Unavailable []RoleUnavailable `json:"unavailable,omitempty"`
}

// RoleUnavailable records a model+provider pair that could not be ranked.
type RoleUnavailable struct {
	ModelID    string `json:"model_id"`
	ProviderID string `json:"provider_id"`
}

// RoleModelEntry describes a model and its configured providers for role-based routing.
type RoleModelEntry struct {
	ModelID     string
	ModelName   string
	ProviderIDs []string
}

// RankByRole ranks candidates across multiple models and their providers.
// Each entry in modelEntries represents a model and the configured providers
// that offer it. The snapshots map is keyed by provider ID (shared across models).
func RankByRole(modelEntries []RoleModelEntry, snapshots map[string]ProviderData, multiplierFn func(modelName string, providerID string) *float64) (candidates []RoleCandidate, unavailable []RoleUnavailable) {
	seen := make(map[string]bool) // "providerID" → already added (avoid duplicates)

	for _, entry := range modelEntries {
		for _, pid := range entry.ProviderIDs {
			key := pid
			if seen[key] {
				// A provider can only appear once — use the first model that maps to it.
				// This avoids listing "claude" twice for sonnet + opus in the same role.
				continue
			}

			data, ok := snapshots[pid]
			if !ok || data.Snapshot == nil {
				unavailable = append(unavailable, RoleUnavailable{
					ModelID:    entry.ModelID,
					ProviderID: pid,
				})
				seen[key] = true
				continue
			}

			snap := data.Snapshot
			primary := snap.BottleneckPeriod()
			if primary == nil {
				unavailable = append(unavailable, RoleUnavailable{
					ModelID:    entry.ModelID,
					ProviderID: pid,
				})
				seen[key] = true
				continue
			}

			var plan string
			if snap.Identity != nil {
				plan = snap.Identity.Plan
			}

			headroom := primary.Remaining()
			var mult *float64
			if multiplierFn != nil {
				mult = multiplierFn(entry.ModelName, pid)
			}
			effectiveHeadroom := computeEffectiveHeadroom(headroom, mult)

			candidates = append(candidates, RoleCandidate{
				Candidate: Candidate{
					ProviderID:        pid,
					Headroom:          headroom,
					Utilization:       primary.Utilization,
					Multiplier:        mult,
					EffectiveHeadroom: effectiveHeadroom,
					PeriodType:        primary.PeriodType,
					ResetsAt:          primary.ResetsAt,
					Plan:              plan,
					Cached:            data.Cached,
				},
				ModelID:   entry.ModelID,
				ModelName: entry.ModelName,
			})
			seen[key] = true
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].EffectiveHeadroom != candidates[j].EffectiveHeadroom {
			return candidates[i].EffectiveHeadroom > candidates[j].EffectiveHeadroom
		}
		if candidates[i].ProviderID != candidates[j].ProviderID {
			return candidates[i].ProviderID < candidates[j].ProviderID
		}
		return candidates[i].ModelID < candidates[j].ModelID
	})

	sort.Slice(unavailable, func(i, j int) bool {
		if unavailable[i].ProviderID != unavailable[j].ProviderID {
			return unavailable[i].ProviderID < unavailable[j].ProviderID
		}
		return unavailable[i].ModelID < unavailable[j].ModelID
	})

	return candidates, unavailable
}
