package fetch

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func defaultTestOrchestratorCfg() OrchestratorConfig {
	return OrchestratorConfig{
		MaxConcurrent: 5,
		Pipeline:      defaultTestPipelineCfg(),
	}
}

func TestFetchAllProviders_MultipleProviders(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	providerMap := map[string][]Strategy{
		"provider-a": {
			&mockStrategy{
				name:      "strategy-a",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("provider-a", "strategy-a", 40)), nil
				},
			},
		},
		"provider-b": {
			&mockStrategy{
				name:      "strategy-b",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("provider-b", "strategy-b", 60)), nil
				},
			},
		},
		"provider-c": {
			&mockStrategy{
				name:      "strategy-c",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("provider-c", "strategy-c", 80)), nil
				},
			},
		},
	}

	ctx := context.Background()
	outcomes := FetchAllProviders(ctx, providerMap, false, cfg, nil)

	if len(outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(outcomes))
	}
	for _, pid := range []string{"provider-a", "provider-b", "provider-c"} {
		o, ok := outcomes[pid]
		if !ok {
			t.Errorf("missing outcome for %q", pid)
			continue
		}
		if !o.Success {
			t.Errorf("%s: expected success, got error: %s", pid, o.Error)
		}
		if o.ProviderID != pid {
			t.Errorf("%s: ProviderID = %q, want %q", pid, o.ProviderID, pid)
		}
	}
}

func TestFetchAllProviders_EmptyMap(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	ctx := context.Background()
	outcomes := FetchAllProviders(ctx, map[string][]Strategy{}, false, cfg, nil)

	if len(outcomes) != 0 {
		t.Errorf("expected 0 outcomes, got %d", len(outcomes))
	}
}

func TestFetchAllProviders_NilMap(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	ctx := context.Background()
	outcomes := FetchAllProviders(ctx, nil, false, cfg, nil)

	if len(outcomes) != 0 {
		t.Errorf("expected 0 outcomes, got %d", len(outcomes))
	}
}

func TestFetchAllProviders_MixedResults(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	providerMap := map[string][]Strategy{
		"succeeder": {
			&mockStrategy{
				name:      "good",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("succeeder", "good", 50)), nil
				},
			},
		},
		"failer": {
			&mockStrategy{
				name:      "bad",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultFatal("credentials revoked"), nil
				},
			},
		},
	}

	ctx := context.Background()
	outcomes := FetchAllProviders(ctx, providerMap, false, cfg, nil)

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}

	if !outcomes["succeeder"].Success {
		t.Errorf("succeeder: expected success, got error: %s", outcomes["succeeder"].Error)
	}
	if outcomes["failer"].Success {
		t.Error("failer: expected failure")
	}
	if outcomes["failer"].Error != "credentials revoked" {
		t.Errorf("failer error = %q, want %q", outcomes["failer"].Error, "credentials revoked")
	}
}

func TestFetchAllProviders_OnCompleteCallback(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	providerMap := map[string][]Strategy{
		"alpha": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("alpha", "s", 10)), nil
				},
			},
		},
		"beta": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("beta", "s", 20)), nil
				},
			},
		},
	}

	var mu sync.Mutex
	var completedProviders []string

	ctx := context.Background()
	FetchAllProviders(ctx, providerMap, false, cfg, func(outcome FetchOutcome) {
		mu.Lock()
		defer mu.Unlock()
		completedProviders = append(completedProviders, outcome.ProviderID)
	})

	mu.Lock()
	defer mu.Unlock()

	if len(completedProviders) != 2 {
		t.Fatalf("expected 2 onComplete calls, got %d", len(completedProviders))
	}

	// Both providers should appear (order may vary due to concurrency)
	seen := make(map[string]bool)
	for _, pid := range completedProviders {
		seen[pid] = true
	}
	for _, want := range []string{"alpha", "beta"} {
		if !seen[want] {
			t.Errorf("onComplete not called for %q", want)
		}
	}
}

