package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func CredentialPath(providerID, credType string) string {
	return filepath.Join(CredentialsDir(), providerID, credType+".json")
}

func expandPath(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// FindProviderCredential checks for credentials in vibeusage storage,
// provider CLI paths, and environment variables.
// cliPaths are external CLI credential file paths (may contain ~ for home).
// envVars are environment variable names to check.
// Returns (found, source, path).
func FindProviderCredential(providerID string, cliPaths []string, envVars []string) (bool, string, string) {
	cfg := Get()

	// Check vibeusage storage first
	for _, credType := range []string{"oauth", "session", "apikey"} {
		p := CredentialPath(providerID, credType)
		if fileExists(p) {
			return true, "vibeusage", p
		}
	}

	// Check provider CLI credentials
	if cfg.Credentials.ReuseProviderCredentials {
		for _, raw := range cliPaths {
			p := expandPath(raw)
			if fileExists(p) {
				return true, "provider_cli", p
			}
		}
	}

	// Check environment variables
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			return true, "env", ""
		}
	}

	return false, "", ""
}

func WriteCredential(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("writing credential: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return fmt.Errorf("writing credential: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("writing credential: %w", err)
	}
	return nil
}

func ReadCredential(path string) ([]byte, error) {
	if !fileExists(path) {
		return nil, nil
	}
	return os.ReadFile(path)
}

func DeleteCredential(path string) bool {
	if err := os.Remove(path); err != nil {
		return false
	}
	return true
}

// CredentialStatus reports whether a provider has credentials and their source.
type CredentialStatus struct {
	HasCredentials bool
	Source         string
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
