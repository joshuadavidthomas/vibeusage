package fetch

import (
	"context"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// ExecutePipeline tries each strategy in order until one succeeds.
// When enabled, a very short fresh-cache window deduplicates bursty repeat
// invocations before any live fetch is attempted. All configuration is
// provided via cfg rather than read from a global singleton.
func ExecutePipeline(ctx context.Context, providerID string, strategies []Strategy, useCache bool, cfg PipelineConfig) FetchOutcome {
	anyAttempted := false
	lastErr := ""

	if useCache && cfg.Cache != nil && cfg.FreshCacheTTL > 0 && hasAvailableStrategy(strategies) {
		if cached := cfg.Cache.Load(providerID); isFreshSnapshot(cached, cfg.FreshCacheTTL) {
			return FetchOutcome{
				ProviderID: providerID,
				Success:    true,
				Snapshot:   cached,
				Source:     "cache",
				Cached:     true,
			}
		}
	}

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
	// Only serve cache when credentials exist (anyAttempted=true) but
	// the API failed. This provides resilience when services are down
	// without misleading unconfigured users with old data.
	if useCache && cfg.Cache != nil {
		if cached := cfg.Cache.Load(providerID); cached != nil && anyAttempted {
			return FetchOutcome{
				ProviderID: providerID,
				Success:    true,
				Snapshot:   cached,
				Source:     "cache",
				Cached:     true,
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

func hasAvailableStrategy(strategies []Strategy) bool {
	for _, strategy := range strategies {
		if strategy.IsAvailable() {
			return true
		}
	}
	return false
}

func isFreshSnapshot(snapshot *models.UsageSnapshot, ttl time.Duration) bool {
	if snapshot == nil || snapshot.FetchedAt.IsZero() || ttl <= 0 {
		return false
	}
	return time.Since(snapshot.FetchedAt) <= ttl
}
