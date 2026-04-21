package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

const latestReleaseCacheFilename = "update-check.json"

// LatestReleaseInfo stores the latest known release metadata for lightweight
// update checks in normal CLI output.
type LatestReleaseInfo struct {
	// CheckedAt is the last successful live check time.
	CheckedAt time.Time `json:"checked_at,omitempty"`
	// AttemptedAt is the last live-check attempt time, whether it succeeded or failed.
	AttemptedAt   time.Time `json:"attempted_at,omitempty"`
	LatestVersion string    `json:"latest_version,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
}

// AvailableUpdate describes an update available for the current version.
type AvailableUpdate struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
}

// NewCheckClient creates a GitHub-backed client with a short timeout for
// lightweight latest-release checks in normal CLI output.
func NewCheckClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = time.Second
	}
	c := NewClient()
	c.HTTP = &http.Client{Timeout: timeout}
	return c
}

// LatestReleaseCachePath returns the path to the lightweight update-check
// cache file.
func LatestReleaseCachePath() string {
	return filepath.Join(config.CacheDir(), latestReleaseCacheFilename)
}

// LoadLatestReleaseCache returns the cached latest-release metadata, or nil
// when absent or malformed.
func LoadLatestReleaseCache() *LatestReleaseInfo {
	data, err := os.ReadFile(LatestReleaseCachePath())
	if err != nil {
		return nil
	}

	var entry LatestReleaseInfo
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if entry.AttemptedAt.IsZero() && entry.CheckedAt.IsZero() {
		return nil
	}
	if entry.AttemptedAt.IsZero() && !entry.CheckedAt.IsZero() {
		entry.AttemptedAt = entry.CheckedAt
	}
	return &entry
}

// SaveLatestReleaseCache persists the latest-release metadata for reuse by
// future commands.
func SaveLatestReleaseCache(entry LatestReleaseInfo) error {
	path := LatestReleaseCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("saving update-check cache: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("saving update-check cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("saving update-check cache: %w", err)
	}
	return nil
}

// AttemptExpired reports whether the last live-check attempt is older than the
// supplied interval. A zero AttemptedAt is treated as expired.
func (e LatestReleaseInfo) AttemptExpired(interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	if e.AttemptedAt.IsZero() {
		return true
	}
	return time.Since(e.AttemptedAt) >= interval
}

// IsReleaseVersion reports whether the version looks like a tagged semver
// release suitable for update comparisons.
func IsReleaseVersion(v string) bool {
	normalized := normalizeVersion(v)
	if normalized == "" {
		return false
	}
	return semver.IsValid("v" + normalized)
}

// AvailableUpdateForVersion reports whether the current version is behind the
// latest known release.
func AvailableUpdateForVersion(currentVersion string, latest LatestReleaseInfo) (*AvailableUpdate, bool) {
	if !IsReleaseVersion(currentVersion) {
		return nil, false
	}
	if strings.TrimSpace(latest.LatestVersion) == "" {
		return nil, false
	}
	cmp, ok := compareVersions(currentVersion, latest.LatestVersion)
	if !ok || cmp >= 0 {
		return nil, false
	}
	return &AvailableUpdate{
		CurrentVersion: currentVersion,
		LatestVersion:  latest.LatestVersion,
		ReleaseURL:     latest.ReleaseURL,
	}, true
}

// CheckLatestRelease fetches the latest release metadata from GitHub without
// performing the heavier asset validation used by self-update install flows.
func (c *Client) CheckLatestRelease(ctx context.Context) (LatestReleaseInfo, error) {
	checkedAt := time.Now().UTC()
	rel, err := c.fetchRelease(ctx, "")
	if err != nil {
		return LatestReleaseInfo{}, err
	}
	return LatestReleaseInfo{
		CheckedAt:     checkedAt,
		AttemptedAt:   checkedAt,
		LatestVersion: rel.TagName,
		ReleaseURL:    rel.HTMLURL,
	}, nil
}

// RefreshLatestReleaseCache performs a live latest-release check and merges the
// result into the existing cache entry. Failed attempts still advance
// AttemptedAt so callers can throttle future checks.
func RefreshLatestReleaseCache(ctx context.Context, client *Client, cached *LatestReleaseInfo) (LatestReleaseInfo, error) {
	next := LatestReleaseInfo{}
	if cached != nil {
		next = *cached
	}
	next.AttemptedAt = time.Now().UTC()

	latest, err := client.CheckLatestRelease(ctx)
	if err != nil {
		return next, err
	}
	return LatestReleaseInfo{
		CheckedAt:     latest.CheckedAt,
		AttemptedAt:   latest.AttemptedAt,
		LatestVersion: latest.LatestVersion,
		ReleaseURL:    latest.ReleaseURL,
	}, nil
}
