package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func SnapshotPath(providerID string) string {
	return filepath.Join(SnapshotsDir(), providerID+".json")
}

func CacheSnapshot(snapshot models.UsageSnapshot) error {
	path := SnapshotPath(snapshot.Provider)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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

func IsSnapshotFresh(providerID string, staleMinutes int) bool {
	snap := LoadCachedSnapshot(providerID)
	if snap == nil {
		return false
	}
	return time.Since(snap.FetchedAt).Minutes() < float64(staleMinutes)
}

func GetSnapshotAgeMinutes(providerID string) *int {
	snap := LoadCachedSnapshot(providerID)
	if snap == nil {
		return nil
	}
	age := int(time.Since(snap.FetchedAt).Minutes())
	return &age
}

// Org ID caching
func OrgIDPath(providerID string) string {
	return filepath.Join(OrgIDsDir(), providerID+".txt")
}

func CacheOrgID(providerID, orgID string) error {
	path := OrgIDPath(providerID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(orgID), 0o644)
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
		os.Remove(OrgIDPath(providerID))
		return
	}
	entries, _ := os.ReadDir(OrgIDsDir())
	for _, e := range entries {
		os.Remove(filepath.Join(OrgIDsDir(), e.Name()))
	}
}

func ClearProviderCache(providerID string) {
	os.Remove(SnapshotPath(providerID))
	os.Remove(OrgIDPath(providerID))
}

func ClearSnapshotCache(providerID string) {
	if providerID != "" {
		os.Remove(SnapshotPath(providerID))
		return
	}
	entries, _ := os.ReadDir(SnapshotsDir())
	for _, e := range entries {
		os.Remove(filepath.Join(SnapshotsDir(), e.Name()))
	}
}

func ClearAllCache(providerID string) {
	if providerID != "" {
		ClearProviderCache(providerID)
		return
	}
	ClearSnapshotCache("")
	ClearOrgIDCache("")
}
