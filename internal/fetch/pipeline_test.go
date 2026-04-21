package fetch

import (
	"context"
	"fmt"
	"strings"
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

// memThrottles is a thread-safe in-memory ThrottleStore for testing.
// Load respects RetryAt — callers see nil once the cooldown has passed,
// matching the filesystem implementation.
type memThrottles struct {
	mu   sync.Mutex
	data map[string]ThrottleMarker
}

func newMemThrottles() *memThrottles {
	return &memThrottles{data: make(map[string]ThrottleMarker)}
}

func (t *memThrottles) Load(providerID string) *ThrottleMarker {
	t.mu.Lock()
	defer t.mu.Unlock()
	m, ok := t.data[providerID]
	if !ok {
		return nil
	}
	if time.Now().After(m.RetryAt) {
		delete(t.data, providerID)
		return nil
	}
	return &m
}

func (t *memThrottles) Save(providerID string, marker ThrottleMarker) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data[providerID] = marker
	return nil
}

func (t *memThrottles) Clear(providerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.data, providerID)
}

func defaultTestPipelineCfg() PipelineConfig {
	return PipelineConfig{
		Timeout: 30 * time.Second,

		Cache: newMemCache(),
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
		Timeout: 50 * time.Millisecond,

		Cache: newMemCache(),
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
		Timeout: 50 * time.Millisecond,

		Cache: newMemCache(),
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
		Timeout: 30 * time.Second,

		Cache: cache,
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

func TestExecutePipeline_FreshCacheHitSkipsFetch(t *testing.T) {
	cache := newMemCache()
	cached := models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-500 * time.Millisecond).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}
	cache.data["test-provider"] = cached

	cfg := PipelineConfig{
		Timeout:       30 * time.Second,
		Cache:         cache,
		FreshCacheTTL: time.Second,
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 99)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success from fresh cache reuse, got error: %s", outcome.Error)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true for fresh cache reuse")
	}
	if outcome.Source != "cache" {
		t.Errorf("expected source %q, got %q", "cache", outcome.Source)
	}
	if fetchCalled {
		t.Error("expected fresh cache hit to skip live fetch")
	}
	if outcome.Snapshot == nil {
		t.Fatal("expected cached snapshot")
	}
	if !outcome.Snapshot.FetchedAt.Equal(cached.FetchedAt) {
		t.Errorf("cached snapshot timestamp changed: got %v want %v", outcome.Snapshot.FetchedAt, cached.FetchedAt)
	}
}

func TestExecutePipeline_FreshCacheMissFallsThroughToFetch(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-2 * time.Second).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}

	cfg := PipelineConfig{
		Timeout:       30 * time.Second,
		Cache:         cache,
		FreshCacheTTL: time.Second,
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 55)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected live fetch success, got error: %s", outcome.Error)
	}
	if outcome.Cached {
		t.Error("expected stale cache to fall through to live fetch")
	}
	if !fetchCalled {
		t.Error("expected stale cache miss to call live fetch")
	}
	if outcome.Source != "mock" {
		t.Errorf("expected source %q, got %q", "mock", outcome.Source)
	}
}

func TestExecutePipeline_FreshCacheDisabledWithNoCacheFlag(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-500 * time.Millisecond).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
	}

	cfg := PipelineConfig{
		Timeout:       30 * time.Second,
		Cache:         cache,
		FreshCacheTTL: time.Second,
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 65)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected live fetch success with cache disabled, got error: %s", outcome.Error)
	}
	if outcome.Cached {
		t.Error("expected Cached=false when cache is disabled")
	}
	if !fetchCalled {
		t.Error("expected --no-cache behavior to bypass fresh cache reuse")
	}
}

func TestExecutePipeline_FreshCacheRejectsZeroFetchedAt(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider: "test-provider",
		Periods:  []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
	}

	cfg := PipelineConfig{
		Timeout:       30 * time.Second,
		Cache:         cache,
		FreshCacheTTL: time.Second,
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 70)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected live fetch success, got error: %s", outcome.Error)
	}
	if outcome.Cached {
		t.Error("expected zero-timestamp cache entry to be ignored for fresh reuse")
	}
	if !fetchCalled {
		t.Error("expected zero-timestamp cache entry to be treated as stale")
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
		Timeout: 30 * time.Second,

		Cache: cache,
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
		Timeout: 30 * time.Second,

		Cache: cache,
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
		Timeout: 30 * time.Second,

		Cache: newMemCache(),
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
		Timeout: 30 * time.Second,

		Cache: cache,
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
		Timeout: 30 * time.Second,

		Cache: cache,
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

func TestExecutePipeline_NoCacheFallbackWhenNotConfigured(t *testing.T) {
	// When no strategies are available (unconfigured provider), cache should NOT be served
	// even if the snapshot is fresh. This prevents misleading users with stale or
	// recently cached data after credentials disappear.
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-500 * time.Millisecond).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
	}

	cfg := PipelineConfig{
		Timeout:       5 * time.Second,
		Cache:         cache,
		FreshCacheTTL: time.Second,
	}

	strategy := &mockStrategy{
		available: false,
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true, cfg)

	if outcome.Success {
		t.Fatal("expected failure when no strategies available, got success")
	}
	if outcome.Error != "No strategies available" {
		t.Errorf("expected 'No strategies available', got: %s", outcome.Error)
	}
}

