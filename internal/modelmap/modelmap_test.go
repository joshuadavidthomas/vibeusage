package modelmap

import (
	"testing"
)

// testData returns a minimal models.dev-shaped fixture for testing.
func testData() map[string]modelsDevProvider {
	return map[string]modelsDevProvider{
		"anthropic": {
			ID:   "anthropic",
			Name: "Anthropic",
			Models: map[string]modelsDevModel{
				"claude-sonnet-4-5": {ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Family: "claude-sonnet"},
				"claude-sonnet-4-6": {ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Family: "claude-sonnet"},
				"claude-opus-4-0":   {ID: "claude-opus-4-0", Name: "Claude Opus 4", Family: "claude-opus"},
				"claude-haiku-4-5":  {ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Family: "claude-haiku"},
			},
		},
		"openai": {
			ID:   "openai",
			Name: "OpenAI",
			Models: map[string]modelsDevModel{
				"gpt-4o":                 {ID: "gpt-4o", Name: "GPT-4o", Family: "gpt"},
				"gpt-5.2":                {ID: "gpt-5.2", Name: "GPT-5.2", Family: "gpt"},
				"o4-mini":                {ID: "o4-mini", Name: "o4-mini", Family: "o-mini"},
				"text-embedding-3-small": {ID: "text-embedding-3-small", Name: "text-embedding-3-small", Family: "text-embedding"},
			},
		},
		"google": {
			ID:   "google",
			Name: "Google",
			Models: map[string]modelsDevModel{
				"gemini-2.5-pro":   {ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Family: "gemini-pro"},
				"gemini-2.5-flash": {ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Family: "gemini-flash"},
			},
		},
		"github-copilot": {
			ID:   "github-copilot",
			Name: "GitHub Copilot",
			Models: map[string]modelsDevModel{
				"claude-sonnet-4.5": {ID: "claude-sonnet-4.5", Name: "Claude Sonnet 4.5", Family: "claude-sonnet"},
				"claude-sonnet-4.6": {ID: "claude-sonnet-4.6", Name: "Claude Sonnet 4.6", Family: "claude-sonnet"},
				"gpt-4o":            {ID: "gpt-4o", Name: "GPT-4o", Family: "gpt"},
				"gpt-5.2":           {ID: "gpt-5.2", Name: "GPT-5.2", Family: "gpt"},
				"gemini-2.5-pro":    {ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Family: "gemini-pro"},
			},
		},
		"moonshotai": {
			ID:   "moonshotai",
			Name: "Moonshot AI",
			Models: map[string]modelsDevModel{
				"kimi-k2.5": {ID: "kimi-k2.5", Name: "Kimi K2.5", Family: "kimi"},
			},
		},
		"minimax": {
			ID:   "minimax",
			Name: "MiniMax",
			Models: map[string]modelsDevModel{
				"MiniMax-M2.5": {ID: "MiniMax-M2.5", Name: "MiniMax-M2.5", Family: "minimax"},
			},
		},
		"zai": {
			ID:   "zai",
			Name: "Z.AI",
			Models: map[string]modelsDevModel{
				"glm-4.7": {ID: "glm-4.7", Name: "GLM-4.7", Family: "glm"},
			},
		},
	}
}

func setupTest(t *testing.T) {
	t.Helper()
	cleanup := SetLoaderForTesting(testData)
	t.Cleanup(cleanup)
}

func TestLookup_CanonicalID(t *testing.T) {
	setupTest(t)

	info := Lookup("claude-sonnet-4-6")
	if info == nil {
		t.Fatal("expected model info, got nil")
	}
	if info.ID != "claude-sonnet-4-6" {
		t.Errorf("ID = %q, want %q", info.ID, "claude-sonnet-4-6")
	}
	if info.Name != "Claude Sonnet 4.6" {
		t.Errorf("Name = %q, want %q", info.Name, "Claude Sonnet 4.6")
	}
	if len(info.Providers) == 0 {
		t.Error("expected providers, got none")
	}
}

