package fetch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// mockStrategy implements Strategy for testing.
type mockStrategy struct {
	available bool
	fetchFn   func(ctx context.Context) (FetchResult, error)
}

func (m *mockStrategy) IsAvailable() bool                              { return m.available }
func (m *mockStrategy) Fetch(ctx context.Context) (FetchResult, error) { return m.fetchFn(ctx) }

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

// testSnapshot returns a minimal valid snapshot for testing.
func testSnapshot(provider, source string, utilization int) models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  provider,
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "daily", Utilization: utilization}},
		Source:    source,
	}
}

func TestExecutePipeline_ContextCancellation(t *testing.T) {
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			<-ctx.Done()
			return ResultFail("cancelled"), ctx.Err()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)
	if outcome.Success {
		t.Error("expected failure for cancelled context")
	}
	if outcome.Error != "Context cancelled" {
		t.Errorf("expected 'Context cancelled' error, got: %s", outcome.Error)
	}
}

func TestExecutePipeline_ContextPassedToStrategy(t *testing.T) {
	var receivedCtx context.Context

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			receivedCtx = ctx
			snap := models.UsageSnapshot{
				Provider:  "test",
				FetchedAt: time.Now().UTC(),
				Periods:   []models.UsagePeriod{{Name: "test", Utilization: 50}},
			}
			return ResultOK(snap), nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)
	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if receivedCtx == nil {
		t.Fatal("strategy did not receive a context")
	}
	cancel()
	select {
	case <-receivedCtx.Done():
		// Expected
	default:
		t.Error("strategy received a context that is not derived from the parent")
	}
}

func TestExecutePipeline_SuccessfulFetch(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "test",
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "daily", Utilization: 42}},
		Source:    "mock",
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(snap), nil
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)
	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Snapshot == nil {
		t.Fatal("expected snapshot")
	}
	// Source is derived from type name: *fetch.mockStrategy → "mock"
	if outcome.Source != "mock" {
		t.Errorf("expected source 'mock', got '%s'", outcome.Source)
	}
}

func TestExecutePipeline_FallbackToSecondStrategy(t *testing.T) {
	type fallbackStrategy struct{ mockStrategy }

	snap := testSnapshot("test", "fallback", 10)

	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("primary failed"), nil
			},
		},
		&fallbackStrategy{mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		}},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)
	if !outcome.Success {
		t.Fatalf("expected success from fallback, got error: %s", outcome.Error)
	}
	if outcome.Source != "fallback" {
		t.Errorf("expected source 'fallback', got '%s'", outcome.Source)
	}
}

func TestExecutePipeline_FatalStopsChain(t *testing.T) {
	fallbackCalled := false
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFatal("token expired"), nil
			},
		},
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				fallbackCalled = true
				return ResultOK(models.UsageSnapshot{}), nil
			},
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)
	if outcome.Success {
		t.Error("expected failure from fatal error")
	}
	if outcome.Error != "token expired" {
		t.Errorf("expected 'token expired', got '%s'", outcome.Error)
	}
	if fallbackCalled {
		t.Error("fallback strategy should not have been called after fatal error")
	}
}

func TestExecutePipeline_UnavailableStrategy(t *testing.T) {
	strategy := &mockStrategy{
		available: false,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			t.Fatal("should not call Fetch on unavailable strategy")
			return FetchResult{}, nil
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)
	if outcome.Success {
		t.Error("expected failure when only strategy is unavailable")
	}
	if outcome.Error != "No strategies available" {
		t.Errorf("expected 'No strategies available', got '%s'", outcome.Error)
	}
}

func TestExecutePipeline_EmptyStrategies(t *testing.T) {
	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", nil, false, cfg)
	if outcome.Success {
		t.Error("expected failure with no strategies")
	}
	if outcome.Error != "No strategies available" {
		t.Errorf("expected 'No strategies available', got '%s'", outcome.Error)
	}
}

