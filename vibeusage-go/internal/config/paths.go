package config

import (
	"os"
	"path/filepath"
	"runtime"
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

func StateDir() string {
	return filepath.Join(userStateDir(), appName)
}

func CredentialsDir() string { return filepath.Join(ConfigDir(), "credentials") }
func SnapshotsDir() string   { return filepath.Join(CacheDir(), "snapshots") }
func OrgIDsDir() string      { return filepath.Join(CacheDir(), "org-ids") }
func GateDir() string        { return filepath.Join(StateDir(), "gates") }
func ConfigFile() string     { return filepath.Join(ConfigDir(), "config.toml") }

func EnsureDirectories() error {
	dirs := []string{
		ConfigDir(), CacheDir(), StateDir(),
		CredentialsDir(), SnapshotsDir(), OrgIDsDir(), GateDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

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

func userStateDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return d
		}
		return filepath.Join(homeDir(), "AppData", "Local")
	default:
		if d := os.Getenv("XDG_STATE_HOME"); d != "" {
			return d
		}
		return filepath.Join(homeDir(), ".local", "state")
	}
}

func homeDir() string {
	if d, err := os.UserHomeDir(); err == nil {
		return d
	}
	return "."
}
