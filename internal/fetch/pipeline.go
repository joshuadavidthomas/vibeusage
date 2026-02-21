package fetch

import (
	"context"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

// ExecutePipeline tries each strategy in order until one succeeds.
func ExecutePipeline(ctx context.Context, providerID string, strategies []Strategy, useCache bool) FetchOutcome {
	cfg := config.Get()
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
		timeout := time.Duration(cfg.Fetch.Timeout * float64(time.Second))

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
		case <-time.After(timeout):
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
			_ = config.CacheSnapshot(*result.Snapshot)

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
	if useCache {
		if cached := config.LoadCachedSnapshot(providerID); cached != nil {
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
			if ageMinutes < cfg.Fetch.StaleThresholdMinutes {
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
