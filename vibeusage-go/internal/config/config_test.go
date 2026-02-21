package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// Helpers

func setupTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("VIBEUSAGE_CACHE_DIR", filepath.Join(dir, "cache"))
	// Clear env override variables so tests aren't affected by the host environment.
	t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", "")
	t.Setenv("VIBEUSAGE_NO_COLOR", "")
	// Reset global config so tests don't leak state.
	configMu.Lock()
	globalConfig = nil
	configMu.Unlock()
	return dir
}

// setupTempDirWithCredentialIsolation sets up temp dirs AND writes a config
// that disables ReuseProviderCredentials and clears all provider env vars.
// This prevents tests from detecting real CLI credentials on the developer machine.
func setupTempDirWithCredentialIsolation(t *testing.T) string {
	t.Helper()
	dir := setupTempDir(t)

	// Clear all provider env vars so env-based detection doesn't fire
	for _, envVar := range ProviderEnvVars {
		t.Setenv(envVar, "")
	}

	// Write a config that disables CLI credential reuse
	cfg := DefaultConfig()
	cfg.Credentials.ReuseProviderCredentials = false
	configPath := filepath.Join(dir, "config", "config.toml")
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Reset global config again so it picks up the new file
	configMu.Lock()
	globalConfig = nil
	configMu.Unlock()

	return dir
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// DefaultConfig

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.EnabledProviders != nil {
		t.Error("EnabledProviders should be nil (all enabled)")
	}
	if !cfg.Display.ShowRemaining {
		t.Error("Display.ShowRemaining should default to true")
	}
	if !cfg.Display.PaceColors {
		t.Error("Display.PaceColors should default to true")
	}
	if cfg.Display.ResetFormat != "countdown" {
		t.Errorf("Display.ResetFormat = %q, want %q", cfg.Display.ResetFormat, "countdown")
	}
	if cfg.Fetch.Timeout != 30.0 {
		t.Errorf("Fetch.Timeout = %v, want 30.0", cfg.Fetch.Timeout)
	}
	if cfg.Fetch.MaxConcurrent != 5 {
		t.Errorf("Fetch.MaxConcurrent = %d, want 5", cfg.Fetch.MaxConcurrent)
	}
	if cfg.Fetch.StaleThresholdMinutes != 60 {
		t.Errorf("Fetch.StaleThresholdMinutes = %d, want 60", cfg.Fetch.StaleThresholdMinutes)
	}
	if cfg.Credentials.UseKeyring {
		t.Error("Credentials.UseKeyring should default to false")
	}
	if !cfg.Credentials.ReuseProviderCredentials {
		t.Error("Credentials.ReuseProviderCredentials should default to true")
	}
	if cfg.Providers == nil {
		t.Error("Providers map should be initialized (non-nil)")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("Providers map should be empty, got %d entries", len(cfg.Providers))
	}
}

// IsProviderEnabled

func TestIsProviderEnabled(t *testing.T) {
	tests := []struct {
		name             string
		enabledProviders []string
		providers        map[string]ProviderConfig
		providerID       string
		want             bool
	}{
		{
			name:             "all enabled when EnabledProviders is nil",
			enabledProviders: nil,
			providers:        map[string]ProviderConfig{},
			providerID:       "claude",
			want:             true,
		},
		{
			name:             "all enabled when EnabledProviders is empty",
			enabledProviders: []string{},
			providers:        map[string]ProviderConfig{},
			providerID:       "claude",
			want:             true,
		},
		{
			name:             "provider in allowlist is enabled",
			enabledProviders: []string{"claude", "copilot"},
			providers:        map[string]ProviderConfig{},
			providerID:       "claude",
			want:             true,
		},
		{
			name:             "provider not in allowlist is disabled",
			enabledProviders: []string{"claude", "copilot"},
			providers:        map[string]ProviderConfig{},
			providerID:       "gemini",
			want:             false,
		},
		{
			name:             "explicit Enabled:false overrides allowlist",
			enabledProviders: []string{"claude", "copilot"},
			providers: map[string]ProviderConfig{
				"claude": {Enabled: false},
			},
			providerID: "claude",
			want:       false,
		},
		{
			name:             "explicit Enabled:false overrides nil EnabledProviders",
			enabledProviders: nil,
			providers: map[string]ProviderConfig{
				"claude": {Enabled: false},
			},
			providerID: "claude",
			want:       false,
		},
		{
			name:             "explicit Enabled:true with nil EnabledProviders is enabled",
			enabledProviders: nil,
			providers: map[string]ProviderConfig{
				"claude": {Enabled: true},
			},
			providerID: "claude",
			want:       true,
		},
		{
			name:             "provider not in Providers map and not in allowlist",
			enabledProviders: []string{"copilot"},
			providers:        map[string]ProviderConfig{},
			providerID:       "claude",
			want:             false,
		},
		{
			name:             "Enabled:true does NOT override allowlist exclusion",
			enabledProviders: []string{"copilot"},
			providers: map[string]ProviderConfig{
				"claude": {Enabled: true},
			},
			providerID: "claude",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				EnabledProviders: tt.enabledProviders,
				Providers:        tt.providers,
			}
			got := cfg.IsProviderEnabled(tt.providerID)
			if got != tt.want {
				t.Errorf("IsProviderEnabled(%q) = %v, want %v", tt.providerID, got, tt.want)
			}
		})
	}
}

