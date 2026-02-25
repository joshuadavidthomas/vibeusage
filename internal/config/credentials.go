package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		if legacy := legacyCredentialPathFor(providerID, credType); fileExists(legacy) {
			// TODO(v0.3.0): remove legacy credential read fallback after the v0.2.0 migration window.
			return true, "vibeusage", legacy
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
	if err := writeCredentialFile(path, content); err != nil {
		return err
	}

	if legacy := legacyCredentialPath(path); legacy != "" {
		// TODO(v0.3.0): remove temporary dual-write compatibility after v0.2.0 migration window.
		_ = writeCredentialFile(legacy, content)
	}

	return nil
}

func writeCredentialFile(path string, content []byte) error {
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
	if fileExists(path) {
		return os.ReadFile(path)
	}
	if legacy := legacyCredentialPath(path); legacy != "" && fileExists(legacy) {
		// TODO(v0.3.0): remove legacy credential read fallback after the v0.2.0 migration window.
		return os.ReadFile(legacy)
	}
	return nil, nil
}

func DeleteCredential(path string) bool {
	deleted := false
	if err := os.Remove(path); err == nil {
		deleted = true
	}
	if legacy := legacyCredentialPath(path); legacy != "" {
		// TODO(v0.3.0): remove legacy credential delete fallback after the v0.2.0 migration window.
		if err := os.Remove(legacy); err == nil {
			deleted = true
		}
	}
	return deleted
}

// TODO(v0.3.0): remove legacy credential path helpers after the v0.2.0 migration window.
func legacyCredentialPathFor(providerID, credType string) string {
	return filepath.Join(legacyCredentialsDir(), providerID, credType+".json")
}

// TODO(v0.3.0): remove legacy credential path helpers after the v0.2.0 migration window.
func legacyCredentialPath(path string) string {
	currentDir := filepath.Clean(CredentialsDir())
	cleanPath := filepath.Clean(path)

	rel, err := filepath.Rel(currentDir, cleanPath)
	if err != nil {
		return ""
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}

	legacyPath := filepath.Join(legacyCredentialsDir(), rel)
	if legacyPath == cleanPath {
		return ""
	}
	return legacyPath
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
