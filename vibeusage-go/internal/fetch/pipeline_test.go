package fetch

import (
	"context"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// mockStrategy implements Strategy for testing.
type mockStrategy struct {
	name      string
	available bool
	fetchFn   func(ctx context.Context) (FetchResult, error)
}

func (m *mockStrategy) Name() string       { return m.name }
func (m *mockStrategy) IsAvailable() bool   { return m.available }
func (m *mockStrategy) Fetch(ctx context.Context) (FetchResult, error) {
	return m.fetchFn(ctx)
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
