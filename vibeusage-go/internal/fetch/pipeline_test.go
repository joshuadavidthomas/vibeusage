package fetch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// mockStrategy implements Strategy for testing.
type mockStrategy struct {
	name      string
	available bool
	fetchFn   func(ctx context.Context) (FetchResult, error)
}

func (m *mockStrategy) Name() string                                    { return m.name }
func (m *mockStrategy) IsAvailable() bool                               { return m.available }
func (m *mockStrategy) Fetch(ctx context.Context) (FetchResult, error) { return m.fetchFn(ctx) }

// setupFetchTestEnv redirects config and cache to a temp directory,
// reloads config (which picks up defaults), and ensures cleanup.
func setupFetchTestEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("VIBEUSAGE_CACHE_DIR", filepath.Join(dir, "cache"))
	t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", "")
	t.Setenv("VIBEUSAGE_NO_COLOR", "")
	config.Reload()
}

// setupFetchTestEnvWithConfig sets up a temp environment and writes a
// custom config file before reloading.
func setupFetchTestEnvWithConfig(t *testing.T, modify func(*config.Config)) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("VIBEUSAGE_CACHE_DIR", filepath.Join(dir, "cache"))
	t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", "")
	t.Setenv("VIBEUSAGE_NO_COLOR", "")

	cfg := config.DefaultConfig()
	modify(&cfg)
	if err := config.Save(cfg, config.ConfigFile()); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}
	config.Reload()
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
	// A strategy that blocks until context is cancelled
	strategy := &mockStrategy{
		name:      "blocking",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			<-ctx.Done()
			return ResultFail("cancelled"), ctx.Err()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)
	if outcome.Success {
		t.Error("expected failure for cancelled context")
	}
	if outcome.Error != "Context cancelled" {
		t.Errorf("expected 'Context cancelled' error, got: %s", outcome.Error)
	}
}

func TestExecutePipeline_ContextPassedToStrategy(t *testing.T) {
	// Verify the context is actually passed to strategy.Fetch
	var receivedCtx context.Context

	strategy := &mockStrategy{
		name:      "ctx-checker",
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

	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)
	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if receivedCtx == nil {
		t.Fatal("strategy did not receive a context")
	}
	// The strategy should receive the parent context (or a derived one)
	// Verify it's not a bare background context by checking the cancel works
	cancel()
	select {
	case <-receivedCtx.Done():
		// Expected - context was properly derived from the parent
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
		name:      "mock",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(snap), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)
	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if outcome.Source != "mock" {
		t.Errorf("expected source 'mock', got '%s'", outcome.Source)
	}
}

func TestExecutePipeline_FallbackToSecondStrategy(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "test",
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "daily", Utilization: 10}},
		Source:    "fallback",
	}

	strategies := []Strategy{
		&mockStrategy{
			name:      "primary",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("primary failed"), nil
			},
		},
		&mockStrategy{
			name:      "fallback",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)
	if !outcome.Success {
		t.Fatalf("expected success from fallback, got error: %s", outcome.Error)
	}
	if outcome.Source != "fallback" {
		t.Errorf("expected source 'fallback', got '%s'", outcome.Source)
	}
	if len(outcome.Attempts) < 1 {
		t.Error("expected at least 1 attempt recorded")
	}
}

func TestExecutePipeline_FatalStopsChain(t *testing.T) {
	fallbackCalled := false
	strategies := []Strategy{
		&mockStrategy{
			name:      "fatal",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFatal("token expired"), nil
			},
		},
		&mockStrategy{
			name:      "fallback",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				fallbackCalled = true
				return ResultOK(models.UsageSnapshot{}), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)
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
		name:      "unavailable",
		available: false,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			t.Fatal("should not call Fetch on unavailable strategy")
			return FetchResult{}, nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)
	if outcome.Success {
		t.Error("expected failure when only strategy is unavailable")
	}
	if len(outcome.Attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", len(outcome.Attempts))
	}
	if outcome.Attempts[0].Error != "Strategy not available" {
		t.Errorf("expected 'Strategy not available', got '%s'", outcome.Attempts[0].Error)
	}
}

