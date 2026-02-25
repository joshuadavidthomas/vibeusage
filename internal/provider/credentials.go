package provider

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

// APIKeySource describes where to find an API key for a provider. Declare one
// per provider and call Load() from both IsAvailable and Fetch.
type APIKeySource struct {
	EnvVars  []string // environment variables to check, in order
	CredPath string   // credential file path
	JSONKeys []string // JSON keys to try within the credential file
}

// Load checks environment variables and then the credential file for an API
// key. Returns the first non-empty value found, or "".
func (s APIKeySource) Load() string {
	for _, env := range s.EnvVars {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	data, err := config.ReadCredential(s.CredPath)
	if err != nil || data == nil {
		return ""
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	for _, key := range s.JSONKeys {
		if v := strings.TrimSpace(raw[key]); v != "" {
			return v
		}
	}
	return ""
}

// ExternalCredentialReuseEnabled reports whether provider strategies should
// consider credentials managed outside vibeusage storage (CLI files, keychains,
// editor state DBs, etc.).
func ExternalCredentialReuseEnabled() bool {
	return config.Get().Credentials.ReuseProviderCredentials
}

// CredentialSearchPaths returns the canonical vibeusage credential path first,
// followed by external credential paths when reuse is enabled.
func CredentialSearchPaths(providerID, credType string, external ...string) []string {
	paths := []string{config.CredentialPath(providerID, credType)}
	if !ExternalCredentialReuseEnabled() {
		return paths
	}
	return append(paths, external...)
}

// FindCredential checks for credentials for the given provider in
// vibeusage storage, provider CLI paths, and environment variables.
// Returns (found, source, path).
func FindCredential(providerID string) (bool, string, string) {
	p, ok := Get(providerID)
	if !ok {
		return false, "", ""
	}
	info := p.CredentialSources()
	return config.FindProviderCredential(providerID, info.CLIPaths, info.EnvVars)
}

// CheckCredentials reports whether the given provider has credentials
// and where they came from.
func CheckCredentials(providerID string) (bool, string) {
	found, source, _ := FindCredential(providerID)
	if found {
		return true, source
	}

	if !ExternalCredentialReuseEnabled() {
		return false, ""
	}

	p, ok := Get(providerID)
	if !ok {
		return false, ""
	}

	for _, s := range p.FetchStrategies() {
		if s.IsAvailable() {
			return true, "provider_cli"
		}
	}

	return false, ""
}

// GetAllCredentialStatus returns the credential status for every
// registered provider.
func GetAllCredentialStatus() map[string]config.CredentialStatus {
	all := All()
	status := make(map[string]config.CredentialStatus, len(all))
	for id := range all {
		hasCreds, source := CheckCredentials(id)
		status[id] = config.CredentialStatus{
			HasCredentials: hasCreds,
			Source:         source,
		}
	}
	return status
}

// IsFirstRun returns true if no registered provider has credentials.
func IsFirstRun() bool {
	for id := range All() {
		hasCreds, _ := CheckCredentials(id)
		if hasCreds {
			return false
		}
	}
	return true
}

// CountConfigured returns the number of registered providers that
// have credentials.
func CountConfigured() int {
	count := 0
	for id := range All() {
		hasCreds, _ := CheckCredentials(id)
		if hasCreds {
			count++
		}
	}
	return count
}
