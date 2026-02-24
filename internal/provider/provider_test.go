package provider

import (
	"context"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// stubProvider implements Provider for tests.
type stubProvider struct {
	id         string
	name       string // if empty, defaults to id
	strategies []fetch.Strategy
	creds      CredentialInfo
}

func (s *stubProvider) Meta() Metadata {
	name := s.name
	if name == "" {
		name = s.id
	}
	return Metadata{ID: s.id, Name: name}
}

func (s *stubProvider) CredentialSources() CredentialInfo {
	return s.creds
}

func (s *stubProvider) FetchStrategies() []fetch.Strategy {
	return s.strategies
}

func (s *stubProvider) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{}
}

// stubStrategy implements fetch.Strategy for tests.
type stubStrategy struct {
	available bool
}

func (s *stubStrategy) Name() string      { return "stub" }
func (s *stubStrategy) IsAvailable() bool { return s.available }
func (s *stubStrategy) Fetch(_ context.Context) (fetch.FetchResult, error) {
	return fetch.FetchResult{}, nil
}

func TestConfiguredIDs_FiltersToRegisteredAndAvailable(t *testing.T) {
	// Save and restore registry state.
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	Register(&stubProvider{
		id:         "alpha",
		strategies: []fetch.Strategy{&stubStrategy{available: true}},
	})
	Register(&stubProvider{
		id:         "beta",
		strategies: []fetch.Strategy{&stubStrategy{available: false}},
	})
	// "gamma" is not registered at all.

	got := ConfiguredIDs([]string{"alpha", "beta", "gamma"})

	if len(got) != 1 {
		t.Fatalf("expected 1 configured, got %d: %v", len(got), got)
	}
	if got[0] != "alpha" {
		t.Errorf("got %q, want alpha", got[0])
	}
}

func TestConfiguredIDs_Empty(t *testing.T) {
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	got := ConfiguredIDs([]string{"anything"})
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestConfiguredIDs_MultipleStrategiesOnlyNeedsOne(t *testing.T) {
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	Register(&stubProvider{
		id: "multi",
		strategies: []fetch.Strategy{
			&stubStrategy{available: false},
			&stubStrategy{available: true},
		},
	})

	got := ConfiguredIDs([]string{"multi"})
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0] != "multi" {
		t.Errorf("got %q, want multi", got[0])
	}
}

func TestDisplayName_KnownProvider(t *testing.T) {
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	Register(&stubProvider{id: "zai", name: "Z.ai"})
	Register(&stubProvider{id: "claude", name: "Claude"})

	if got := DisplayName("zai"); got != "Z.ai" {
		t.Errorf("DisplayName(%q) = %q, want %q", "zai", got, "Z.ai")
	}
	if got := DisplayName("claude"); got != "Claude" {
		t.Errorf("DisplayName(%q) = %q, want %q", "claude", got, "Claude")
	}
}

func TestDisplayName_UnknownProvider(t *testing.T) {
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	// Unknown ID falls back to the ID itself.
	if got := DisplayName("unknown"); got != "unknown" {
		t.Errorf("DisplayName(%q) = %q, want %q", "unknown", got, "unknown")
	}
}

func TestDisplayName_Empty(t *testing.T) {
	orig := registry
	registry = map[string]Provider{}
	defer func() { registry = orig }()

	if got := DisplayName(""); got != "" {
		t.Errorf("DisplayName(%q) = %q, want %q", "", got, "")
	}
}
