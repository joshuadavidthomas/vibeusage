package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// ReadEnabledProviders loads the list of enabled provider IDs from the
// data directory. Returns nil if the file does not exist.
func ReadEnabledProviders() []string {
	data, err := os.ReadFile(EnabledProvidersFile())
	if err != nil {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil
	}
	return ids
}

// WriteEnabledProviders persists the list of enabled provider IDs to the
// data directory.
func WriteEnabledProviders(ids []string) error {
	sort.Strings(ids)
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	path := EnabledProvidersFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AddEnabledProvider adds a provider ID to the enabled list if not
// already present. Returns true if the provider was added.
func AddEnabledProvider(providerID string) bool {
	ids := ReadEnabledProviders()
	for _, id := range ids {
		if id == providerID {
			return false
		}
	}
	ids = append(ids, providerID)
	_ = WriteEnabledProviders(ids)
	return true
}
