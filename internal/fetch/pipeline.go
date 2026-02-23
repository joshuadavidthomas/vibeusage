package fetch

import (
	"context"
	"time"
)

// ExecutePipeline tries each strategy in order until one succeeds.
// All configuration (timeout, stale threshold, cache) is provided via cfg
// rather than read from a global singleton.
func ExecutePipeline(ctx context.Context, providerID string, strategies []Strategy, useCache bool, cfg PipelineConfig) FetchOutcome {
	var attempts []FetchAttempt

	// Try each strategy
	for _, strategy := range strategies {
		if !strategy.IsAvailable() {
			attempts = append(attempts, FetchAttempt{
				Strategy: strategy.Name(),
				Success:  false,
				Error:    "not configured",
			})
			continue
		}

		start := time.Now()

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
				Attempts:   attempts,
				Error:      "Context cancelled",
			}
		case <-time.After(cfg.Timeout):
			durationMs := int(time.Since(start).Milliseconds())
			attempts = append(attempts, FetchAttempt{
				Strategy:   strategy.Name(),
				Success:    false,
				Error:      "Fetch timed out",
				DurationMs: durationMs,
			})
			continue
		case r := <-resultCh:
			result = r.result
			fetchErr = r.err
		}

		durationMs := int(time.Since(start).Milliseconds())

		if fetchErr != nil {
			attempts = append(attempts, FetchAttempt{
				Strategy:   strategy.Name(),
				Success:    false,
				Error:      fetchErr.Error(),
				DurationMs: durationMs,
			})
			continue
		}

		if result.Success && result.Snapshot != nil {
			// Cache the result
			if cfg.Cache != nil {
				_ = cfg.Cache.Save(*result.Snapshot)
			}

			return FetchOutcome{
				ProviderID: providerID,
				Success:    true,
				Snapshot:   result.Snapshot,
				Source:     strategy.Name(),
				Attempts:   attempts,
			}
		}

		if !result.ShouldFallback {
			attempts = append(attempts, FetchAttempt{
				Strategy:   strategy.Name(),
				Success:    false,
				Error:      result.Error,
				DurationMs: durationMs,
			})
			return FetchOutcome{
				ProviderID: providerID,
				Success:    false,
				Attempts:   attempts,
				Error:      result.Error,
			}
		}

		attempts = append(attempts, FetchAttempt{
			Strategy:   strategy.Name(),
			Success:    false,
			Error:      result.Error,
			DurationMs: durationMs,
		})
	}

	// Did any strategy actually attempt a fetch (had credentials)?
	anyAttempted := false
	for _, a := range attempts {
		if a.Error != "not configured" {
			anyAttempted = true
			break
		}
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
					Attempts:   attempts,
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
					Attempts:   attempts,
					Cached:     true,
				}
			}
		}
	}

	lastErr := "No strategies available"
	if len(attempts) > 0 {
		lastErr = attempts[len(attempts)-1].Error
	}

	return FetchOutcome{
		ProviderID: providerID,
		Success:    false,
		Attempts:   attempts,
		Error:      lastErr,
	}
}

type fetchAttemptResult struct {
	result FetchResult
	err    error
}
