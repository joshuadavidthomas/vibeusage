package config

import (
	"sync"
	"testing"
)

func TestGetAndReload_NoConcurrentRace(t *testing.T) {
	// This test verifies that concurrent Get() and Reload() calls
	// do not race. Run with -race to detect issues.
	var wg sync.WaitGroup

	// Warm up
	_ = Get()

	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = Get()
		}()
		go func() {
			defer wg.Done()
			_ = Reload()
		}()
	}

	wg.Wait()
}

func TestGet_ReturnsCopy(t *testing.T) {
	cfg := Get()
	// Mutating the returned value should not affect the global config.
	cfg.Fetch.Timeout = 999
	cfg2 := Get()
	if cfg2.Fetch.Timeout == 999 {
		t.Error("Get() should return a copy, not a reference to global state")
	}
}

func TestReload_ReturnsCurrentConfig(t *testing.T) {
	cfg := Reload()
	if cfg.Fetch.Timeout <= 0 {
		t.Error("Reload() should return a valid config with positive timeout")
	}
}
