package modelmap

import "strings"

// providerSources maps vibeusage provider IDs to models.dev provider IDs.
var providerSources = map[string]string{
	"claude":  "anthropic",
	"copilot": "github-copilot",
	"codex":   "openai",
	"gemini":  "google",
	"kimi":    "moonshotai",
	"minimax": "minimax",
	"zai":     "zai",
}

// inferredProviders maps vibeusage providers that aren't in models.dev to the
// models.dev provider IDs whose models they also serve.
var inferredProviders = map[string][]string{
	"antigravity": {"anthropic", "google"},
	"cursor":      {"anthropic", "openai", "google"},
}

// originProviders defines which models.dev provider is canonical for each
// model family prefix. When the same model appears in multiple providers,
// the origin provider's ID is used as the canonical model ID.
var originProviders = []string{
	"anthropic",
	"openai",
	"google",
	"moonshotai",
	"minimax",
	"zai",
}

// skipFamilies lists model families from models.dev that aren't relevant
// for routing (not chat/completion models).
var skipFamilies = map[string]bool{
	"text-embedding": true,
	"tts":            true,
	"whisper":        true,
	"dall-e":         true,
	"rerank":         true,
	"gemini":         true, // generic embedding family
}

// buildRegistry constructs the model registry from models.dev API data.
func buildRegistry(data map[string]modelsDevProvider) (models map[string]ModelInfo, aliases map[string]string) {
	models = make(map[string]ModelInfo)
	aliases = make(map[string]string)

	// Reverse map: models.dev provider ID → list of vibeusage provider IDs.
	sourceToOurs := make(map[string][]string)
	for ours, theirs := range providerSources {
		sourceToOurs[theirs] = append(sourceToOurs[theirs], ours)
	}
	for ours, sources := range inferredProviders {
		for _, src := range sources {
			sourceToOurs[src] = append(sourceToOurs[src], ours)
		}
	}

	// Track: normalized model name → canonical ID, so we can dedup across providers.
	nameToID := make(map[string]string)

	// Process origin providers first so their IDs become canonical.
	var providerOrder []string
	providerOrder = append(providerOrder, originProviders...)
	// Then secondary providers (like github-copilot).
	for _, src := range providerSources {
		found := false
		for _, o := range originProviders {
			if src == o {
				found = true
				break
			}
		}
		if !found {
			providerOrder = append(providerOrder, src)
		}
	}

	for _, srcID := range providerOrder {
		ourIDs := sourceToOurs[srcID]
		if len(ourIDs) == 0 {
			continue
		}

		provider, ok := data[srcID]
		if !ok {
			continue
		}

		for _, m := range provider.Models {
			if shouldSkipModel(m) {
				continue
			}

			nameKey := normalizeName(m.Name)
			canonicalID, exists := nameToID[nameKey]
			if exists {
				// Model already registered — add our providers and alias this ID.
				info := models[canonicalID]
				for _, pid := range ourIDs {
					if !containsStr(info.Providers, pid) {
						info.Providers = append(info.Providers, pid)
					}
				}
				models[canonicalID] = info

				// Register the provider-specific ID as an alias if different.
				if m.ID != canonicalID {
					aliases[normalize(m.ID)] = canonicalID
				}
			} else {
				// New model — register with canonical ID.
				nameToID[nameKey] = m.ID
				models[m.ID] = ModelInfo{
					ID:        m.ID,
					Name:      m.Name,
					Providers: append([]string{}, ourIDs...),
				}
			}

			// Also register the normalized ID for direct lookup.
			if m.ID != normalize(m.ID) {
				aliases[normalize(m.ID)] = m.ID
			}
		}
	}

	// Generate convenience aliases from model names.
	generateAliases(models, aliases)

	return models, aliases
}

// shouldSkipModel returns true for models that aren't useful for routing.
func shouldSkipModel(m modelsDevModel) bool {
	if skipFamilies[m.Family] {
		return true
	}

	fam := strings.ToLower(m.Family)
	id := strings.ToLower(m.ID)

	// Skip embedding, TTS, image-gen families.
	if strings.Contains(fam, "embedding") ||
		strings.Contains(fam, "tts") ||
		strings.Contains(fam, "whisper") ||
		strings.Contains(fam, "dall") ||
		strings.Contains(fam, "image") ||
		strings.Contains(fam, "rerank") {
		return true
	}

	// Skip TTS/audio/live/image-gen variants by ID.
	if strings.Contains(id, "-tts") ||
		strings.Contains(id, "-live-") ||
		strings.HasPrefix(id, "gemini-live-") ||
		strings.HasSuffix(id, "-image") ||
		strings.Contains(id, "-image-") {
		return true
	}

	return false
}

// generateAliases creates shorthand aliases for common model names.
func generateAliases(models map[string]ModelInfo, aliases map[string]string) {
	// For each model, generate dot-notation aliases.
	// e.g., "claude-sonnet-4-5" also matchable as "claude-sonnet-4.5"
	for id := range models {
		dotForm := dashToDotVersion(id)
		if dotForm != "" && dotForm != id {
			if _, conflict := models[dotForm]; !conflict {
				if _, exists := aliases[normalize(dotForm)]; !exists {
					aliases[normalize(dotForm)] = id
				}
			}
		}
	}
}

// dashToDotVersion converts version-like suffixes from dash to dot form.
// "claude-sonnet-4-5" → "claude-sonnet-4.5"
// "gpt-5-2" → "gpt-5.2"
// Only converts the last dash-digit pair to avoid mangling non-version dashes.
func dashToDotVersion(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return ""
	}

	last := parts[len(parts)-1]
	if len(last) == 0 {
		return ""
	}

	// Only convert if the last segment is a pure number (version component).
	for _, c := range last {
		if c < '0' || c > '9' {
			return ""
		}
	}

	// Also check the second-to-last is a number (so we're joining version parts).
	prev := parts[len(parts)-2]
	if len(prev) == 0 {
		return ""
	}
	for _, c := range prev {
		if c < '0' || c > '9' {
			return ""
		}
	}

	prefix := strings.Join(parts[:len(parts)-2], "-")
	if prefix != "" {
		return prefix + "-" + prev + "." + last
	}
	return prev + "." + last
}

// normalizeName produces a dedup key from a model name.
// Handles differences like "GPT-5-mini" vs "GPT-5 Mini".
func normalizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "-", " ")
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
