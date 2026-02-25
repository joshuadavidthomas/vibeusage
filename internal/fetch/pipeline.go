package fetch

import (
	"context"
	"time"
)

// ExecutePipeline tries each strategy in order until one succeeds.
// All configuration (timeout, stale threshold, cache) is provided via cfg
// rather than read from a global singleton.
func ExecutePipeline(ctx context.Context, providerID string, strategies []Strategy, useCache bool, cfg PipelineConfig) FetchOutcome {
	anyAttempted := false
	lastErr := ""

	for _, strategy := range strategies {
		if !strategy.IsAvailable() {
			continue
		}

		anyAttempted = true

		resultCh := make(chan fetchAttemptResult, 1)
		go func() {
			result, err := strategy.Fetch(ctx)
			resultCh <- fetchAttemptResult{result: result, err: err}
		}()

		var result FetchResult
		var fetchErr error

		select {
		case <-ctx.Done():
			return FetchOutcome{
				ProviderID: providerID,
				Success:    false,
				Error:      "Context cancelled",
			}
		case <-time.After(cfg.Timeout):
			lastErr = "Fetch timed out"
			continue
		case r := <-resultCh:
			result = r.result
			fetchErr = r.err
		}

		if fetchErr != nil {
			lastErr = fetchErr.Error()
			continue
		}

		if result.Success && result.Snapshot != nil {
			if cfg.Cache != nil {
				_ = cfg.Cache.Save(*result.Snapshot)
			}

			return FetchOutcome{
				ProviderID: providerID,
				Success:    true,
				Snapshot:   result.Snapshot,
				Source:     StrategyName(strategy),
			}
		}

		if !result.ShouldFallback {
			return FetchOutcome{
				ProviderID: providerID,
				Success:    false,
				Error:      result.Error,
			}
		}

		lastErr = result.Error
	}

	// All strategies failed — try cache fallback.
	// If a real fetch was attempted (credentials exist, API failed),
	// always serve cache — the service is probably just down.
	// If nothing was even attempted (no credentials), only serve
	// cache within the stale threshold — old data with no way to
	// refresh is misleading.
	if useCache && cfg.Cache != nil {
		if cached := cfg.Cache.Load(providerID); cached != nil {
			if anyAttempted {
				return FetchOutcome{
					ProviderID: providerID,
					Success:    true,
					Snapshot:   cached,
					Source:     "cache",
					Cached:     true,
				}
			}
			ageMinutes := int(time.Since(cached.FetchedAt).Minutes())
			if ageMinutes < cfg.StaleThresholdMinutes {
				return FetchOutcome{
					ProviderID: providerID,
					Success:    true,
					Snapshot:   cached,
					Source:     "cache",
					Cached:     true,
				}
			}
		}
	}

	if lastErr == "" {
		lastErr = "No strategies available"
	}

	return FetchOutcome{
		ProviderID: providerID,
		Success:    false,
		Error:      lastErr,
	}
}

type fetchAttemptResult struct {
	result FetchResult
	err    error
}
