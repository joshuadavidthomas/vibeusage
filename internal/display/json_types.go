package display

import "github.com/joshuadavidthomas/vibeusage/internal/models"

// SnapshotErrorJSON represents a failed fetch outcome.
type SnapshotErrorJSON struct {
	Error ErrorDetailJSON `json:"error"`
}

// ErrorDetailJSON is the nested error detail within SnapshotErrorJSON.
type ErrorDetailJSON struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
}

// multiProviderJSON is the top-level response for multi-provider fetches.
type multiProviderJSON struct {
	Providers map[string]models.UsageSnapshot `json:"providers"`
	Errors    map[string]string               `json:"errors"`
	FetchedAt string                          `json:"fetched_at"`
}

// StatusEntryJSON represents a single provider's status.
type StatusEntryJSON struct {
	Level       string `json:"level"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// AuthStatusEntryJSON represents a single provider's auth status.
type AuthStatusEntryJSON struct {
	Authenticated bool   `json:"authenticated"`
	Source        string `json:"source"`
}

// ConfigShowJSON represents the config show JSON output.
type ConfigShowJSON struct {
	Fetch   ConfigFetchJSON   `json:"fetch"`
	Display ConfigDisplayJSON `json:"display"`
	Credentials      ConfigCredentialsJSON `json:"credentials"`
	Roles            any                   `json:"roles"`
	Path             string                `json:"path"`
}

// ConfigFetchJSON represents the fetch section of config.
type ConfigFetchJSON struct {
	Timeout               float64 `json:"timeout"`
	StaleThresholdMinutes int     `json:"stale_threshold_minutes"`
	MaxConcurrent         int     `json:"max_concurrent"`
}

// ConfigDisplayJSON represents the display section of config.
type ConfigDisplayJSON struct {
	ShowRemaining bool   `json:"show_remaining"`
	PaceColors    bool   `json:"pace_colors"`
	ResetFormat   string `json:"reset_format"`
}

// ConfigCredentialsJSON represents the credentials section of config.
type ConfigCredentialsJSON struct {
	UseKeyring               bool `json:"use_keyring"`

}

// ActionResultJSON is a generic success/message response used by
// config reset, cache clear, and similar operations.
type ActionResultJSON struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Reset    bool   `json:"reset,omitempty"`
	Provider string `json:"provider,omitempty"`
}

// KeyStatusEntryJSON represents a single provider's key status.
type KeyStatusEntryJSON struct {
	Configured bool   `json:"configured"`
	Source     string `json:"source"`
}

// KeyDetailJSON represents a single provider's key detail.
type KeyDetailJSON struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Source     string `json:"source"`
	Path       string `json:"path"`
}

// UpdateStatusJSON represents update check/apply output.
type UpdateStatusJSON struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	TargetVersion   string `json:"target_version"`
	UpdateAvailable bool   `json:"update_available"`
	IsDowngrade     bool   `json:"is_downgrade"`
	Asset           string `json:"asset,omitempty"`
	Applied         bool   `json:"applied,omitempty"`
	Pending         bool   `json:"pending,omitempty"`
}