// Load and Save

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	dir := setupTempDir(t)
	cfg := Load(filepath.Join(dir, "nonexistent.toml"))
	def := DefaultConfig()

	if cfg.Fetch.Timeout != def.Fetch.Timeout {
		t.Errorf("Fetch.Timeout = %v, want default %v", cfg.Fetch.Timeout, def.Fetch.Timeout)
	}
	if cfg.Providers == nil {
		t.Error("Providers map should be initialized even on missing file")
	}
}

func TestLoad_MalformedTOML_ReturnsDefaults(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "bad.toml")
	writeTestFile(t, path, []byte("this is not valid [[[toml"))

	cfg := Load(path)
	def := DefaultConfig()

	if cfg.Fetch.Timeout != def.Fetch.Timeout {
		t.Errorf("Fetch.Timeout = %v, want default %v", cfg.Fetch.Timeout, def.Fetch.Timeout)
	}
}

func TestLoad_PartialTOML_MergesWithDefaults(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "partial.toml")
	writeTestFile(t, path, []byte(`
[fetch]
timeout = 10.0
`))

	cfg := Load(path)

	if cfg.Fetch.Timeout != 10.0 {
		t.Errorf("Fetch.Timeout = %v, want 10.0 (from file)", cfg.Fetch.Timeout)
	}
	// Other fields should retain defaults
	if cfg.Fetch.MaxConcurrent != 5 {
		t.Errorf("Fetch.MaxConcurrent = %d, want 5 (default)", cfg.Fetch.MaxConcurrent)
	}
	if !cfg.Display.ShowRemaining {
		t.Error("Display.ShowRemaining should default to true when not in file")
	}
}

func TestLoad_InitializesNilProvidersMap(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "noproviders.toml")
	// TOML with no [providers] section at all
	writeTestFile(t, path, []byte(`
[fetch]
timeout = 15.0
`))

	cfg := Load(path)
	if cfg.Providers == nil {
		t.Error("Providers map should be initialized when not present in TOML")
	}
}

func TestSave_CreatesDirAndFile(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "sub", "dir", "config.toml")

	cfg := DefaultConfig()
	cfg.Fetch.Timeout = 42.0

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Save() did not create the file")
	}
}

func TestSave_Load_Roundtrip(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "roundtrip.toml")

	original := DefaultConfig()
	original.Fetch.Timeout = 99.0
	original.EnabledProviders = []string{"claude", "copilot"}
	original.Display.PaceColors = false
	original.Display.ResetFormat = "relative"
	original.Providers["claude"] = ProviderConfig{
		AuthSource: "oauth",
		Enabled:    true,
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded := Load(path)

	if loaded.Fetch.Timeout != 99.0 {
		t.Errorf("Fetch.Timeout = %v, want 99.0", loaded.Fetch.Timeout)
	}
	if len(loaded.EnabledProviders) != 2 {
		t.Fatalf("EnabledProviders len = %d, want 2", len(loaded.EnabledProviders))
	}
	if loaded.EnabledProviders[0] != "claude" || loaded.EnabledProviders[1] != "copilot" {
		t.Errorf("EnabledProviders = %v, want [claude copilot]", loaded.EnabledProviders)
	}
	if loaded.Display.PaceColors {
		t.Error("Display.PaceColors should be false after roundtrip")
	}
	if loaded.Display.ResetFormat != "relative" {
		t.Errorf("Display.ResetFormat = %q, want %q", loaded.Display.ResetFormat, "relative")
	}
	pc, ok := loaded.Providers["claude"]
	if !ok {
		t.Fatal("Providers[claude] not found after roundtrip")
	}
	if pc.AuthSource != "oauth" {
		t.Errorf("Providers[claude].AuthSource = %q, want %q", pc.AuthSource, "oauth")
	}
	if !pc.Enabled {
		t.Error("Providers[claude].Enabled should be true after roundtrip")
	}
}

// applyEnvOverrides

func TestApplyEnvOverrides_EnabledProviders(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantLen int
		wantIDs []string
	}{
		{
			name:    "single provider",
			envVal:  "claude",
			wantLen: 1,
			wantIDs: []string{"claude"},
		},
		{
			name:    "multiple providers",
			envVal:  "claude,copilot,gemini",
			wantLen: 3,
			wantIDs: []string{"claude", "copilot", "gemini"},
		},
		{
			name:    "trims whitespace",
			envVal:  " claude , copilot , gemini ",
			wantLen: 3,
			wantIDs: []string{"claude", "copilot", "gemini"},
		},
		{
			name:    "filters empty parts",
			envVal:  "claude,,copilot, ,",
			wantLen: 2,
			wantIDs: []string{"claude", "copilot"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", tt.envVal)
			cfg := applyEnvOverrides(DefaultConfig())
			if len(cfg.EnabledProviders) != tt.wantLen {
				t.Errorf("EnabledProviders len = %d, want %d", len(cfg.EnabledProviders), tt.wantLen)
			}
			for i, want := range tt.wantIDs {
				if i >= len(cfg.EnabledProviders) {
					break
				}
				if cfg.EnabledProviders[i] != want {
					t.Errorf("EnabledProviders[%d] = %q, want %q", i, cfg.EnabledProviders[i], want)
				}
			}
		})
	}
}

