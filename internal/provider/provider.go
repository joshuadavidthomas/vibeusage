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