func TestExecutePipeline_EmptyStrategies(t *testing.T) {
	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", nil, false)
	if outcome.Success {
		t.Error("expected failure with no strategies")
	}
	if outcome.Error != "No strategies available" {
		t.Errorf("expected 'No strategies available', got '%s'", outcome.Error)
	}
}

// Timeout tests

func TestExecutePipeline_Timeout(t *testing.T) {
	setupFetchTestEnvWithConfig(t, func(cfg *config.Config) {
		cfg.Fetch.Timeout = 0.05 // 50ms
	})

	strategy := &mockStrategy{
		name:      "slow",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			time.Sleep(500 * time.Millisecond)
			return ResultOK(testSnapshot("test", "slow", 50)), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)

	if outcome.Success {
		t.Error("expected failure due to timeout")
	}
	if len(outcome.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(outcome.Attempts))
	}
	if outcome.Attempts[0].Strategy != "slow" {
		t.Errorf("attempt strategy = %q, want %q", outcome.Attempts[0].Strategy, "slow")
	}
	if outcome.Attempts[0].Error != "Fetch timed out" {
		t.Errorf("attempt error = %q, want %q", outcome.Attempts[0].Error, "Fetch timed out")
	}
	if outcome.Attempts[0].Success {
		t.Error("timed out attempt should not be marked as success")
	}
}

func TestExecutePipeline_TimeoutFallsBackToNextStrategy(t *testing.T) {
	setupFetchTestEnvWithConfig(t, func(cfg *config.Config) {
		cfg.Fetch.Timeout = 0.05 // 50ms
	})

	snap := testSnapshot("test", "fast", 42)
	strategies := []Strategy{
		&mockStrategy{
			name:      "slow",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				time.Sleep(500 * time.Millisecond)
				return ResultOK(testSnapshot("test", "slow", 0)), nil
			},
		},
		&mockStrategy{
			name:      "fast",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if !outcome.Success {
		t.Fatalf("expected success from fast strategy, got error: %s", outcome.Error)
	}
	if outcome.Source != "fast" {
		t.Errorf("expected source %q, got %q", "fast", outcome.Source)
	}
	if len(outcome.Attempts) < 1 {
		t.Fatal("expected at least 1 recorded attempt")
	}
	if outcome.Attempts[0].Error != "Fetch timed out" {
		t.Errorf("first attempt error = %q, want %q", outcome.Attempts[0].Error, "Fetch timed out")
	}
}

// Go error from Fetch()

func TestExecutePipeline_FetchReturnsGoError(t *testing.T) {
	setupFetchTestEnv(t)

	strategy := &mockStrategy{
		name:      "erroring",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return FetchResult{}, fmt.Errorf("connection refused")
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)

	if outcome.Success {
		t.Error("expected failure when strategy returns Go error")
	}
	if len(outcome.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(outcome.Attempts))
	}
	if outcome.Attempts[0].Error != "connection refused" {
		t.Errorf("attempt error = %q, want %q", outcome.Attempts[0].Error, "connection refused")
	}
	if outcome.Error != "connection refused" {
		t.Errorf("outcome error = %q, want %q", outcome.Error, "connection refused")
	}
}

func TestExecutePipeline_GoErrorFallsBackToNextStrategy(t *testing.T) {
	setupFetchTestEnv(t)

	snap := testSnapshot("test", "backup", 25)
	strategies := []Strategy{
		&mockStrategy{
			name:      "broken",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return FetchResult{}, fmt.Errorf("DNS resolution failed")
			},
		},
		&mockStrategy{
			name:      "backup",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if !outcome.Success {
		t.Fatalf("expected success from backup, got error: %s", outcome.Error)
	}
	if outcome.Source != "backup" {
		t.Errorf("expected source %q, got %q", "backup", outcome.Source)
	}
}

// Cache fallback tests

func TestExecutePipeline_CacheFallback(t *testing.T) {
	setupFetchTestEnv(t)

	// Pre-populate cache
	cachedSnap := models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().Add(-1 * time.Hour).UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 30}},
		Source:    "previous-fetch",
	}
	if err := config.CacheSnapshot(cachedSnap); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	strategy := &mockStrategy{
		name:      "failing",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true)

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