func TestApplyEnvOverrides_NoColor(t *testing.T) {
	t.Run("set disables pace colors", func(t *testing.T) {
		t.Setenv("VIBEUSAGE_NO_COLOR", "1")
		cfg := applyEnvOverrides(DefaultConfig())
		if cfg.Display.PaceColors {
			t.Error("PaceColors should be false when VIBEUSAGE_NO_COLOR is set")
		}
	})

	t.Run("unset leaves pace colors default", func(t *testing.T) {
		// t.Setenv not called → inherits test environment, but we clear it
		t.Setenv("VIBEUSAGE_NO_COLOR", "")
		cfg := applyEnvOverrides(DefaultConfig())
		if !cfg.Display.PaceColors {
			t.Error("PaceColors should remain true when VIBEUSAGE_NO_COLOR is empty")
		}
	})
}

func TestApplyEnvOverrides_NotSet_LeavesDefaults(t *testing.T) {
	// Clear both env vars explicitly.
	t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", "")
	t.Setenv("VIBEUSAGE_NO_COLOR", "")

	cfg := applyEnvOverrides(DefaultConfig())
	if cfg.EnabledProviders != nil {
		t.Error("EnabledProviders should remain nil when env is empty")
	}
	if !cfg.Display.PaceColors {
		t.Error("PaceColors should remain true when env is empty")
	}
}

func TestLoad_RespectsEnvOverrides(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "config.toml")
	writeTestFile(t, path, []byte(`
enabled_providers = ["claude", "copilot", "gemini"]
`))

	t.Setenv("VIBEUSAGE_ENABLED_PROVIDERS", "cursor")

	cfg := Load(path)
	if len(cfg.EnabledProviders) != 1 || cfg.EnabledProviders[0] != "cursor" {
		t.Errorf("Env override should take precedence. Got EnabledProviders = %v", cfg.EnabledProviders)
	}
}

// Get and Reload (existing tests plus new ones)

func TestGetAndReload_NoConcurrentRace(t *testing.T) {
	setupTempDir(t)
	var wg sync.WaitGroup
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
	setupTempDir(t)

	// Scalar field isolation
	cfg := Get()
	cfg.Fetch.Timeout = 999
	cfg2 := Get()
	if cfg2.Fetch.Timeout == 999 {
		t.Error("Get() should return a copy: scalar field mutation leaked")
	}

	// Map isolation: mutating Providers on one copy must not affect another
	cfg3 := Get()
	cfg3.Providers["injected"] = ProviderConfig{Enabled: true}
	cfg4 := Get()
	if _, ok := cfg4.Providers["injected"]; ok {
		t.Error("Get() should return a copy: Providers map mutation leaked")
	}

	// Slice isolation: mutating EnabledProviders must not affect another copy
	configMu.Lock()
	globalConfig = nil
	configMu.Unlock()

	dir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", filepath.Join(dir, "config"))
	configPath := filepath.Join(dir, "config", "config.toml")
	writeTestFile(t, configPath, []byte(`enabled_providers = ["claude", "copilot"]`))

	cfg5 := Get()
	if len(cfg5.EnabledProviders) != 2 {
		t.Fatalf("expected 2 enabled providers, got %d", len(cfg5.EnabledProviders))
	}
	cfg5.EnabledProviders[0] = "MUTATED"
	cfg6 := Get()
	if cfg6.EnabledProviders[0] == "MUTATED" {
		t.Error("Get() should return a copy: EnabledProviders slice mutation leaked")
	}
}

func TestReload_ReturnsCurrentConfig(t *testing.T) {
	setupTempDir(t)
	cfg := Reload()
	if cfg.Fetch.Timeout <= 0 {
		t.Error("Reload() should return a valid config with positive timeout")
	}
}

func TestGet_LoadsOnFirstCall(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "config", "config.toml")
	writeTestFile(t, path, []byte(`
[fetch]
timeout = 77.0
`))

	cfg := Get()
	if cfg.Fetch.Timeout != 77.0 {
		t.Errorf("Get() should load from file. Fetch.Timeout = %v, want 77.0", cfg.Fetch.Timeout)
	}
}

func TestReload_PicksUpChanges(t *testing.T) {
	dir := setupTempDir(t)
	path := filepath.Join(dir, "config", "config.toml")
	writeTestFile(t, path, []byte(`
[fetch]
timeout = 10.0
`))

	cfg1 := Get()
	if cfg1.Fetch.Timeout != 10.0 {
		t.Fatalf("initial Get() Fetch.Timeout = %v, want 10.0", cfg1.Fetch.Timeout)
	}

	writeTestFile(t, path, []byte(`
[fetch]
timeout = 20.0
`))

	cfg2 := Reload()
	if cfg2.Fetch.Timeout != 20.0 {
		t.Errorf("Reload() Fetch.Timeout = %v, want 20.0", cfg2.Fetch.Timeout)
	}
}

