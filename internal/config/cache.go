package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func SnapshotPath(providerID string) string {
	return filepath.Join(SnapshotsDir(), providerID+".json")
}

func CacheSnapshot(snapshot models.UsageSnapshot) error {
	path := SnapshotPath(snapshot.Provider)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("caching snapshot for %s: %w", snapshot.Provider, err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("caching snapshot for %s: %w", snapshot.Provider, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("caching snapshot for %s: %w", snapshot.Provider, err)
	}
	return nil
}

func LoadCachedSnapshot(providerID string) *models.UsageSnapshot {
	path := SnapshotPath(providerID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var snap models.UsageSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil
	}
	return &snap
}

// Org ID caching
func OrgIDPath(providerID string) string {
	return filepath.Join(OrgIDsDir(), providerID+".txt")
}

func CacheOrgID(providerID, orgID string) error {
	path := OrgIDPath(providerID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("caching org ID for %s: %w", providerID, err)
	}
	if err := os.WriteFile(path, []byte(orgID), 0o644); err != nil {
		return fmt.Errorf("caching org ID for %s: %w", providerID, err)
	}
	return nil
}

func LoadCachedOrgID(providerID string) string {
	data, err := os.ReadFile(OrgIDPath(providerID))
	if err != nil {
		return ""
	}
	return string(data)
}

func ClearOrgIDCache(providerID string) {
	if providerID != "" {
		_ = os.Remove(OrgIDPath(providerID))
		return
	}
	entries, _ := os.ReadDir(OrgIDsDir())
	for _, e := range entries {
		_ = os.Remove(filepath.Join(OrgIDsDir(), e.Name()))
	}
}

func ClearProviderCache(providerID string) {
	_ = os.Remove(SnapshotPath(providerID))
	_ = os.Remove(OrgIDPath(providerID))
}

func ClearSnapshotCache(providerID string) {
	if providerID != "" {
		_ = os.Remove(SnapshotPath(providerID))
		return
	}
	entries, _ := os.ReadDir(SnapshotsDir())
	for _, e := range entries {
		_ = os.Remove(filepath.Join(SnapshotsDir(), e.Name()))
	}
}

func ClearModelsCache() {
	_ = os.Remove(ModelsFile())
	_ = os.Remove(MultipliersFile())
}

func ClearAllCache(providerID string) {
	if providerID != "" {
		ClearProviderCache(providerID)
		return
	}
	ClearSnapshotCache("")
	ClearOrgIDCache("")
	ClearModelsCache()
}

// FileCache implements fetch.Cache using the filesystem-based snapshot
// storage. It adapts the existing CacheSnapshot/LoadCachedSnapshot functions
// to the Cache interface, enabling dependency injection in the fetch pipeline.
type FileCache struct{}

func (FileCache) Save(snapshot models.UsageSnapshot) error {
	return CacheSnapshot(snapshot)
}

func (FileCache) Load(providerID string) *models.UsageSnapshot {
	return LoadCachedSnapshot(providerID)
}
