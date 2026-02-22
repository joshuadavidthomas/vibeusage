package config

import (
	"os"
	"path/filepath"
)

const appName = "vibeusage"

func ConfigDir() string {
	if v := os.Getenv("VIBEUSAGE_CONFIG_DIR"); v != "" {
		return v
	}
	return filepath.Join(userConfigDir(), appName)
}

func CacheDir() string {
	if v := os.Getenv("VIBEUSAGE_CACHE_DIR"); v != "" {
		return v
	}
	return filepath.Join(userCacheDir(), appName)
}

func CredentialsDir() string  { return filepath.Join(ConfigDir(), "credentials") }
func SnapshotsDir() string    { return filepath.Join(CacheDir(), "snapshots") }
func OrgIDsDir() string       { return filepath.Join(CacheDir(), "org-ids") }
func ModelsFile() string      { return filepath.Join(CacheDir(), "models.json") }
func MultipliersFile() string { return filepath.Join(CacheDir(), "multipliers.json") }
func ConfigFile() string      { return filepath.Join(ConfigDir(), "config.toml") }

func userConfigDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	return filepath.Join(homeDir(), ".config")
}

func userCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return d
	}
	return filepath.Join(homeDir(), ".cache")
}

func homeDir() string {
	if d, err := os.UserHomeDir(); err == nil {
		return d
	}
	return "."
}