// Paths

func TestConfigDir_EnvOverride(t *testing.T) {
	t.Setenv("VIBEUSAGE_CONFIG_DIR", "/custom/config")
	got := ConfigDir()
	if got != "/custom/config" {
		t.Errorf("ConfigDir() = %q, want %q", got, "/custom/config")
	}
}

func TestCacheDir_EnvOverride(t *testing.T) {
	t.Setenv("VIBEUSAGE_CACHE_DIR", "/custom/cache")
	got := CacheDir()
	if got != "/custom/cache" {
		t.Errorf("CacheDir() = %q, want %q", got, "/custom/cache")
	}
}

func TestConfigDir_DefaultContainsVibeusage(t *testing.T) {
	t.Setenv("VIBEUSAGE_CONFIG_DIR", "")
	got := ConfigDir()
	if filepath.Base(got) != "vibeusage" {
		t.Errorf("ConfigDir() = %q, should end with 'vibeusage'", got)
	}
}

func TestCacheDir_DefaultContainsVibeusage(t *testing.T) {
	t.Setenv("VIBEUSAGE_CACHE_DIR", "")
	got := CacheDir()
	if filepath.Base(got) != "vibeusage" {
		t.Errorf("CacheDir() = %q, should end with 'vibeusage'", got)
	}
}

func TestSubdirectoryPaths(t *testing.T) {
	t.Setenv("VIBEUSAGE_CONFIG_DIR", "/base/config")
	t.Setenv("VIBEUSAGE_CACHE_DIR", "/base/cache")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"CredentialsDir", CredentialsDir(), "/base/config/credentials"},
		{"SnapshotsDir", SnapshotsDir(), "/base/cache/snapshots"},
		{"OrgIDsDir", OrgIDsDir(), "/base/cache/org-ids"},
		{"ConfigFile", ConfigFile(), "/base/config/config.toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// Cache: Snapshots

func TestCacheSnapshot_LoadCachedSnapshot_Roundtrip(t *testing.T) {
	setupTempDir(t)

	now := time.Now().Truncate(time.Millisecond) // JSON loses sub-ms precision
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods: []models.UsagePeriod{
			{
				Name:        "monthly",
				PeriodType:  models.PeriodMonthly,
				Utilization: 42,
			},
		},
		Source: "oauth",
	}

	if err := CacheSnapshot(snap); err != nil {
		t.Fatalf("CacheSnapshot() error: %v", err)
	}

	loaded := LoadCachedSnapshot("claude")
	if loaded == nil {
		t.Fatal("LoadCachedSnapshot() returned nil")
	}

	if loaded.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", loaded.Provider, "claude")
	}
	if !loaded.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want %v", loaded.FetchedAt, now)
	}
	if len(loaded.Periods) != 1 {
		t.Fatalf("Periods len = %d, want 1", len(loaded.Periods))
	}
	if loaded.Periods[0].Utilization != 42 {
		t.Errorf("Periods[0].Utilization = %d, want 42", loaded.Periods[0].Utilization)
	}
	if loaded.Source != "oauth" {
		t.Errorf("Source = %q, want %q", loaded.Source, "oauth")
	}
}

func TestLoadCachedSnapshot_MissingFile_ReturnsNil(t *testing.T) {
	setupTempDir(t)
	if got := LoadCachedSnapshot("nonexistent"); got != nil {
		t.Errorf("LoadCachedSnapshot() = %+v, want nil", got)
	}
}

func TestLoadCachedSnapshot_MalformedJSON_ReturnsNil(t *testing.T) {
	setupTempDir(t)
	path := SnapshotPath("broken")
	writeTestFile(t, path, []byte("{not json}"))

	if got := LoadCachedSnapshot("broken"); got != nil {
		t.Errorf("LoadCachedSnapshot() = %+v, want nil for malformed JSON", got)
	}
}

func TestLoadCachedSnapshot_EmptyFile_ReturnsNil(t *testing.T) {
	setupTempDir(t)
	path := SnapshotPath("empty")
	writeTestFile(t, path, []byte(""))

	if got := LoadCachedSnapshot("empty"); got != nil {
		t.Errorf("LoadCachedSnapshot() = %+v, want nil for empty file", got)
	}
}

func TestSnapshotPath_Format(t *testing.T) {
	setupTempDir(t)
	got := SnapshotPath("claude")
	want := filepath.Join(SnapshotsDir(), "claude.json")
	if got != want {
		t.Errorf("SnapshotPath() = %q, want %q", got, want)
	}
}

// Cache: OrgID

func TestCacheOrgID_LoadCachedOrgID_Roundtrip(t *testing.T) {
	setupTempDir(t)

	if err := CacheOrgID("claude", "org-12345"); err != nil {
		t.Fatalf("CacheOrgID() error: %v", err)
	}

	got := LoadCachedOrgID("claude")
	if got != "org-12345" {
		t.Errorf("LoadCachedOrgID() = %q, want %q", got, "org-12345")
	}
}

