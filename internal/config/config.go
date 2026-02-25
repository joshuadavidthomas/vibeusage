package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

type DisplayConfig struct {
	ShowRemaining bool   `toml:"show_remaining" json:"show_remaining"`
	PaceColors    bool   `toml:"pace_colors" json:"pace_colors"`
	ResetFormat   string `toml:"reset_format" json:"reset_format"`
}

type FetchConfig struct {
	Timeout               float64 `toml:"timeout" json:"timeout"`
	MaxConcurrent         int     `toml:"max_concurrent" json:"max_concurrent"`
	StaleThresholdMinutes int     `toml:"stale_threshold_minutes" json:"stale_threshold_minutes"`
}

type CredentialsConfig struct {
	UseKeyring               bool `toml:"use_keyring" json:"use_keyring"`
	ReuseProviderCredentials bool `toml:"reuse_provider_credentials" json:"reuse_provider_credentials"`
}

type ProviderConfig struct {
	AuthSource       string `toml:"auth_source" json:"auth_source"`
	PreferredBrowser string `toml:"preferred_browser,omitempty" json:"preferred_browser,omitempty"`
	Enabled          bool   `toml:"enabled" json:"enabled"`
}

type RoleConfig struct {
	Models []string `toml:"models" json:"models"`
}

type Config struct {
	EnabledProviders []string                  `toml:"enabled_providers" json:"enabled_providers"`
	Display          DisplayConfig             `toml:"display" json:"display"`
	Fetch            FetchConfig               `toml:"fetch" json:"fetch"`
	Credentials      CredentialsConfig         `toml:"credentials" json:"credentials"`
	Providers        map[string]ProviderConfig `toml:"providers" json:"providers"`
	Roles            map[string]RoleConfig     `toml:"roles" json:"roles"`
}

func DefaultConfig() Config {
	return Config{
		EnabledProviders: nil,
		Display: DisplayConfig{
			ShowRemaining: true,
			PaceColors:    true,
			ResetFormat:   "countdown",
		},
		Fetch: FetchConfig{
			Timeout:               30.0,
			MaxConcurrent:         5,
			StaleThresholdMinutes: 60,
		},
		Credentials: CredentialsConfig{
			UseKeyring:               false,
			ReuseProviderCredentials: true,
		},
		Providers: make(map[string]ProviderConfig),
		Roles:     make(map[string]RoleConfig),
	}
}

// DefaultRoles returns the starter roles seeded during init.
func DefaultRoles() map[string]RoleConfig {
	return map[string]RoleConfig{
		"thinking": {Models: []string{"claude-opus-4-6", "gpt-5.2-pro", "gemini-3.1-pro-preview"}},
		"coding":   {Models: []string{"claude-sonnet-4-6", "gpt-5.3-codex"}},
		"fast":     {Models: []string{"claude-haiku-4-5", "gpt-5.1-codex-mini", "gemini-3-flash-preview"}},
	}
}

// SeedDefaultRoles writes the default roles to the config file if no roles
// are configured yet. Returns true if roles were seeded.
func SeedDefaultRoles() bool {
	cfg := Get()
	if len(cfg.Roles) > 0 {
		return false
	}

	for name, role := range DefaultRoles() {
		cfg.Roles[name] = role
	}

	if err := Save(cfg, ""); err != nil {
		return false
	}
	_, _ = Reload()
	return true
}

func (c Config) clone() Config {
	out := c
	if c.EnabledProviders != nil {
		out.EnabledProviders = make([]string, len(c.EnabledProviders))
		copy(out.EnabledProviders, c.EnabledProviders)
	}
	out.Providers = make(map[string]ProviderConfig, len(c.Providers))
	for k, v := range c.Providers {
		out.Providers[k] = v
	}
	out.Roles = make(map[string]RoleConfig, len(c.Roles))
	for k, v := range c.Roles {
		models := make([]string, len(v.Models))
		copy(models, v.Models)
		out.Roles[k] = RoleConfig{Models: models}
	}
	return out
}

func (c Config) IsProviderEnabled(providerID string) bool {
	if pc, ok := c.Providers[providerID]; ok && !pc.Enabled {
		return false
	}
	if len(c.EnabledProviders) == 0 {
		return true
	}
	for _, id := range c.EnabledProviders {
		if id == providerID {
			return true
		}
	}
	return false
}

func (c Config) GetRole(name string) (RoleConfig, bool) {
	r, ok := c.Roles[name]
	return r, ok
}

func (c Config) RoleNames() []string {
	names := make([]string, 0, len(c.Roles))
	for name := range c.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var (
	globalConfig *Config
	configMu     sync.RWMutex
)

func Get() Config {
	configMu.RLock()
	if c := globalConfig; c != nil {
		configMu.RUnlock()
		return c.clone()
	}
	configMu.RUnlock()

	configMu.Lock()
	defer configMu.Unlock()
	if globalConfig != nil {
		return globalConfig.clone()
	}
	c, _ := Load("")
	globalConfig = &c
	return c.clone()
}

func Reload() (Config, error) {
	configMu.Lock()
	defer configMu.Unlock()
	c, err := Load("")
	globalConfig = &c
	return c.clone(), err
}

func Load(path string) (Config, error) {
	if path == "" {
		path = ConfigFile()
	}
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if legacyPath := legacyConfigFilePath(path); legacyPath != "" {
			// TODO(v0.3.0): remove legacy config read fallback after the v0.2.0 migration window.
			if legacyData, legacyErr := os.ReadFile(legacyPath); legacyErr == nil {
				data = legacyData
				err = nil
			}
		}
		if err != nil {
			return applyEnvOverrides(cfg), nil
		}
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return applyEnvOverrides(DefaultConfig()), fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Ensure maps are initialized
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	if cfg.Roles == nil {
		cfg.Roles = make(map[string]RoleConfig)
	}

	return applyEnvOverrides(cfg), nil
}

func Save(cfg Config, path string) error {
	if path == "" {
		path = ConfigFile()
	}
	if err := saveConfigFile(cfg, path); err != nil {
		return err
	}

	legacyPath := legacyConfigFilePath(path)
	if legacyPath != "" {
		// TODO(v0.3.0): remove temporary dual-write compatibility after v0.2.0 migration window.
		_ = saveConfigFile(cfg, legacyPath)
	}

	return nil
}

func saveConfigFile(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// TODO(v0.3.0): remove legacy config path fallback after the v0.2.0 migration window.
func legacyConfigFilePath(path string) string {
	if filepath.Clean(path) != filepath.Clean(ConfigFile()) {
		return ""
	}
	legacy := legacyConfigFile()
	if filepath.Clean(legacy) == filepath.Clean(path) {
		return ""
	}
	return legacy
}

func applyEnvOverrides(cfg Config) Config {
	if v := os.Getenv("VIBEUSAGE_ENABLED_PROVIDERS"); v != "" {
		parts := strings.Split(v, ",")
		var providers []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				providers = append(providers, p)
			}
		}
		cfg.EnabledProviders = providers
	}
	if os.Getenv("VIBEUSAGE_NO_COLOR") != "" {
		cfg.Display.PaceColors = false
	}
	return cfg
}
