package fetch

import (
	"context"
	"sync"
)

// FetchAllProviders fetches usage from all providers concurrently.
// When useCache is true, stale cached data is used as a fallback if all strategies fail.
// Concurrency and pipeline parameters are provided via cfg rather than read from a global singleton.
func FetchAllProviders(ctx context.Context, providerMap map[string][]Strategy, useCache bool, cfg OrchestratorConfig, onComplete func(FetchOutcome)) map[string]FetchOutcome {
	maxConcurrent := cfg.MaxConcurrent
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

			outcome := ExecutePipeline(ctx, providerID, strats, useCache, cfg.Pipeline)

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
// The isEnabled predicate replaces the previous config.Get().IsProviderEnabled dependency.
func FetchEnabledProviders(ctx context.Context, providerMap map[string][]Strategy, useCache bool, cfg OrchestratorConfig, isEnabled func(string) bool, onComplete func(FetchOutcome)) map[string]FetchOutcome {
	enabledMap := make(map[string][]Strategy)
	for pid, strategies := range providerMap {
		if isEnabled(pid) {
			enabledMap[pid] = strategies
		}
	}
	return FetchAllProviders(ctx, enabledMap, useCache, cfg, onComplete)
}