func TestLoadCachedOrgID_MissingFile_ReturnsEmpty(t *testing.T) {
	setupTempDir(t)
	if got := LoadCachedOrgID("nonexistent"); got != "" {
		t.Errorf("LoadCachedOrgID() = %q, want empty", got)
	}
}

func TestOrgIDPath_Format(t *testing.T) {
	setupTempDir(t)
	got := OrgIDPath("gemini")
	want := filepath.Join(OrgIDsDir(), "gemini.txt")
	if got != want {
		t.Errorf("OrgIDPath() = %q, want %q", got, want)
	}
}

// Clear operations

func TestClearSnapshotCache_SpecificProvider(t *testing.T) {
	setupTempDir(t)

	snap1 := models.UsageSnapshot{Provider: "claude"}
	snap2 := models.UsageSnapshot{Provider: "copilot"}
	if err := CacheSnapshot(snap1); err != nil {
		t.Fatalf("CacheSnapshot(claude) error: %v", err)
	}
	if err := CacheSnapshot(snap2); err != nil {
		t.Fatalf("CacheSnapshot(copilot) error: %v", err)
	}

	ClearSnapshotCache("claude")

	if LoadCachedSnapshot("claude") != nil {
		t.Error("claude snapshot should be cleared")
	}
	if LoadCachedSnapshot("copilot") == nil {
		t.Error("copilot snapshot should still exist")
	}
}

func TestClearSnapshotCache_AllProviders(t *testing.T) {
	setupTempDir(t)

	snap1 := models.UsageSnapshot{Provider: "claude"}
	snap2 := models.UsageSnapshot{Provider: "copilot"}
	if err := CacheSnapshot(snap1); err != nil {
		t.Fatalf("CacheSnapshot(claude) error: %v", err)
	}
	if err := CacheSnapshot(snap2); err != nil {
		t.Fatalf("CacheSnapshot(copilot) error: %v", err)
	}

	ClearSnapshotCache("")

	if LoadCachedSnapshot("claude") != nil {
		t.Error("claude snapshot should be cleared")
	}
	if LoadCachedSnapshot("copilot") != nil {
		t.Error("copilot snapshot should be cleared")
	}
}

func TestClearOrgIDCache_SpecificProvider(t *testing.T) {
	setupTempDir(t)

	if err := CacheOrgID("claude", "org-1"); err != nil {
		t.Fatalf("CacheOrgID(claude) error: %v", err)
	}
	if err := CacheOrgID("copilot", "org-2"); err != nil {
		t.Fatalf("CacheOrgID(copilot) error: %v", err)
	}

	ClearOrgIDCache("claude")

	if LoadCachedOrgID("claude") != "" {
		t.Error("claude org ID should be cleared")
	}
	if LoadCachedOrgID("copilot") != "org-2" {
		t.Error("copilot org ID should still exist")
	}
}

func TestClearOrgIDCache_AllProviders(t *testing.T) {
	setupTempDir(t)

	if err := CacheOrgID("claude", "org-1"); err != nil {
		t.Fatalf("CacheOrgID(claude) error: %v", err)
	}
	if err := CacheOrgID("copilot", "org-2"); err != nil {
		t.Fatalf("CacheOrgID(copilot) error: %v", err)
	}

	ClearOrgIDCache("")

	if LoadCachedOrgID("claude") != "" {
		t.Error("claude org ID should be cleared")
	}
	if LoadCachedOrgID("copilot") != "" {
		t.Error("copilot org ID should be cleared")
	}
}

func TestClearProviderCache_RemovesBoth(t *testing.T) {
	setupTempDir(t)

	snap := models.UsageSnapshot{Provider: "claude"}
	if err := CacheSnapshot(snap); err != nil {
		t.Fatalf("CacheSnapshot error: %v", err)
	}
	if err := CacheOrgID("claude", "org-1"); err != nil {
		t.Fatalf("CacheOrgID error: %v", err)
	}

	ClearProviderCache("claude")

	if LoadCachedSnapshot("claude") != nil {
		t.Error("claude snapshot should be cleared")
	}
	if LoadCachedOrgID("claude") != "" {
		t.Error("claude org ID should be cleared")
	}
}

func TestClearAllCache_SpecificProvider(t *testing.T) {
	setupTempDir(t)

	snap1 := models.UsageSnapshot{Provider: "claude"}
	snap2 := models.UsageSnapshot{Provider: "copilot"}
	if err := CacheSnapshot(snap1); err != nil {
		t.Fatalf("CacheSnapshot(claude) error: %v", err)
	}
	if err := CacheSnapshot(snap2); err != nil {
		t.Fatalf("CacheSnapshot(copilot) error: %v", err)
	}
	if err := CacheOrgID("claude", "org-1"); err != nil {
		t.Fatalf("CacheOrgID(claude) error: %v", err)
	}
	if err := CacheOrgID("copilot", "org-2"); err != nil {
		t.Fatalf("CacheOrgID(copilot) error: %v", err)
	}

	ClearAllCache("claude")

	if LoadCachedSnapshot("claude") != nil {
		t.Error("claude snapshot should be cleared")
	}
	if LoadCachedOrgID("claude") != "" {
		t.Error("claude org ID should be cleared")
	}
	if LoadCachedSnapshot("copilot") == nil {
		t.Error("copilot snapshot should still exist")
	}
	if LoadCachedOrgID("copilot") != "org-2" {
		t.Error("copilot org ID should still exist")
	}
}

