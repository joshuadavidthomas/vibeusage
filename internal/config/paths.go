package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/adrg/xdg"
)

const appName = "vibeusage"

func ConfigDir() string {
	if v := os.Getenv("VIBEUSAGE_CONFIG_DIR"); v != "" {
		return v
	}

	canonical := filepath.Join(xdg.ConfigHome, appName)
	if runtime.GOOS != "darwin" || os.Getenv("XDG_CONFIG_HOME") != "" {
		return canonical
	}

	preferred := filepath.Join(homeDir(), ".config", appName)
	if regularFileExists(filepath.Join(preferred, "config.toml")) {
		return preferred
	}
	if regularFileExists(filepath.Join(canonical, "config.toml")) {
		// TODO(v0.3.0): remove legacy macOS config-path fallback after the v0.2.0 migration window.
		return canonical
	}
	return preferred
}

func DataDir() string {
	if v := os.Getenv("VIBEUSAGE_DATA_DIR"); v != "" {
		return v
	}
	return filepath.Join(xdg.DataHome, appName)
}

func CacheDir() string {
	if v := os.Getenv("VIBEUSAGE_CACHE_DIR"); v != "" {
		return v
	}
	return filepath.Join(xdg.CacheHome, appName)
}

func EnabledProvidersFile() string { return filepath.Join(DataDir(), "enabled_providers.json") }
func CredentialsDir() string       { return filepath.Join(DataDir(), "credentials") }
func SnapshotsDir() string         { return filepath.Join(CacheDir(), "snapshots") }
func OrgIDsDir() string            { return filepath.Join(CacheDir(), "org-ids") }
func ModelsFile() string           { return filepath.Join(CacheDir(), "models.json") }
func MultipliersFile() string      { return filepath.Join(CacheDir(), "multipliers.json") }
func ConfigFile() string           { return filepath.Join(ConfigDir(), "config.toml") }

// TODO(v0.3.0): remove legacy config path helpers after the v0.2.0 migration window.
func legacyConfigDir() string {
	if v := os.Getenv("VIBEUSAGE_CONFIG_DIR"); v != "" {
		return v
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, appName)
	}
	return filepath.Join(homeDir(), ".config", appName)
}

// TODO(v0.3.0): remove legacy config path helpers after the v0.2.0 migration window.
func legacyConfigFile() string {
	return filepath.Join(legacyConfigDir(), "config.toml")
}

// TODO(v0.3.0): remove legacy config path helpers after the v0.2.0 migration window.
func legacyCredentialsDir() string {
	return filepath.Join(legacyConfigDir(), "credentials")
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func homeDir() string {
	if d := xdg.Home; d != "" {
		return d
	}
	if d, err := os.UserHomeDir(); err == nil {
		return d
	}
	return "."
}