func TestExecutePipeline_Timeout(t *testing.T) {
	cfg := PipelineConfig{
		Timeout:               50 * time.Millisecond,
		StaleThresholdMinutes: 60,
		Cache:                 newMemCache(),
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			time.Sleep(500 * time.Millisecond)
			return ResultOK(testSnapshot("test", "slow", 50)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)

	if outcome.Success {
		t.Error("expected failure due to timeout")
	}
	if outcome.Error != "Fetch timed out" {
		t.Errorf("error = %q, want 'Fetch timed out'", outcome.Error)
	}
}

func TestExecutePipeline_TimeoutFallsBackToNextStrategy(t *testing.T) {
	type fastStrategy struct{ mockStrategy }

	cfg := PipelineConfig{
		Timeout:               50 * time.Millisecond,
		StaleThresholdMinutes: 60,
		Cache:                 newMemCache(),
	}

	snap := testSnapshot("test", "fast", 42)
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				time.Sleep(500 * time.Millisecond)
				return ResultOK(testSnapshot("test", "slow", 0)), nil
			},
		},
		&fastStrategy{mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		}},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from fast strategy, got error: %s", outcome.Error)
	}
	if outcome.Source != "fast" {
		t.Errorf("expected source %q, got %q", "fast", outcome.Source)
	}
}

func TestExecutePipeline_FetchReturnsGoError(t *testing.T) {
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return FetchResult{}, fmt.Errorf("connection refused")
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)

	if outcome.Success {
		t.Error("expected failure when strategy returns Go error")
	}
	if outcome.Error != "connection refused" {
		t.Errorf("outcome error = %q, want %q", outcome.Error, "connection refused")
	}
}

func TestExecutePipeline_GoErrorFallsBackToNextStrategy(t *testing.T) {
	type backupStrategy struct{ mockStrategy }

	snap := testSnapshot("test", "backup", 25)
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return FetchResult{}, fmt.Errorf("DNS resolution failed")
			},
		},
		&backupStrategy{mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		}},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from backup, got error: %s", outcome.Error)
	}
	if outcome.Source != "backup" {
		t.Errorf("expected source %q, got %q", "backup", outcome.Source)
	}
}

func TestExecutePipeline_CacheFallback(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-10 * time.Minute).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}

	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from cache fallback, got error: %s", outcome.Error)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true for cache fallback")
	}
	if outcome.Source != "cache" {
		t.Errorf("expected source %q, got %q", "cache", outcome.Source)
	}
	if outcome.Snapshot == nil {
		t.Fatal("expected non-nil snapshot from cache")
	}
	if outcome.Snapshot.Provider != "test-provider" {
		t.Errorf("cached snapshot provider = %q, want %q", outcome.Snapshot.Provider, "test-provider")
	}
}

func TestExecutePipeline_CacheFallbackServesStaleWhenFetchAttempted(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-2 * time.Hour).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}

	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from cache fallback when fetch was attempted, got error: %s", outcome.Error)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true")
	}
}

func TestExecutePipeline_CacheFallbackRejectsStaleWhenNotConfigured(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-2 * time.Hour).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}

	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	strategy := &mockStrategy{
		available: false,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			t.Fatal("should not call Fetch on unavailable strategy")
			return FetchResult{}, nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if outcome.Success {
		t.Error("expected failure when cached data is stale and no credentials configured")
	}
	if outcome.Cached {
		t.Error("Cached should be false when data is too old and unconfigured")
	}
}

func TestExecutePipeline_CacheFallbackNoData(t *testing.T) {
	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 newMemCache(),
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if outcome.Success {
		t.Error("expected failure when no cache data exists")
	}
	if outcome.Cached {
		t.Error("Cached should be false when no cache data exists")
	}
	if outcome.Error != "API error" {
		t.Errorf("expected error %q, got %q", "API error", outcome.Error)
	}
}

func TestExecutePipeline_CacheDisabled(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 10}},
	}

	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)

	if outcome.Success {
		t.Error("expected failure when cache is disabled")
	}
	if outcome.Cached {
		t.Error("Cached should be false when cache is disabled")
	}
}