func TestClearAllCache_AllProviders(t *testing.T) {
	setupTempDir(t)

	snap1 := models.UsageSnapshot{Provider: "claude"}
	snap2 := models.UsageSnapshot{Provider: "copilot"}
	if err := CacheSnapshot(snap1); err != nil {
		t.Fatalf("CacheSnapshot(claude) error: %v", err)
	}
	if err := CacheSnapshot(snap2); err != nil {
		t.Fatalf("CacheSnapshot(copilot) error: %v", err)
	}
	if err := CacheOrgID("claude", "org-1"); err != nil {
		t.Fatalf("CacheOrgID(claude) error: %v", err)
	}
	if err := CacheOrgID("copilot", "org-2"); err != nil {
		t.Fatalf("CacheOrgID(copilot) error: %v", err)
	}

	ClearAllCache("")

	if LoadCachedSnapshot("claude") != nil {
		t.Error("claude snapshot should be cleared")
	}
	if LoadCachedSnapshot("copilot") != nil {
		t.Error("copilot snapshot should be cleared")
	}
	if LoadCachedOrgID("claude") != "" {
		t.Error("claude org ID should be cleared")
	}
	if LoadCachedOrgID("copilot") != "" {
		t.Error("copilot org ID should be cleared")
	}
}

func TestClearSnapshotCache_NonexistentProvider_NoError(t *testing.T) {
	setupTempDir(t)
	// Should not panic or error
	ClearSnapshotCache("doesnotexist")
}

func TestClearOrgIDCache_NonexistentProvider_NoError(t *testing.T) {
	setupTempDir(t)
	// Should not panic or error
	ClearOrgIDCache("doesnotexist")
}

func TestClearAllCache_EmptyDirs_NoError(t *testing.T) {
	setupTempDir(t)
	// Should not panic or error on empty dirs
	ClearAllCache("")
}

// Credentials

