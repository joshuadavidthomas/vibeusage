package modelmap

import (
	"testing"
)

func TestLookup_CanonicalID(t *testing.T) {
	info := Lookup("claude-sonnet-4-5")
	if info == nil {
		t.Fatal("expected model info, got nil")
	}
	if info.ID != "claude-sonnet-4-5" {
		t.Errorf("ID = %q, want %q", info.ID, "claude-sonnet-4-5")
	}
	if info.Name != "Claude Sonnet 4.5" {
		t.Errorf("Name = %q, want %q", info.Name, "Claude Sonnet 4.5")
	}
	if len(info.Providers) == 0 {
		t.Error("expected providers, got none")
	}
}

func TestLookup_Alias(t *testing.T) {
	tests := []struct {
		query    string
		wantID   string
		wantName string
	}{
		{"sonnet-4.5", "claude-sonnet-4-5", "Claude Sonnet 4.5"},
		{"sonnet", "claude-sonnet-4", "Claude Sonnet 4"},
		{"opus", "claude-opus-4", "Claude Opus 4"},
		{"haiku", "claude-haiku-3-5", "Claude Haiku 3.5"},
		{"4o", "gpt-4o", "GPT-4o"},
		{"gemini", "gemini-2-5-pro", "Gemini 2.5 Pro"},
		{"flash", "gemini-2-5-flash", "Gemini 2.5 Flash"},
		{"kimi", "k2-5", "Kimi K2.5"},
		{"m2.5", "minimax-m2-5", "MiniMax M2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			info := Lookup(tt.query)
			if info == nil {
				t.Fatalf("Lookup(%q) returned nil", tt.query)
			}
			if info.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", info.ID, tt.wantID)
			}
			if info.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", info.Name, tt.wantName)
			}
		})
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	info := Lookup("SONNET-4.5")
	if info == nil {
		t.Fatal("expected case-insensitive match")
	}
	if info.ID != "claude-sonnet-4-5" {
		t.Errorf("ID = %q, want %q", info.ID, "claude-sonnet-4-5")
	}
}

func TestLookup_NotFound(t *testing.T) {
	info := Lookup("nonexistent-model")
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestProvidersForModel(t *testing.T) {
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
}

func TestProvidersForModel_NotFound(t *testing.T) {
	providers := ProvidersForModel("nonexistent")
	if providers != nil {
		t.Errorf("expected nil, got %v", providers)
	}
}

func TestSearch(t *testing.T) {
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
	if !ids["claude-sonnet-4"] {
		t.Error("expected claude-sonnet-4 in results")
	}
}

func TestSearch_NoResults(t *testing.T) {
	results := Search("zzzzz-nonexistent")
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestSearch_MatchesAlias(t *testing.T) {
	results := Search("haiku")
	if len(results) == 0 {
		t.Fatal("expected search results for alias 'haiku'")
	}

	found := false
	for _, r := range results {
		if r.ID == "claude-haiku-3-5" {
			found = true
		}
	}
	if !found {
		t.Error("expected claude-haiku-3-5 in alias search results")
	}
}

func TestListModels(t *testing.T) {
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
	result := ListModelsForProvider("nonexistent")
	if len(result) != 0 {
		t.Errorf("expected no models, got %d", len(result))
	}
}

func TestRegistryConsistency(t *testing.T) {
	// Every alias should resolve to a valid canonical model.
	for alias, canonical := range aliases {
		if _, ok := models[canonical]; !ok {
			t.Errorf("alias %q points to non-existent model %q", alias, canonical)
		}
	}

	// Every model should have at least one provider.
	for id, info := range models {
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