func TestExecutePipeline_NilCacheNoFallback(t *testing.T) {
	cfg := PipelineConfig{
		Timeout: 5 * time.Second,

		Cache: nil,
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

func TestExecutePipeline_ThrottleMarkerServesCacheWithoutFetch(t *testing.T) {
	cache := newMemCache()
	cache.data["test-provider"] = models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-5 * time.Minute).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 42}},
		Source:    "previous-fetch",
	}

	throttles := newMemThrottles()
	throttles.data["test-provider"] = ThrottleMarker{
		RetryAt: time.Now().Add(10 * time.Minute),
		Reason:  "Rate limited",
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 80)), nil
		},
	}

	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     cache,
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected cache hit during throttle window, got error: %s", outcome.Error)
	}
	if outcome.Source != "cache (throttled)" {
		t.Errorf("expected source %q, got %q", "cache (throttled)", outcome.Source)
	}
	if !outcome.Cached {
		t.Error("expected Cached=true")
	}
	if fetchCalled {
		t.Error("expected throttle marker to skip live fetch")
	}
}

func TestExecutePipeline_ThrottleMarkerNoCacheReturnsError(t *testing.T) {
	throttles := newMemThrottles()
	throttles.data["test-provider"] = ThrottleMarker{
		RetryAt: time.Now().Add(10 * time.Minute),
		Reason:  "Rate limited by Anthropic",
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 80)), nil
		},
	}

	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     newMemCache(),
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, true, cfg)

	if outcome.Success {
		t.Fatal("expected failure when throttled with no cache")
	}
	if fetchCalled {
		t.Error("expected throttle marker to skip live fetch even without cache")
	}
	if !strings.Contains(outcome.Error, "Rate limited by Anthropic") {
		t.Errorf("expected rate-limit reason in error, got %q", outcome.Error)
	}
	if !strings.Contains(outcome.Error, "retry after") {
		t.Errorf("expected retry-after hint in error, got %q", outcome.Error)
	}
}

func TestExecutePipeline_ThrottleMarkerBypassedWithNoCacheFlag(t *testing.T) {
	throttles := newMemThrottles()
	throttles.data["test-provider"] = ThrottleMarker{
		RetryAt: time.Now().Add(10 * time.Minute),
		Reason:  "Rate limited",
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 12)), nil
		},
	}

	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     newMemCache(),
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected --no-cache to bypass throttle and call strategy, got error: %s", outcome.Error)
	}
	if !fetchCalled {
		t.Error("expected --no-cache to bypass throttle check")
	}
}

func TestExecutePipeline_RetryAfterPersisted(t *testing.T) {
	retryAt := time.Now().Add(2 * time.Minute).UTC()

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultThrottled("Rate limited by Anthropic", retryAt), nil
		},
	}

	throttles := newMemThrottles()
	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     newMemCache(),
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, false, cfg)

	if outcome.Success {
		t.Error("expected non-success outcome when strategy returned throttled result without cache")
	}
	saved, ok := throttles.data["test-provider"]
	if !ok {
		t.Fatal("expected throttle marker to be persisted after 429 result")
	}
	if !saved.RetryAt.Equal(retryAt) {
		t.Errorf("persisted RetryAt = %v, want %v", saved.RetryAt, retryAt)
	}
	if saved.Reason != "Rate limited by Anthropic" {
		t.Errorf("persisted Reason = %q, want %q", saved.Reason, "Rate limited by Anthropic")
	}
}

func TestExecutePipeline_SuccessClearsThrottleMarker(t *testing.T) {
	throttles := newMemThrottles()
	throttles.data["test-provider"] = ThrottleMarker{
		RetryAt: time.Now().Add(-1 * time.Second),
		Reason:  "stale marker",
	}

	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(testSnapshot("test-provider", "mock", 33)), nil
		},
	}

	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     newMemCache(),
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, false, cfg)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if _, ok := throttles.data["test-provider"]; ok {
		t.Error("expected throttle marker to be cleared on success")
	}
}

func TestExecutePipeline_ExpiredThrottleMarkerIgnored(t *testing.T) {
	throttles := newMemThrottles()
	throttles.data["test-provider"] = ThrottleMarker{
		RetryAt: time.Now().Add(-1 * time.Minute),
		Reason:  "stale",
	}

	fetchCalled := false
	strategy := &mockStrategy{
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			fetchCalled = true
			return ResultOK(testSnapshot("test-provider", "mock", 44)), nil
		},
	}

	cfg := PipelineConfig{
		Timeout:   30 * time.Second,
		Cache:     newMemCache(),
		Throttles: throttles,
	}

	outcome := ExecutePipeline(context.Background(), "test-provider", []Strategy{strategy}, true, cfg)

	if !outcome.Success {
		t.Fatalf("expected success once throttle window passed, got error: %s", outcome.Error)
	}
	if !fetchCalled {
		t.Error("expected live fetch once throttle window expired")
	}
}
