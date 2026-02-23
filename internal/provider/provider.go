package provider

import (
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

type Provider interface {
	Meta() Metadata
	FetchStrategies() []fetch.Strategy
	FetchStatus() models.ProviderStatus
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
