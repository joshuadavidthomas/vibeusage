package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProviderCLIPaths maps provider IDs to their CLI credential file paths.
var ProviderCLIPaths = map[string][]string{
	"antigravity": {"~/.config/Antigravity/credentials.json"},
	"claude":      {"~/.claude/.credentials.json"},
	"codex":       {"~/.codex/auth.json"},
	"gemini":      {"~/.gemini/oauth_creds.json"},
	"copilot":     {"~/.config/github-copilot/hosts.json"},
	"cursor":      {"~/.cursor/mcp-state.json"},
	"kimi":        {"~/.kimi/credentials/kimi-code.json"},
	"minimax":     {},
	"zai":         {},
}

// ProviderEnvVars maps provider IDs to their environment variable names.
var ProviderEnvVars = map[string]string{
	"antigravity": "ANTIGRAVITY_API_KEY",
	"claude":      "ANTHROPIC_API_KEY",
	"codex":       "OPENAI_API_KEY",
	"gemini":      "GEMINI_API_KEY",
	"copilot":     "GITHUB_TOKEN",
	"cursor":      "CURSOR_API_KEY",
	"kimi":        "KIMI_CODE_API_KEY",
	"minimax":     "MINIMAX_API_KEY",
	"zai":         "ZAI_API_KEY",
}

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

// FindProviderCredential checks for credentials in vibeusage storage, provider CLIs, and env vars.
// Returns (found, source, path).
func FindProviderCredential(providerID string) (bool, string, string) {
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
		if paths, ok := ProviderCLIPaths[providerID]; ok {
			for _, raw := range paths {
				p := expandPath(raw)
				if fileExists(p) {
					return true, "provider_cli", p
				}
			}
		}
	}

	// Check environment variables
	if envVar, ok := ProviderEnvVars[providerID]; ok {
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

func CheckProviderCredentials(providerID string) (bool, string) {
	found, source, _ := FindProviderCredential(providerID)
	return found, source
}

// CredentialStatus reports whether a provider has credentials and their source.
type CredentialStatus struct {
	HasCredentials bool
	Source         string
}

func GetAllCredentialStatus() map[string]CredentialStatus {
	status := make(map[string]CredentialStatus)
	for providerID := range ProviderCLIPaths {
		hasCreds, source := CheckProviderCredentials(providerID)
		status[providerID] = CredentialStatus{
			HasCredentials: hasCreds,
			Source:         source,
		}
	}
	return status
}

func IsFirstRun() bool {
	for providerID := range ProviderCLIPaths {
		hasCreds, _ := CheckProviderCredentials(providerID)
		if hasCreds {
			return false
		}
	}
	return true
}

func CountConfiguredProviders() int {
	count := 0
	for providerID := range ProviderCLIPaths {
		hasCreds, _ := CheckProviderCredentials(providerID)
		if hasCreds {
			count++
		}
	}
	return count
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
