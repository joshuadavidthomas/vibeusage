package routing

import (
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
)

// BuildProviderData converts fetch outcomes into the ProviderData map
// used by Rank and RankByRole.
func BuildProviderData(outcomes map[string]fetch.FetchOutcome) map[string]ProviderData {
	data := make(map[string]ProviderData)
	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			data[pid] = ProviderData{
				Snapshot: outcome.Snapshot,
				Cached:   outcome.Cached,
			}
		}
	}
	return data
}

// BuildStrategyMap builds a provider→strategies map using the given lookup function.
func BuildStrategyMap(providerIDs []string, lookupStrategies func(string) []fetch.Strategy) map[string][]fetch.Strategy {
	m := make(map[string][]fetch.Strategy, len(providerIDs))
	for _, pid := range providerIDs {
		m[pid] = lookupStrategies(pid)
	}
	return m
}
