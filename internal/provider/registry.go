package provider

import (
	"context"
	"sort"

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

// AvailableIDs returns registered provider IDs that are enabled in the given
// config and have at least one available fetch strategy. The result is sorted.
func AvailableIDs(cfg config.Config) []string {
	var ids []string
	for id, p := range registry {
		if !cfg.IsProviderEnabled(id) {
			continue
		}
		for _, s := range p.FetchStrategies() {
			if s.IsAvailable() {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// DisplayName returns the human-readable display name for the given
// provider ID by looking it up in the registry. If the ID is not
// registered, it returns the ID itself as a fallback.
func DisplayName(id string) string {
	p, ok := Get(id)
	if !ok {
		return id
	}
	return p.Meta().Name
}
