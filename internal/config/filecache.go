package config

import (
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

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
