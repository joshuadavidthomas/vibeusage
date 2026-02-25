package display

import (
	"context"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

// displayStubProvider is a minimal Provider for display-package tests.
type displayStubProvider struct {
	id   string
	name string
}

func (s displayStubProvider) Meta() provider.Metadata {
	return provider.Metadata{ID: s.id, Name: s.name}
}

func (s displayStubProvider) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{}
}

func (s displayStubProvider) FetchStrategies() []fetch.Strategy {
	return nil
}

func (s displayStubProvider) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{}
}

func TestMain(m *testing.M) {
	// Register providers used by display tests so DisplayName resolves correctly.
	for _, p := range []displayStubProvider{
		{id: "claude", name: "Claude"},
		{id: "codex", name: "Codex"},
		{id: "copilot", name: "Copilot"},
		{id: "cursor", name: "Cursor"},
		{id: "empty", name: "Empty"},
	} {
		provider.Register(p)
	}
	os.Exit(m.Run())
}
