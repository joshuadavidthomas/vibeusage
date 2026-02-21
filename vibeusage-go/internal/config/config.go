package config

import (
	"os"
	"path/filepath"
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

type Config struct {
	EnabledProviders []string                  `toml:"enabled_providers" json:"enabled_providers"`
	Display          DisplayConfig             `toml:"display" json:"display"`
	Fetch            FetchConfig               `toml:"fetch" json:"fetch"`
	Credentials      CredentialsConfig         `toml:"credentials" json:"credentials"`
	Providers        map[string]ProviderConfig `toml:"providers" json:"providers"`
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
	}
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
	c := Load("")
	globalConfig = &c
	return c.clone()
}

func Reload() Config {
	configMu.Lock()
	defer configMu.Unlock()
	c := Load("")
	globalConfig = &c
	return c.clone()
}

func Load(path string) Config {
	if path == "" {
		path = ConfigFile()
	}
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return applyEnvOverrides(cfg)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return applyEnvOverrides(DefaultConfig())
	}

	// Ensure Providers map is initialized
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}

	return applyEnvOverrides(cfg)
}

func Save(cfg Config, path string) error {
	if path == "" {
		path = ConfigFile()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return toml.NewEncoder(f).Encode(cfg)
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