func TestCredentialPath_Format(t *testing.T) {
	t.Setenv("VIBEUSAGE_CONFIG_DIR", "/base/config")
	tests := []struct {
		provider string
		credType string
		want     string
	}{
		{"claude", "oauth", "/base/config/credentials/claude/oauth.json"},
		{"copilot", "session", "/base/config/credentials/copilot/session.json"},
		{"gemini", "apikey", "/base/config/credentials/gemini/apikey.json"},
	}
	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.credType, func(t *testing.T) {
			got := CredentialPath(tt.provider, tt.credType)
			if got != tt.want {
				t.Errorf("CredentialPath(%q, %q) = %q, want %q", tt.provider, tt.credType, got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"tilde prefix expands", "~/foo/bar", filepath.Join(home, "foo/bar")},
		{"absolute path unchanged", "/absolute/path", "/absolute/path"},
		{"relative path unchanged", "relative/path", "relative/path"},
		{"empty string unchanged", "", ""},
		{"tilde only not expanded", "~", "~"},
		{"tilde-user not expanded", "~user/path", "~user/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.path)
			if got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestWriteCredential_ReadCredential_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "provider", "oauth.json")
	content := []byte(`{"token":"secret123"}`)

	if err := WriteCredential(path, content); err != nil {
		t.Fatalf("WriteCredential() error: %v", err)
	}

	got, err := ReadCredential(path)
	if err != nil {
		t.Fatalf("ReadCredential() error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("ReadCredential() = %q, want %q", got, content)
	}
}

func TestWriteCredential_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cred.json")

	if err := WriteCredential(path, []byte("secret")); err != nil {
		t.Fatalf("WriteCredential() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestWriteCredential_Atomic_NoTmpFileLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cred.json")

	if err := WriteCredential(path, []byte("data")); err != nil {
		t.Fatalf("WriteCredential() error: %v", err)
	}

	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temporary file should not remain after WriteCredential")
	}
}

func TestWriteCredential_CreatesDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "cred.json")

	if err := WriteCredential(path, []byte("data")); err != nil {
		t.Fatalf("WriteCredential() error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("WriteCredential should create parent directories")
	}
}

func TestReadCredential_MissingFile_ReturnsNilNil(t *testing.T) {
	data, err := ReadCredential("/nonexistent/path/cred.json")
	if err != nil {
		t.Errorf("ReadCredential() error = %v, want nil", err)
	}
	if data != nil {
		t.Errorf("ReadCredential() = %v, want nil", data)
	}
}

func TestDeleteCredential_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cred.json")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !DeleteCredential(path) {
		t.Error("DeleteCredential() should return true for existing file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestDeleteCredential_MissingFile(t *testing.T) {
	if DeleteCredential("/nonexistent/cred.json") {
		t.Error("DeleteCredential() should return false for missing file")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !fileExists(existingFile) {
		t.Error("fileExists() should return true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("fileExists() should return false for missing file")
	}
	// directories also return true for fileExists
	if !fileExists(dir) {
		t.Error("fileExists() should return true for existing directory")
	}
}

// FindProviderCredential

func TestFindProviderCredential_VibeusageStorage(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)

	// Write a credential in vibeusage storage
	credPath := CredentialPath("claude", "oauth")
	if err := WriteCredential(credPath, []byte(`{"token":"test"}`)); err != nil {
		t.Fatalf("WriteCredential() error: %v", err)
	}

	found, source, path := FindProviderCredential("claude")
	if !found {
		t.Error("should find credential in vibeusage storage")
	}
	if source != "vibeusage" {
		t.Errorf("source = %q, want %q", source, "vibeusage")
	}
	if path != credPath {
		t.Errorf("path = %q, want %q", path, credPath)
	}
}

func TestFindProviderCredential_EnvVar(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")

	found, source, path := FindProviderCredential("claude")
	if !found {
		t.Error("should find credential from env var")
	}
	if source != "env" {
		t.Errorf("source = %q, want %q", source, "env")
	}
	if path != "" {
		t.Errorf("path = %q, want empty for env source", path)
	}
}

func TestFindProviderCredential_UnknownProvider(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	found, source, path := FindProviderCredential("unknownprovider")
	if found {
		t.Error("should not find credential for unknown provider")
	}
	if source != "" {
		t.Errorf("source = %q, want empty", source)
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
}

func TestFindProviderCredential_VibeusageTakesPrecedenceOverEnv(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")

	credPath := CredentialPath("claude", "session")
	WriteCredential(credPath, []byte(`{"key":"val"}`))

	_, source, _ := FindProviderCredential("claude")
	if source != "vibeusage" {
		t.Errorf("vibeusage storage should take precedence, got source = %q", source)
	}
}

func TestFindProviderCredential_CredentialTypes(t *testing.T) {
	// Test that all three credential types are checked
	credTypes := []string{"oauth", "session", "apikey"}

	for _, credType := range credTypes {
		t.Run(credType, func(t *testing.T) {
			setupTempDirWithCredentialIsolation(t)
			credPath := CredentialPath("claude", credType)
			WriteCredential(credPath, []byte(`{}`))

			found, source, _ := FindProviderCredential("claude")
			if !found {
				t.Errorf("should find %s credential", credType)
			}
			if source != "vibeusage" {
				t.Errorf("source = %q, want %q", source, "vibeusage")
			}
		})
	}
}

// CheckProviderCredentials

func TestCheckProviderCredentials_Found(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	WriteCredential(CredentialPath("claude", "oauth"), []byte(`{}`))

	hasCreds, source := CheckProviderCredentials("claude")
	if !hasCreds {
		t.Error("should report credentials found")
	}
	if source != "vibeusage" {
		t.Errorf("source = %q, want %q", source, "vibeusage")
	}
}

func TestCheckProviderCredentials_NotFound(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	hasCreds, source := CheckProviderCredentials("claude")
	if hasCreds {
		t.Error("should report no credentials")
	}
	if source != "" {
		t.Errorf("source = %q, want empty", source)
	}
}

// GetAllCredentialStatus

func TestGetAllCredentialStatus_ReturnsAllProviders(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	status := GetAllCredentialStatus()

	// Should have an entry for every provider in ProviderCLIPaths
	for providerID := range ProviderCLIPaths {
		if _, ok := status[providerID]; !ok {
			t.Errorf("missing status for provider %q", providerID)
		}
	}
}

func TestGetAllCredentialStatus_ReflectsCredentials(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)

	WriteCredential(CredentialPath("claude", "oauth"), []byte(`{}`))

	status := GetAllCredentialStatus()

	claudeStatus := status["claude"]
	if !claudeStatus.HasCredentials {
		t.Error("claude should have credentials")
	}
	if claudeStatus.Source != "vibeusage" {
		t.Errorf("claude source = %q, want %q", claudeStatus.Source, "vibeusage")
	}

	copilotStatus := status["copilot"]
	if copilotStatus.HasCredentials {
		t.Error("copilot should not have credentials")
	}
}

// IsFirstRun

func TestIsFirstRun_NoCreds(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	if !IsFirstRun() {
		t.Error("IsFirstRun() should be true when no credentials exist")
	}
}

func TestIsFirstRun_WithCreds(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	WriteCredential(CredentialPath("claude", "oauth"), []byte(`{}`))

	if IsFirstRun() {
		t.Error("IsFirstRun() should be false when credentials exist")
	}
}

// CountConfiguredProviders

func TestCountConfiguredProviders_None(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)
	if got := CountConfiguredProviders(); got != 0 {
		t.Errorf("CountConfiguredProviders() = %d, want 0", got)
	}
}

func TestCountConfiguredProviders_Some(t *testing.T) {
	setupTempDirWithCredentialIsolation(t)

	WriteCredential(CredentialPath("claude", "oauth"), []byte(`{}`))
	WriteCredential(CredentialPath("copilot", "session"), []byte(`{}`))

	if got := CountConfiguredProviders(); got != 2 {
		t.Errorf("CountConfiguredProviders() = %d, want 2", got)
	}
}

// CacheSnapshot edge cases

func TestCacheSnapshot_OverwritesExisting(t *testing.T) {
	setupTempDir(t)

	snap1 := models.UsageSnapshot{
		Provider: "claude",
		Source:   "v1",
	}
	snap2 := models.UsageSnapshot{
		Provider: "claude",
		Source:   "v2",
	}

	if err := CacheSnapshot(snap1); err != nil {
		t.Fatalf("CacheSnapshot(v1) error: %v", err)
	}
	if err := CacheSnapshot(snap2); err != nil {
		t.Fatalf("CacheSnapshot(v2) error: %v", err)
	}

	loaded := LoadCachedSnapshot("claude")
	if loaded == nil {
		t.Fatal("LoadCachedSnapshot() returned nil")
	}
	if loaded.Source != "v2" {
		t.Errorf("Source = %q, want %q (overwritten)", loaded.Source, "v2")
	}
}

func TestCacheSnapshot_MultipleProviders(t *testing.T) {
	setupTempDir(t)

	for _, id := range []string{"claude", "copilot", "gemini"} {
		snap := models.UsageSnapshot{Provider: id, Source: id + "-src"}
		if err := CacheSnapshot(snap); err != nil {
			t.Fatalf("CacheSnapshot(%q) error: %v", id, err)
		}
	}

	for _, id := range []string{"claude", "copilot", "gemini"} {
		loaded := LoadCachedSnapshot(id)
		if loaded == nil {
			t.Errorf("LoadCachedSnapshot(%q) returned nil", id)
			continue
		}
		if loaded.Source != id+"-src" {
			t.Errorf("LoadCachedSnapshot(%q).Source = %q, want %q", id, loaded.Source, id+"-src")
		}
	}
}

func TestCacheSnapshot_PreservesComplexData(t *testing.T) {
	setupTempDir(t)

	now := time.Now().Truncate(time.Millisecond)
	reset := now.Add(24 * time.Hour)

	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods: []models.UsagePeriod{
			{
				Name:        "monthly",
				PeriodType:  models.PeriodMonthly,
				Utilization: 75,
				ResetsAt:    &reset,
			},
			{
				Name:        "sonnet",
				PeriodType:  models.PeriodDaily,
				Utilization: 30,
				Model:       "claude-3-sonnet",
			},
		},
		Overage: &models.OverageUsage{
			Used:  5.50,
			Limit: 100.0,
		},
		Identity: &models.ProviderIdentity{
			Email: "user@example.com",
		},
		Source: "oauth",
	}

	if err := CacheSnapshot(snap); err != nil {
		t.Fatalf("CacheSnapshot() error: %v", err)
	}

	loaded := LoadCachedSnapshot("claude")
	if loaded == nil {
		t.Fatal("LoadCachedSnapshot() returned nil")
	}

	if len(loaded.Periods) != 2 {
		t.Fatalf("Periods len = %d, want 2", len(loaded.Periods))
	}
	if loaded.Periods[0].Utilization != 75 {
		t.Errorf("Periods[0].Utilization = %d, want 75", loaded.Periods[0].Utilization)
	}
	if loaded.Periods[0].PeriodType != models.PeriodMonthly {
		t.Errorf("Periods[0].PeriodType = %q, want %q", loaded.Periods[0].PeriodType, models.PeriodMonthly)
	}
	if loaded.Periods[0].ResetsAt == nil {
		t.Fatal("Periods[0].ResetsAt should not be nil")
	}
	if !loaded.Periods[0].ResetsAt.Equal(reset) {
		t.Errorf("Periods[0].ResetsAt = %v, want %v", *loaded.Periods[0].ResetsAt, reset)
	}
	if loaded.Periods[1].Model != "claude-3-sonnet" {
		t.Errorf("Periods[1].Model = %q, want %q", loaded.Periods[1].Model, "claude-3-sonnet")
	}
	if loaded.Periods[1].PeriodType != models.PeriodDaily {
		t.Errorf("Periods[1].PeriodType = %q, want %q", loaded.Periods[1].PeriodType, models.PeriodDaily)
	}
	if loaded.Overage == nil {
		t.Fatal("Overage should not be nil")
	}
	if loaded.Overage.Used != 5.50 {
		t.Errorf("Overage.Used = %v, want 5.50", loaded.Overage.Used)
	}
	if loaded.Overage.Limit != 100.0 {
		t.Errorf("Overage.Limit = %v, want 100.0", loaded.Overage.Limit)
	}
	if loaded.Identity == nil {
		t.Fatal("Identity should not be nil")
	}
	if loaded.Identity.Email != "user@example.com" {
		t.Errorf("Identity.Email = %q, want %q", loaded.Identity.Email, "user@example.com")
	}
}

// Validate snapshot JSON is valid JSON on disk

func TestCacheSnapshot_WritesValidJSON(t *testing.T) {
	setupTempDir(t)

	snap := models.UsageSnapshot{
		Provider: "claude",
		Source:   "test",
	}
	if err := CacheSnapshot(snap); err != nil {
		t.Fatalf("CacheSnapshot error: %v", err)
	}

	data, err := os.ReadFile(SnapshotPath("claude"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("snapshot file is not valid JSON: %s", data)
	}
}
