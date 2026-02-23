package fetch

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// memCache is a thread-safe in-memory Cache for testing, replacing filesystem deps.
type memCache struct {
	mu   sync.Mutex
	data map[string]models.UsageSnapshot
}

func newMemCache() *memCache {
	return &memCache{data: make(map[string]models.UsageSnapshot)}
}

func (c *memCache) Save(snap models.UsageSnapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[snap.Provider] = snap
	return nil
}

func (c *memCache) Load(providerID string) *models.UsageSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.data[providerID]
	if !ok {
		return nil
	}
	return &s
}

func defaultTestPipelineCfg() PipelineConfig {
	return PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 newMemCache(),
	}
}

// Tests that ExecutePipeline accepts PipelineConfig (no config.Get calls).

func TestExecutePipeline_PipelineConfig_Success(t *testing.T) {
	cache := newMemCache()
	cfg := PipelineConfig{
		Timeout:               5 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	snap := models.UsageSnapshot{
		Provider:  "test",
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "daily", Utilization: 42}},
		Source:    "mock",
	}
	strategy := &mockStrategy{
		name:      "mock",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(snap), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Source != "mock" {
		t.Errorf("expected source 'mock', got %q", outcome.Source)
	}

	// Verify the snapshot was cached via the injected cache
	cached := cache.Load("test")
	if cached == nil {
		t.Fatal("expected snapshot to be cached via injected Cache")
	}
}

func TestExecutePipeline_PipelineConfig_Timeout(t *testing.T) {
	cfg := PipelineConfig{
		Timeout:               50 * time.Millisecond,
		StaleThresholdMinutes: 60,
		Cache:                 newMemCache(),
	}

	strategy := &mockStrategy{
		name:      "slow",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			time.Sleep(500 * time.Millisecond)
			return ResultOK(models.UsageSnapshot{}), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)

	if outcome.Success {
		t.Error("expected failure due to timeout")
	}
	if len(outcome.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(outcome.Attempts))
	}
	if outcome.Attempts[0].Error != "Fetch timed out" {
		t.Errorf("expected 'Fetch timed out', got %q", outcome.Attempts[0].Error)
	}
}

func TestExecutePipeline_PipelineConfig_CacheFallbackStale(t *testing.T) {
	cache := newMemCache()
	// Old snapshot (2 hours ago)
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-2 * time.Hour).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
	}

	cfg := PipelineConfig{
		Timeout:               5 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	// Unconfigured provider — strategy not available
	strategy := &mockStrategy{
		name:      "unavailable",
		available: false,
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	// Should reject stale cache when no credentials exist
	if outcome.Success {
		t.Error("expected failure when cache is stale and no credentials configured")
	}
}

func TestExecutePipeline_PipelineConfig_CacheFallbackFresh(t *testing.T) {
	cache := newMemCache()
	// Recent snapshot (10 minutes ago)
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-10 * time.Minute).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
	}

	cfg := PipelineConfig{
		Timeout:               5 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	// Unconfigured provider — strategy not available
	strategy := &mockStrategy{
		name:      "unavailable",
		available: false,
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	// Should serve fresh cache even when unconfigured
	if !outcome.Success {
		t.Fatalf("expected success from fresh cache, got error: %s", outcome.Error)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true")
	}
}

func TestExecutePipeline_PipelineConfig_NilCache(t *testing.T) {
	cfg := PipelineConfig{
		Timeout:               5 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 nil,
	}

	strategy := &mockStrategy{
		name:      "failing",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	// Should not panic with nil cache, just skip caching
	if outcome.Success {
		t.Error("expected failure — nil cache means no fallback")
	}
}

// Tests that FetchAllProviders accepts OrchestratorConfig.

func TestFetchAllProviders_OrchestratorConfig_ConcurrencyLimit(t *testing.T) {
	cfg := OrchestratorConfig{
		MaxConcurrent: 2,
		Pipeline: PipelineConfig{
			Timeout:               5 * time.Second,
			StaleThresholdMinutes: 60,
			Cache:                 newMemCache(),
		},
	}

	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	makeStrategy := func(pid string) []Strategy {
		return []Strategy{
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					cur := concurrentCount.Add(1)
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(50 * time.Millisecond)
					concurrentCount.Add(-1)
					return ResultOK(models.UsageSnapshot{
						Provider:  pid,
						FetchedAt: time.Now().UTC(),
						Periods:   []models.UsagePeriod{{Name: "test", Utilization: 10}},
					}), nil
				},
			},
		}
	}

	providerMap := map[string][]Strategy{
		"p1": makeStrategy("p1"),
		"p2": makeStrategy("p2"),
		"p3": makeStrategy("p3"),
		"p4": makeStrategy("p4"),
		"p5": makeStrategy("p5"),
	}

	ctx := context.Background()
	outcomes := FetchAllProviders(ctx, providerMap, true, cfg, nil)

	if len(outcomes) != 5 {
		t.Fatalf("expected 5 outcomes, got %d", len(outcomes))
	}

	observed := maxConcurrent.Load()
	if observed > 2 {
		t.Errorf("max concurrent = %d, expected <= 2", observed)
	}
}

func TestFetchEnabledProviders_OrchestratorConfig_Filter(t *testing.T) {
	cfg := OrchestratorConfig{
		MaxConcurrent: 5,
		Pipeline: PipelineConfig{
			Timeout:               5 * time.Second,
			StaleThresholdMinutes: 60,
			Cache:                 newMemCache(),
		},
	}

	providerMap := map[string][]Strategy{
		"alpha": {&mockStrategy{
			name: "s", available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(models.UsageSnapshot{
					Provider: "alpha", FetchedAt: time.Now().UTC(),
					Periods: []models.UsagePeriod{{Name: "test", Utilization: 10}},
				}), nil
			},
		}},
		"beta": {&mockStrategy{
			name: "s", available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				t.Error("beta should not be fetched — it's not enabled")
				return ResultOK(models.UsageSnapshot{}), nil
			},
		}},
	}

	isEnabled := func(id string) bool { return id == "alpha" }

	ctx := context.Background()
	outcomes := FetchEnabledProviders(ctx, providerMap, true, cfg, isEnabled, nil)

	if _, ok := outcomes["alpha"]; !ok {
		t.Error("expected outcome for 'alpha'")
	}
	if _, ok := outcomes["beta"]; ok {
		t.Error("should not have outcome for disabled 'beta'")
	}
}
