package modelmap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

const multipliersURL = "https://raw.githubusercontent.com/github/docs/main/data/tables/copilot/model-multipliers.yml"

// ModelMultiplier holds the premium request multipliers for a Copilot model.
type ModelMultiplier struct {
	Name string   `json:"name"`
	Paid *float64 `json:"paid,omitempty"` // nil = "Not applicable"
	Free *float64 `json:"free,omitempty"` // nil = "Not applicable"
}

var (
	multipliersOnce   sync.Once
	multipliersByName map[string]ModelMultiplier
)

func ensureMultipliersLoaded() {
	multipliersOnce.Do(func() {
		multipliersByName = loadMultipliers()
		if multipliersByName == nil {
			multipliersByName = make(map[string]ModelMultiplier)
		}
	})
}

// LookupMultiplier returns the paid-plan multiplier for a model on copilot.
// Returns nil if the model has no multiplier data (non-copilot provider or
// unknown model). Returns a pointer to 0 for free models (0x cost).
func LookupMultiplier(modelName string) *float64 {
	ensureMultipliersLoaded()

	// Try exact match first.
	if m, ok := multipliersByName[modelName]; ok {
		return m.Paid
	}

	// Try normalized match (case-insensitive, hyphens = spaces).
	key := normalizeName(modelName)
	for name, m := range multipliersByName {
		if normalizeName(name) == key {
			return m.Paid
		}
	}

	return nil
}

// ResetMultipliersForTesting clears cached multiplier data.
func ResetMultipliersForTesting() {
	multipliersOnce = sync.Once{}
	multipliersByName = nil
}

func loadMultipliers() map[string]ModelMultiplier {
	path := config.MultipliersFile()

	if data := readMultipliersCacheIfFresh(path); data != nil {
		return data
	}

	raw, err := fetchMultipliersYAML()
	if err != nil {
		// Network failed — serve stale cache.
		if data := readMultipliersCache(path); data != nil {
			return data
		}
		return nil
	}

	entries := parseMultipliersYAML(raw)
	if entries == nil {
		return nil
	}

	_ = writeMultipliersCache(path, entries)
	return indexByName(entries)
}

func readMultipliersCacheIfFresh(path string) map[string]ModelMultiplier {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil
	}
	return readMultipliersCache(path)
}

func readMultipliersCache(path string) map[string]ModelMultiplier {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []ModelMultiplier
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return indexByName(entries)
}

func writeMultipliersCache(path string, entries []ModelMultiplier) error {
	if err := os.MkdirAll(config.CacheDir(), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fetchMultipliersYAML() (string, error) {
	client := httpclient.NewWithTimeout(15 * time.Second)
	resp, err := client.DoCtx(context.Background(), "GET", multipliersURL, nil)
	if err != nil {
		return "", fmt.Errorf("fetching copilot multipliers: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetching copilot multipliers: HTTP %d", resp.StatusCode)
	}
	return string(resp.Body), nil
}

// yamlMultiplierEntry is the raw YAML shape from github/docs. Paid and Free are
// interface{} because the source mixes bare numbers (int/float) with the string
// "Not applicable" in the same field.
type yamlMultiplierEntry struct {
	Name string      `yaml:"name"`
	Paid interface{} `yaml:"multiplier_paid"`
	Free interface{} `yaml:"multiplier_free"`
}

// parseMultipliersYAML parses the YAML format from github/docs.
// Each entry is:
//
//   - name: MODEL_NAME
//     multiplier_paid: NUMBER_OR_NOT_APPLICABLE
//     multiplier_free: NUMBER_OR_NOT_APPLICABLE
func parseMultipliersYAML(raw string) []ModelMultiplier {
	var rows []yamlMultiplierEntry
	if err := yaml.Unmarshal([]byte(raw), &rows); err != nil {
		return nil
	}

	entries := make([]ModelMultiplier, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, ModelMultiplier{
			Name: r.Name,
			Paid: convertYAMLMultiplier(r.Paid),
			Free: convertYAMLMultiplier(r.Free),
		})
	}
	return entries
}

// convertYAMLMultiplier converts a yaml.v3 scalar value (int, float64, or
// string) to a *float64. Returns nil for "Not applicable" or unrecognised
// values.
func convertYAMLMultiplier(v interface{}) *float64 {
	switch val := v.(type) {
	case int:
		f := float64(val)
		return &f
	case float64:
		return &val
	case string:
		return parseMultiplierValue(val)
	default:
		return nil
	}
}

func parseMultiplierValue(s string) *float64 {
	if strings.EqualFold(s, "not applicable") || s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func indexByName(entries []ModelMultiplier) map[string]ModelMultiplier {
	m := make(map[string]ModelMultiplier, len(entries))
	for _, e := range entries {
		m[e.Name] = e
	}
	return m
}
