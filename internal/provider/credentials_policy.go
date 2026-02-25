package provider

import "github.com/joshuadavidthomas/vibeusage/internal/config"

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
