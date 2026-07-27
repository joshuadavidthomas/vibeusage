package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
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
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("caching snapshot for %s: %w", snapshot.Provider, err)
	}
	return nil
}

func LoadCachedSnapshot(providerID string) (*models.UsageSnapshot, error) {
	path := SnapshotPath(providerID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cached snapshot for %s: %w", providerID, err)
	}
	var snap models.UsageSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing cached snapshot for %s: %w", providerID, err)
	}
	return &snap, nil
}

func atomicWriteFile(path string, data []byte) (err error) {
	var (
		existingMode os.FileMode
		pathExists   bool
	)
	if info, statErr := os.Stat(path); statErr == nil {
		existingMode = info.Mode().Perm()
		pathExists = true
	} else if !os.IsNotExist(statErr) {
		return statErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if pathExists {
		if err = tmp.Chmod(existingMode); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = replaceFile(tmpPath, path); err != nil {
		return err
	}
	return nil
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
	_ = ClearThrottle(providerID)
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

func ThrottlePath(providerID string) string {
	return filepath.Join(ThrottlesDir(), providerID+".json")
}

func SaveThrottle(providerID string, marker fetch.ThrottleMarker) error {
	path := ThrottlePath(providerID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("saving throttle for %s: %w", providerID, err)
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("saving throttle for %s: %w", providerID, err)
	}
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("saving throttle for %s: %w", providerID, err)
	}
	return nil
}

// LoadThrottle returns the persisted throttle marker for the provider,
// or nil if none exists or it has expired. Expired markers are deleted
// lazily so the on-disk state stays tidy.
func LoadThrottle(providerID string) (*fetch.ThrottleMarker, error) {
	path := ThrottlePath(providerID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading throttle for %s: %w", providerID, err)
	}
	var m fetch.ThrottleMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing throttle for %s: %w", providerID, err)
	}
	if time.Now().After(m.RetryAt) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("deleting expired throttle for %s: %w", providerID, err)
		}
		return nil, nil
	}
	return &m, nil
}

func ClearThrottle(providerID string) error {
	if err := os.Remove(ThrottlePath(providerID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clearing throttle for %s: %w", providerID, err)
	}
	return nil
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
	ClearThrottles()
	ClearModelsCache()
}

func ClearThrottles() {
	entries, _ := os.ReadDir(ThrottlesDir())
	for _, e := range entries {
		_ = os.Remove(filepath.Join(ThrottlesDir(), e.Name()))
	}
}

// FileCache implements fetch.Cache using the filesystem-based snapshot
// storage. It adapts the existing CacheSnapshot/LoadCachedSnapshot functions
// to the Cache interface, enabling dependency injection in the fetch pipeline.
type FileCache struct{}

func (FileCache) Save(snapshot models.UsageSnapshot) error {
	return CacheSnapshot(snapshot)
}

func (FileCache) Load(providerID string) (*models.UsageSnapshot, error) {
	return LoadCachedSnapshot(providerID)
}

// FileThrottleStore implements fetch.ThrottleStore on top of the
// filesystem-based throttle marker helpers.
type FileThrottleStore struct{}

func (FileThrottleStore) Load(providerID string) (*fetch.ThrottleMarker, error) {
	return LoadThrottle(providerID)
}

func (FileThrottleStore) Save(providerID string, marker fetch.ThrottleMarker) error {
	return SaveThrottle(providerID, marker)
}

func (FileThrottleStore) Clear(providerID string) error {
	return ClearThrottle(providerID)
}