func TestLookup_Alias(t *testing.T) {
	setupTest(t)

	// Copilot uses "claude-sonnet-4.6" but canonical is "claude-sonnet-4-6".
	info := Lookup("claude-sonnet-4.6")
	if info == nil {
		t.Fatal("expected alias match, got nil")
	}
	if info.ID != "claude-sonnet-4-6" {
		t.Errorf("ID = %q, want canonical %q", info.ID, "claude-sonnet-4-6")
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	setupTest(t)

	info := Lookup("CLAUDE-SONNET-4-6")
	if info == nil {
		t.Fatal("expected case-insensitive match")
	}
	if info.ID != "claude-sonnet-4-6" {
		t.Errorf("ID = %q, want %q", info.ID, "claude-sonnet-4-6")
	}
}

func TestLookup_NotFound(t *testing.T) {
	setupTest(t)

	info := Lookup("nonexistent-model")
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestLookup_SkipsEmbeddings(t *testing.T) {
	setupTest(t)

	info := Lookup("text-embedding-3-small")
	if info != nil {
		t.Errorf("expected nil for embedding model, got %+v", info)
	}
}

func TestProvidersForModel(t *testing.T) {
	setupTest(t)

	providers := ProvidersForModel("claude-sonnet-4-5")
	if len(providers) == 0 {
		t.Fatal("expected providers")
	}

	has := func(pid string) bool {
		for _, p := range providers {
			if p == pid {
				return true
			}
		}
		return false
	}
	if !has("claude") {
		t.Error("expected claude in providers")
	}
	if !has("copilot") {
		t.Error("expected copilot in providers")
	}
	// Antigravity infers from anthropic.
	if !has("antigravity") {
		t.Error("expected antigravity in providers (inferred from anthropic)")
	}
	// Cursor infers from anthropic.
	if !has("cursor") {
		t.Error("expected cursor in providers (inferred from anthropic)")
	}
}

func TestProvidersForModel_NotFound(t *testing.T) {
	setupTest(t)

	providers := ProvidersForModel("nonexistent")
	if providers != nil {
		t.Errorf("expected nil, got %v", providers)
	}
}

func TestSearch(t *testing.T) {
	setupTest(t)

	results := Search("sonnet")
	if len(results) == 0 {
		t.Fatal("expected search results for 'sonnet'")
	}

	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.ID] = true
	}

	if !ids["claude-sonnet-4-5"] {
		t.Error("expected claude-sonnet-4-5 in results")
	}
	if !ids["claude-sonnet-4-6"] {
		t.Error("expected claude-sonnet-4-6 in results")
	}
}

func TestSearch_NoResults(t *testing.T) {
	setupTest(t)

	results := Search("zzzzz-nonexistent")
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestListModels(t *testing.T) {
	setupTest(t)

	all := ListModels()
	if len(all) == 0 {
		t.Fatal("expected models")
	}

	// Verify sorted.
	for i := 1; i < len(all); i++ {
		if all[i].ID < all[i-1].ID {
			t.Errorf("not sorted: %q before %q", all[i-1].ID, all[i].ID)
		}
	}
}

func TestListModelsForProvider(t *testing.T) {
	setupTest(t)

	claudeModels := ListModelsForProvider("claude")
	if len(claudeModels) == 0 {
		t.Fatal("expected claude models")
	}

	for _, m := range claudeModels {
		found := false
		for _, pid := range m.Providers {
			if pid == "claude" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("model %q doesn't list claude as provider", m.ID)
		}
	}
}

func TestListModelsForProvider_Unknown(t *testing.T) {
	setupTest(t)

	result := ListModelsForProvider("nonexistent")
	if len(result) != 0 {
		t.Errorf("expected no models, got %d", len(result))
	}
}

func TestRegistryConsistency(t *testing.T) {
	setupTest(t)
	ensureLoaded()

	// Every alias should resolve to a valid canonical model.
	for alias, canonical := range registryAlias {
		if _, ok := registryModels[canonical]; !ok {
			t.Errorf("alias %q points to non-existent model %q", alias, canonical)
		}
	}

	// Every model should have at least one provider.
	for id, info := range registryModels {
		if len(info.Providers) == 0 {
			t.Errorf("model %q has no providers", id)
		}
		if info.ID != id {
			t.Errorf("model key %q has mismatched ID %q", id, info.ID)
		}
		if info.Name == "" {
			t.Errorf("model %q has empty name", id)
		}
	}
}

func TestProviderMerging(t *testing.T) {
	setupTest(t)

	// GPT-4o should be available from codex (openai) and copilot (github-copilot)
	// and also cursor (inferred from openai).
	info := Lookup("gpt-4o")
	if info == nil {
		t.Fatal("expected gpt-4o, got nil")
	}

	has := func(pid string) bool {
		for _, p := range info.Providers {
			if p == pid {
				return true
			}
		}
		return false
	}

	if !has("codex") {
		t.Error("expected codex in gpt-4o providers")
	}
	if !has("copilot") {
		t.Error("expected copilot in gpt-4o providers")
	}
	if !has("cursor") {
		t.Error("expected cursor in gpt-4o providers (inferred from openai)")
	}
}

func TestInferredProviders(t *testing.T) {
	setupTest(t)

	// Gemini models should be available from antigravity (inferred from google).
	info := Lookup("gemini-2.5-pro")
	if info == nil {
		t.Fatal("expected gemini-2.5-pro, got nil")
	}

	has := func(pid string) bool {
		for _, p := range info.Providers {
			if p == pid {
				return true
			}
		}
		return false
	}

	if !has("gemini") {
		t.Error("expected gemini in providers")
	}
	if !has("antigravity") {
		t.Error("expected antigravity in providers (inferred from google)")
	}
	if !has("cursor") {
		t.Error("expected cursor in providers (inferred from google)")
	}
}

func TestEmptyData(t *testing.T) {
	cleanup := SetLoaderForTesting(func() map[string]modelsDevProvider {
		return nil
	})
	t.Cleanup(cleanup)

	all := ListModels()
	if len(all) != 0 {
		t.Errorf("expected 0 models with nil data, got %d", len(all))
	}

	info := Lookup("anything")
	if info != nil {
		t.Errorf("expected nil lookup with no data, got %+v", info)
	}
}
