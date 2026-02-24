package routing

import (
	"context"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

// routingStubProvider is a minimal Provider for routing-package tests.
type routingStubProvider struct {
	id   string
	name string
}

func (s routingStubProvider) Meta() provider.Metadata {
	return provider.Metadata{ID: s.id, Name: s.name}
}

func (s routingStubProvider) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{}
}

func (s routingStubProvider) FetchStrategies() []fetch.Strategy {
	return nil
}

func (s routingStubProvider) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{}
}

func TestMain(m *testing.M) {
	// Register providers used by routing tests so DisplayName resolves correctly.
	for _, p := range []routingStubProvider{
		{id: "claude", name: "Claude"},
		{id: "copilot", name: "Copilot"},
		{id: "cursor", name: "Cursor"},
		{id: "codex", name: "Codex"},
	} {
		provider.Register(p)
	}
	os.Exit(m.Run())
}
