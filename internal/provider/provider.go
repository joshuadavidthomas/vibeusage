package provider

import (
	"context"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

type Metadata struct {
	ID           string
	Name         string
	Description  string
	Homepage     string
	StatusURL    string
	DashboardURL string
}

// CredentialInfo describes where a provider's credentials can be found
// outside of vibeusage's own storage.
type CredentialInfo struct {
	// CLIPaths are external CLI credential file paths (e.g. ~/.claude/.credentials.json).
	CLIPaths []string
	// EnvVars are environment variable names (e.g. ANTHROPIC_API_KEY).
	EnvVars []string
}

type Provider interface {
	Meta() Metadata
	CredentialSources() CredentialInfo
	FetchStrategies() []fetch.Strategy
	FetchStatus(ctx context.Context) models.ProviderStatus
}

var registry = map[string]Provider{}

func Register(p Provider) {
	registry[p.Meta().ID] = p
}

func Get(id string) (Provider, bool) {
	p, ok := registry[id]
	return p, ok
}

func All() map[string]Provider {
	result := make(map[string]Provider, len(registry))
	for k, v := range registry {
		result[k] = v
	}
	return result
}

func ListIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}

// ConfiguredIDs filters a list of provider IDs to only those that are
// registered and have at least one available fetch strategy.
func ConfiguredIDs(providerIDs []string) []string {
	var result []string
	for _, pid := range providerIDs {
		p, ok := Get(pid)
		if !ok {
			continue
		}
		for _, s := range p.FetchStrategies() {
			if s.IsAvailable() {
				result = append(result, pid)
				break
			}
		}
	}
	return result
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
	return found, source
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
