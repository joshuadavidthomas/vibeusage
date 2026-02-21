package modelmap

import (
	"sort"
	"strings"
)

// ModelInfo describes a model and which providers offer it.
type ModelInfo struct {
	// ID is the canonical model identifier (e.g. "claude-sonnet-4-5").
	ID string
	// Name is the human-readable display name (e.g. "Claude Sonnet 4.5").
	Name string
	// Providers lists provider IDs that offer this model.
	Providers []string
}

// Lookup resolves a model query (canonical ID or alias) to a ModelInfo.
// Returns nil if the model is not found.
func Lookup(query string) *ModelInfo {
	q := normalize(query)

	// Direct canonical match.
	if info, ok := models[q]; ok {
		return &info
	}

	// Alias match.
	if canonical, ok := aliases[q]; ok {
		if info, found := models[canonical]; found {
			return &info
		}
	}

	return nil
}

// Search returns all models whose ID or name contains the query substring.
// Useful for fuzzy "did you mean?" suggestions.
func Search(query string) []ModelInfo {
	q := normalize(query)
	var results []ModelInfo

	seen := make(map[string]bool)
	for id, info := range models {
		if strings.Contains(id, q) || strings.Contains(normalize(info.Name), q) {
			if !seen[info.ID] {
				results = append(results, info)
				seen[info.ID] = true
			}
		}
	}

	// Also search aliases.
	for alias, canonical := range aliases {
		if strings.Contains(alias, q) {
			if info, ok := models[canonical]; ok && !seen[info.ID] {
				results = append(results, info)
				seen[info.ID] = true
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results
}

// ProvidersForModel returns the provider IDs that offer the given model.
// Returns nil if the model is not found.
func ProvidersForModel(query string) []string {
	info := Lookup(query)
	if info == nil {
		return nil
	}
	result := make([]string, len(info.Providers))
	copy(result, info.Providers)
	return result
}

// ListModels returns all known models, sorted by ID.
func ListModels() []ModelInfo {
	var result []ModelInfo
	for _, info := range models {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ListModelsForProvider returns all models available through a given provider.
func ListModelsForProvider(providerID string) []ModelInfo {
	var result []ModelInfo
	for _, info := range models {
		for _, pid := range info.Providers {
			if pid == providerID {
				result = append(result, info)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