func TestFetchAllProviders_ContextCancellation(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()

	providerMap := map[string][]Strategy{
		"blocking": {
			&mockStrategy{
				name:      "blocker",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					<-ctx.Done()
					return ResultFail("cancelled"), ctx.Err()
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	outcomes := FetchAllProviders(ctx, providerMap, false, cfg, nil)

	o, ok := outcomes["blocking"]
	if !ok {
		t.Fatal("missing outcome for 'blocking'")
	}
	if o.Success {
		t.Error("expected failure for cancelled context")
	}
}

func TestFetchAllProviders_ConcurrencyLimit(t *testing.T) {
	cfg := OrchestratorConfig{
		MaxConcurrent: 2,
		Pipeline:      defaultTestPipelineCfg(),
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
					// Track maximum observed concurrency
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(50 * time.Millisecond)
					concurrentCount.Add(-1)
					return ResultOK(testSnapshot(pid, "s", 10)), nil
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
	outcomes := FetchAllProviders(ctx, providerMap, false, cfg, nil)

	if len(outcomes) != 5 {
		t.Fatalf("expected 5 outcomes, got %d", len(outcomes))
	}
	for pid, o := range outcomes {
		if !o.Success {
			t.Errorf("%s: expected success", pid)
		}
	}

	observed := maxConcurrent.Load()
	if observed > 2 {
		t.Errorf("max concurrent = %d, expected <= 2 (config limit)", observed)
	}
}

// FetchEnabledProviders tests

func TestFetchEnabledProviders_FiltersDisabled(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()
	isEnabled := func(id string) bool { return id == "alpha" || id == "gamma" }

	providerMap := map[string][]Strategy{
		"alpha": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("alpha", "s", 10)), nil
				},
			},
		},
		"beta": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					t.Error("beta should not be fetched — it's not enabled")
					return ResultOK(testSnapshot("beta", "s", 20)), nil
				},
			},
		},
		"gamma": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("gamma", "s", 30)), nil
				},
			},
		},
	}

	ctx := context.Background()
	outcomes := FetchEnabledProviders(ctx, providerMap, false, cfg, isEnabled, nil)

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes (only enabled), got %d", len(outcomes))
	}
	if _, ok := outcomes["alpha"]; !ok {
		t.Error("expected outcome for 'alpha'")
	}
	if _, ok := outcomes["beta"]; ok {
		t.Error("should not have outcome for disabled 'beta'")
	}
	if _, ok := outcomes["gamma"]; !ok {
		t.Error("expected outcome for 'gamma'")
	}
}

func TestFetchEnabledProviders_AllEnabledByDefault(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()
	allEnabled := func(string) bool { return true }

	providerMap := map[string][]Strategy{
		"alpha": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("alpha", "s", 10)), nil
				},
			},
		},
		"beta": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("beta", "s", 20)), nil
				},
			},
		},
	}

	ctx := context.Background()
	outcomes := FetchEnabledProviders(ctx, providerMap, false, cfg, allEnabled, nil)

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes (all enabled by default), got %d", len(outcomes))
	}
}

func TestFetchEnabledProviders_WithProviderConfigDisabled(t *testing.T) {
	cfg := defaultTestOrchestratorCfg()
	isEnabled := func(id string) bool { return id != "beta" }

	providerMap := map[string][]Strategy{
		"alpha": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					return ResultOK(testSnapshot("alpha", "s", 10)), nil
				},
			},
		},
		"beta": {
			&mockStrategy{
				name:      "s",
				available: true,
				fetchFn: func(ctx context.Context) (FetchResult, error) {
					t.Error("beta should not be fetched — disabled in config")
					return ResultOK(models.UsageSnapshot{}), nil
				},
			},
		},
	}

	ctx := context.Background()
	outcomes := FetchEnabledProviders(ctx, providerMap, false, cfg, isEnabled, nil)

	if _, ok := outcomes["beta"]; ok {
		t.Error("should not have outcome for disabled 'beta'")
	}
	if _, ok := outcomes["alpha"]; !ok {
		t.Error("expected outcome for 'alpha'")
	}
}
