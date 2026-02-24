package modelmap

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

// ModelInfo describes a model and which providers offer it.
type ModelInfo struct {
	// ID is the canonical model identifier (e.g. "claude-sonnet-4-6").
	ID string
	// Name is the human-readable display name (e.g. "Claude Sonnet 4.6").
	Name string
	// Providers lists provider IDs that offer this model.
	Providers []string
}

var (
	initOnce       sync.Once
	registryModels map[string]ModelInfo
	registryAlias  map[string]string

	// dataLoader is the function that loads models.dev data.
	// Tests can override this to avoid network calls.
	dataLoader = loadModelsDevData
)

func ensureLoaded() {
	initOnce.Do(func() {
		data := dataLoader()
		if data != nil {
			registryModels, registryAlias = buildRegistry(data)
		}
		if registryModels == nil {
			registryModels = make(map[string]ModelInfo)
		}
		if registryAlias == nil {
			registryAlias = make(map[string]string)
		}
	})
}

// ResetForTesting clears the cached registry so the next call re-initializes.
// Only use in tests.
func ResetForTesting() {
	initOnce = sync.Once{}
	registryModels = nil
	registryAlias = nil
}

// SetLoaderForTesting overrides the data loader for tests.
// Returns a cleanup function that restores the original loader.
func SetLoaderForTesting(loader func() map[string]modelsDevProvider) func() {
	old := dataLoader
	dataLoader = loader
	ResetForTesting()
	return func() {
		dataLoader = old
		ResetForTesting()
	}
}

// Preload explicitly loads model registry data and Copilot multipliers, making
// the network fetch (if needed) happen at a known point instead of silently on
// the first Lookup call. Safe to call multiple times — subsequent calls are
// no-ops. ctx is accepted for future cancellation support.
func Preload(_ context.Context) {
	ensureLoaded()
	ensureMultipliersLoaded()
}

// CacheIsFresh reports whether both model-data cache files exist and are within
// their TTL. When false, Preload may make network calls; when true it will
// return near-instantly from disk.
func CacheIsFresh() bool {
	return cacheFileFresh(config.ModelsFile()) && cacheFileFresh(config.MultipliersFile())
}

func cacheFileFresh(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= cacheTTL
}

// Lookup resolves a model query (canonical ID or alias) to a ModelInfo.
// Returns nil if the model is not found.
func Lookup(query string) *ModelInfo {
	ensureLoaded()
	q := normalize(query)

	// Direct canonical match.
	if info, ok := registryModels[q]; ok {
		return &info
	}

	// Alias match.
	if canonical, ok := registryAlias[q]; ok {
		if info, found := registryModels[canonical]; found {
			return &info
		}
	}

	return nil
}

// Search returns all models whose ID or name contains the query substring.
// Useful for fuzzy "did you mean?" suggestions.
func Search(query string) []ModelInfo {
	ensureLoaded()
	q := normalize(query)
	var results []ModelInfo

	seen := make(map[string]bool)
	for id, info := range registryModels {
		if strings.Contains(id, q) || strings.Contains(normalize(info.Name), q) {
			if !seen[info.ID] {
				results = append(results, info)
				seen[info.ID] = true
			}
		}
	}

	// Also search aliases.
	for alias, canonical := range registryAlias {
		if strings.Contains(alias, q) {
			if info, ok := registryModels[canonical]; ok && !seen[info.ID] {
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

// MatchPrefix returns all models whose canonical ID starts with the query.
// Results are sorted by ID length (shortest first), then alphabetically.
// This is useful for expanding "claude-opus-4-5" to include dated variants
// like "claude-opus-4-5-20251101".
func MatchPrefix(query string) []ModelInfo {
	ensureLoaded()
	q := normalize(query)
	if q == "" {
		return nil
	}

	var results []ModelInfo
	seen := make(map[string]bool)

	for _, info := range registryModels {
		if strings.HasPrefix(normalize(info.ID), q) && !seen[info.ID] {
			results = append(results, info)
			seen[info.ID] = true
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if len(results[i].ID) != len(results[j].ID) {
			return len(results[i].ID) < len(results[j].ID)
		}
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
	ensureLoaded()
	var result []ModelInfo
	for _, info := range registryModels {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ListModelsForProvider returns all models available through a given provider.
func ListModelsForProvider(providerID string) []ModelInfo {
	ensureLoaded()
	var result []ModelInfo
	for _, info := range registryModels {
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
