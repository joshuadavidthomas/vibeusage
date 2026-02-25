package config

import "testing"

// Override sets the global config to cfg for the duration of the test,
// restoring the previous value on cleanup. This avoids the need to write
// config files to disk and call Init in tests.
func Override(t testing.TB, cfg Config) {
	t.Helper()
	configMu.RLock()
	prev := globalConfig
	configMu.RUnlock()

	set(cfg)
	t.Cleanup(func() {
		configMu.Lock()
		defer configMu.Unlock()
		globalConfig = prev
	})
}