func TestExecutePipeline_CacheFallbackNoData(t *testing.T) {
	setupFetchTestEnv(t)
	// No cache pre-populated

	strategy := &mockStrategy{
		name:      "failing",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, true)

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
	setupFetchTestEnv(t)

	// Pre-populate cache (should NOT be used because useCache=false)
	cachedSnap := models.UsageSnapshot{
		Provider:  "test-provider",
		FetchedAt: time.Now().UTC(),
		Periods:   []models.UsagePeriod{{Name: "monthly", Utilization: 10}},
	}
	if err := config.CacheSnapshot(cachedSnap); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	strategy := &mockStrategy{
		name:      "failing",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultFail("API error"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)

	if outcome.Success {
		t.Error("expected failure when cache is disabled")
	}
	if outcome.Cached {
		t.Error("Cached should be false when cache is disabled")
	}
}

// Multi-strategy chain

func TestExecutePipeline_ThreeStrategyChain(t *testing.T) {
	setupFetchTestEnv(t)

	snap := testSnapshot("test", "third", 75)
	strategies := []Strategy{
		&mockStrategy{
			name:      "first",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("first failed"), nil
			},
		},
		&mockStrategy{
			name:      "second",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("second failed"), nil
			},
		},
		&mockStrategy{
			name:      "third",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if !outcome.Success {
		t.Fatalf("expected success from third strategy, got error: %s", outcome.Error)
	}
	if outcome.Source != "third" {
		t.Errorf("expected source %q, got %q", "third", outcome.Source)
	}
	// Two failed attempts should be recorded (third succeeded, not in Attempts)
	if len(outcome.Attempts) != 2 {
		t.Errorf("expected 2 recorded attempts, got %d", len(outcome.Attempts))
	}
	if len(outcome.Attempts) >= 1 && outcome.Attempts[0].Strategy != "first" {
		t.Errorf("first attempt strategy = %q, want %q", outcome.Attempts[0].Strategy, "first")
	}
	if len(outcome.Attempts) >= 2 && outcome.Attempts[1].Strategy != "second" {
		t.Errorf("second attempt strategy = %q, want %q", outcome.Attempts[1].Strategy, "second")
	}
}

// Attempt tracking

func TestExecutePipeline_AttemptsRecordErrors(t *testing.T) {
	setupFetchTestEnv(t)

	strategies := []Strategy{
		&mockStrategy{
			name:      "unavail",
			available: false,
			fetchFn:   func(ctx context.Context) (FetchResult, error) { return FetchResult{}, nil },
		},
		&mockStrategy{
			name:      "failing",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultFail("auth expired"), nil
			},
		},
		&mockStrategy{
			name:      "erroring",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return FetchResult{}, fmt.Errorf("network timeout")
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if outcome.Success {
		t.Error("expected failure")
	}
	if len(outcome.Attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(outcome.Attempts))
	}

	// Attempt 0: unavailable
	if outcome.Attempts[0].Strategy != "unavail" {
		t.Errorf("attempt[0] strategy = %q, want %q", outcome.Attempts[0].Strategy, "unavail")
	}
	if outcome.Attempts[0].Error != "Strategy not available" {
		t.Errorf("attempt[0] error = %q, want %q", outcome.Attempts[0].Error, "Strategy not available")
	}

	// Attempt 1: ResultFail
	if outcome.Attempts[1].Strategy != "failing" {
		t.Errorf("attempt[1] strategy = %q, want %q", outcome.Attempts[1].Strategy, "failing")
	}
	if outcome.Attempts[1].Error != "auth expired" {
		t.Errorf("attempt[1] error = %q, want %q", outcome.Attempts[1].Error, "auth expired")
	}

	// Attempt 2: Go error
	if outcome.Attempts[2].Strategy != "erroring" {
		t.Errorf("attempt[2] strategy = %q, want %q", outcome.Attempts[2].Strategy, "erroring")
	}
	if outcome.Attempts[2].Error != "network timeout" {
		t.Errorf("attempt[2] error = %q, want %q", outcome.Attempts[2].Error, "network timeout")
	}

	// Final error should be the last attempt's error
	if outcome.Error != "network timeout" {
		t.Errorf("outcome error = %q, want %q (last attempt)", outcome.Error, "network timeout")
	}
}

func TestExecutePipeline_AttemptsDurationTracked(t *testing.T) {
	setupFetchTestEnv(t)

	strategy := &mockStrategy{
		name:      "sleeper",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			time.Sleep(10 * time.Millisecond)
			return ResultFail("intentional failure"), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", []Strategy{strategy}, false)

	if len(outcome.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(outcome.Attempts))
	}
	if outcome.Attempts[0].DurationMs < 1 {
		t.Errorf("DurationMs = %d, expected > 0 for a strategy that slept", outcome.Attempts[0].DurationMs)
	}
}

// Success behavior

func TestExecutePipeline_SuccessCachesResult(t *testing.T) {
	setupFetchTestEnv(t)

	snap := testSnapshot("cache-test-provider", "mock", 60)
	strategy := &mockStrategy{
		name:      "mock",
		available: true,
		fetchFn: func(ctx context.Context) (FetchResult, error) {
			return ResultOK(snap), nil
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "cache-test-provider", []Strategy{strategy}, false)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}

	// Verify the snapshot was cached
	cached := config.LoadCachedSnapshot("cache-test-provider")
	if cached == nil {
		t.Fatal("expected snapshot to be cached after successful fetch")
	}
	if cached.Provider != "cache-test-provider" {
		t.Errorf("cached provider = %q, want %q", cached.Provider, "cache-test-provider")
	}
}

func TestExecutePipeline_SuccessStopsChain(t *testing.T) {
	setupFetchTestEnv(t)

	secondCalled := false
	strategies := []Strategy{
		&mockStrategy{
			name:      "fast-success",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(testSnapshot("test", "fast-success", 20)), nil
			},
		},
		&mockStrategy{
			name:      "never-reached",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				secondCalled = true
				return ResultOK(testSnapshot("test", "never-reached", 0)), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Source != "fast-success" {
		t.Errorf("expected source %q, got %q", "fast-success", outcome.Source)
	}
	if secondCalled {
		t.Error("second strategy should not be called when first succeeds")
	}
}

// ProviderID propagation

func TestExecutePipeline_ProviderIDInOutcome(t *testing.T) {
	setupFetchTestEnv(t)

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
					name:      "mock",
					available: true,
					fetchFn: func(ctx context.Context) (FetchResult, error) {
						return ResultOK(testSnapshot(tt.providerID, "mock", 50)), nil
					},
				}
			} else {
				strategy = &mockStrategy{
					name:      "mock",
					available: true,
					fetchFn: func(ctx context.Context) (FetchResult, error) {
						return ResultFail("error"), nil
					},
				}
			}

			ctx := context.Background()
			outcome := ExecutePipeline(ctx, tt.providerID, []Strategy{strategy}, false)

			if outcome.ProviderID != tt.providerID {
				t.Errorf("ProviderID = %q, want %q", outcome.ProviderID, tt.providerID)
			}
		})
	}
}

