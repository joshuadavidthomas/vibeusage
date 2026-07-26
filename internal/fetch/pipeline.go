package fetch

import (
	"context"
	"fmt"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// ExecutePipeline tries each strategy in order until one succeeds.
// When enabled, a very short fresh-cache window deduplicates bursty repeat
// invocations before any live fetch is attempted. All configuration is
// provided via cfg rather than read from a global singleton.
func ExecutePipeline(ctx context.Context, providerID string, strategies []Strategy, useCache bool, cfg PipelineConfig) FetchOutcome {
	logger := logging.FromContext(ctx)
	anyAttempted := false
	lastErr := ""

	// Honor a persisted rate-limit cooldown before any network attempt.
	// Within the window, serve cache if present; otherwise surface the
	// cooldown as the error so the user sees why nothing fetched.
	if useCache && cfg.Throttles != nil && hasAvailableStrategy(strategies) {
		marker, err := cfg.Throttles.Load(providerID)
		if err != nil {
			logger.Warn("loading throttle marker failed", "provider", providerID, "err", err)
			marker = nil
		}
		if marker != nil {
			if cfg.Cache != nil {
				cached, err := cfg.Cache.Load(providerID)
				if err != nil {
					logger.Warn("loading cached snapshot failed", "provider", providerID, "err", err)
					cached = nil
				}
				if cached != nil {
					return FetchOutcome{
						ProviderID: providerID,
						Success:    true,
						Snapshot:   cached,
						Source:     "cache (throttled)",
						Cached:     true,
					}
				}
			}
			reason := marker.Reason
			if reason == "" {
				reason = "Rate limited"
			}
			return FetchOutcome{
				ProviderID: providerID,
				Success:    false,
				Error:      fmt.Sprintf("%s; retry after %s", reason, marker.RetryAt.Format(time.RFC3339)),
			}
		}
	}

	if useCache && cfg.Cache != nil && cfg.FreshCacheTTL > 0 && hasAvailableStrategy(strategies) {
		cached, err := cfg.Cache.Load(providerID)
		if err != nil {
			logger.Warn("loading cached snapshot failed", "provider", providerID, "err", err)
			cached = nil
		}
		if isFreshSnapshot(cached, cfg.FreshCacheTTL) {
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

		attemptCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		result, fetchErr := strategy.Fetch(attemptCtx)
		attemptErr := attemptCtx.Err()
		cancel()

		if ctx.Err() != nil {
			return FetchOutcome{
				ProviderID: providerID,
				Success:    false,
				Error:      "Context cancelled",
			}
		}
		if attemptErr == context.DeadlineExceeded {
			lastErr = "Fetch timed out"
			continue
		}
		if fetchErr != nil {
			lastErr = fetchErr.Error()
			continue
		}

		if result.RetryAfter != nil && cfg.Throttles != nil {
			reason := result.Error
			if reason == "" {
				reason = "Rate limited"
			}
			if err := cfg.Throttles.Save(providerID, ThrottleMarker{RetryAt: *result.RetryAfter, Reason: reason}); err != nil {
				logger.Warn("saving throttle marker failed", "provider", providerID, "err", err)
			}
		}

		if result.Success && result.Snapshot != nil {
			if cfg.Cache != nil {
				if err := cfg.Cache.Save(*result.Snapshot); err != nil {
					logger.Warn("saving cached snapshot failed", "provider", providerID, "err", err)
				}
			}
			if cfg.Throttles != nil {
				if err := cfg.Throttles.Clear(providerID); err != nil {
					logger.Warn("clearing throttle marker failed", "provider", providerID, "err", err)
				}
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
		cached, err := cfg.Cache.Load(providerID)
		if err != nil {
			logger.Warn("loading cached snapshot failed", "provider", providerID, "err", err)
			cached = nil
		}
		if cached != nil && anyAttempted {
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
