package fetch

import (
	"context"
	"sync"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

// FetchAllProviders fetches usage from all providers concurrently.
// When useCache is true, stale cached data is used as a fallback if all strategies fail.
func FetchAllProviders(ctx context.Context, providerMap map[string][]Strategy, useCache bool, onComplete func(FetchOutcome)) map[string]FetchOutcome {
	cfg := config.Get()
	maxConcurrent := cfg.Fetch.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	outcomes := make(map[string]FetchOutcome)
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for pid, strategies := range providerMap {
		wg.Add(1)
		go func(providerID string, strats []Strategy) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			outcome := ExecutePipeline(ctx, providerID, strats, useCache)

			mu.Lock()
			outcomes[providerID] = outcome
			mu.Unlock()

			if onComplete != nil {
				onComplete(outcome)
			}
		}(pid, strategies)
	}

	wg.Wait()
	return outcomes
}

// FetchEnabledProviders fetches only enabled providers.
// When useCache is true, stale cached data is used as a fallback if all strategies fail.
func FetchEnabledProviders(ctx context.Context, providerMap map[string][]Strategy, useCache bool, onComplete func(FetchOutcome)) map[string]FetchOutcome {
	cfg := config.Get()
	enabledMap := make(map[string][]Strategy)
	for pid, strategies := range providerMap {
		if cfg.IsProviderEnabled(pid) {
			enabledMap[pid] = strategies
		}
	}
	return FetchAllProviders(ctx, enabledMap, useCache, onComplete)
}