// Mixed unavailable and available strategies

func TestExecutePipeline_SkipsUnavailableTriesAvailable(t *testing.T) {
	setupFetchTestEnv(t)

	snap := testSnapshot("test", "available", 55)
	strategies := []Strategy{
		&mockStrategy{
			name:      "unavailable-1",
			available: false,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				t.Fatal("should not call Fetch on unavailable strategy")
				return FetchResult{}, nil
			},
		},
		&mockStrategy{
			name:      "unavailable-2",
			available: false,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				t.Fatal("should not call Fetch on unavailable strategy")
				return FetchResult{}, nil
			},
		},
		&mockStrategy{
			name:      "available",
			available: true,
			fetchFn: func(ctx context.Context) (FetchResult, error) {
				return ResultOK(snap), nil
			},
		},
	}

	ctx := context.Background()
	outcome := ExecutePipeline(ctx, "test-provider", strategies, false)

	if !outcome.Success {
		t.Fatalf("expected success, got error: %s", outcome.Error)
	}
	if outcome.Source != "available" {
		t.Errorf("expected source %q, got %q", "available", outcome.Source)
	}
	// Two unavailable attempts should be recorded
	if len(outcome.Attempts) != 2 {
		t.Errorf("expected 2 attempts (for unavailable strategies), got %d", len(outcome.Attempts))
	}
}