func TestExecutePipeline_ThreeStrategyChain(t *testing.T) {
	type thirdStrategy struct{ mockStrategy }

	snap := testSnapshot("test", "third", 75)
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("first failed"), nil
			},
		},
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("second failed"), nil
			},
		},
		&thirdStrategy{mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		}},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from third strategy, got error: %s", outcome.Error)
	}
	if outcome.Source != "third" {
		t.Errorf("expected source %q, got %q", "third", outcome.Source)
	}
}

func TestExecutePipeline_LastErrorPropagated(t *testing.T) {
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("auth expired"), nil
			},
		},
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return FetchResult{}, fmt.Errorf("network timeout")
			},
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if outcome.Success {
		t.Error("expected failure")
	}
	if outcome.Error != "network timeout" {
		t.Errorf("outcome error = %q, want %q (last error)", outcome.Error, "network timeout")
	}
}

func TestExecutePipeline_SuccessCachesResult(t *testing.T) {
	cache := newMemCache()
	cfg := PipelineConfig{
		Timeout:               30 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 cache,
	}

	snap := testSnapshot("cache-test-provider", "mock", 60)
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(snap), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "cache-test-provider", []Strategy{strategy}, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}

	cached := cache.Load("cache-test-provider")
	if cached == nil {
		t.Fatal("expected snapshot to be cached after successful fetch")
	}
	if cached.Provider != "cache-test-provider" {
		t.Errorf("cached provider = %q, want %q", cached.Provider, "cache-test-provider")
	}
}

func TestExecutePipeline_SuccessStopsChain(t *testing.T) {
	secondCalled := false
	strategies := []Strategy{
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(testSnapshot("test", "first", 20)), nil
			},
		},
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				secondCalled = true
				return ResultOK(testSnapshot("test", "second", 0)), nil
			},
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if secondCalled {
		t.Error("second strategy should not be called when first succeeds")
	}
}

func TestExecutePipeline_ProviderIDInOutcome(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		success    bool
	}{
		{"success case", "my-provider", true},
		{"failure case", "failing-provider", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var strategy *mockStrategy
			if tt.success {
				strategy = &mockStrategy{
					available: true,
					fetchFn: func(ctx context.Context) (FetchResult, error) {
						return ResultOK(testSnapshot(tt.providerID, "mock", 50)), nil
					},
				}
			} else {
				strategy = &mockStrategy{
					available: true,
					fetchFn: func(ctx context.Context) (FetchResult, error) {
						return ResultFail("error"), nil
					},
				}
			}

			ctx := context.Background()
			cfg := defaultTestPipelineCfg()
			outcome := ExecutePipeline(ctx, tt.providerID, []Strategy{strategy}, false, cfg)

			if outcome.ProviderID != tt.providerID {
				t.Errorf("ProviderID = %q, want %q", outcome.ProviderID, tt.providerID)
			}
		})
	}
}

func TestExecutePipeline_SkipsUnavailableTriesAvailable(t *testing.T) {
	snap := testSnapshot("test", "mock", 55)
	strategies := []Strategy{
		&mockStrategy{
			available: false,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				t.Fatal("should not call Fetch on unavailable strategy")
				return FetchResult{}, nil
			},
		},
		&mockStrategy{
			available: false,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				t.Fatal("should not call Fetch on unavailable strategy")
				return FetchResult{}, nil
			},
		},
		&mockStrategy{
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	cfg := defaultTestPipelineCfg()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
}

func TestExecutePipeline_CacheFallbackServesFreshWhenNotConfigured(t *testing.T) {
	cache := newMemCache()
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

	strategy := &mockStrategy{
		available: false,
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from fresh cache, got error: %s", outcome.Error)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true")
	}
}

func TestExecutePipeline_NilCacheNoFallback(t *testing.T) {
	cfg := PipelineConfig{
		Timeout:               5 * time.Second,
		StaleThresholdMinutes: 60,
		Cache:                 nil,
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if outcome.Success {
		t.Error("expected failure — nil cache means no fallback")
	}
}
