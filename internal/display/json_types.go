package display

import (
	"encoding/json"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// SnapshotErrorJSON represents a failed fetch outcome.
type SnapshotErrorJSON struct {
	Error ErrorDetailJSON `json:"error"`
}

// ErrorDetailJSON is the nested error detail within SnapshotErrorJSON.
type ErrorDetailJSON struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
}

// snapshotJSON is a JSON wrapper around snapshot data that adds
// fetch metadata (source, cached).
type snapshotJSON struct {
	Provider string                   `json:"provider"`
	Source   string                   `json:"source"`
	Cached   bool                     `json:"cached"`
	Identity *models.ProviderIdentity `json:"identity,omitempty"`
	Periods  []models.UsagePeriod     `json:"periods"`
	Overage  *models.OverageUsage     `json:"overage,omitempty"`
}

func (s snapshotJSON) MarshalJSON() ([]byte, error) {
	type periodJSON struct {
		Name        string `json:"name"`
		Utilization int    `json:"utilization"`
		Remaining   int    `json:"remaining"`
		PeriodType  string `json:"period_type"`
		ResetsAt    string `json:"resets_at,omitempty"`
		Model       string `json:"model,omitempty"`
	}

	periods := make([]periodJSON, len(s.Periods))
	for i, p := range s.Periods {
		periods[i] = periodJSON{
			Name:        p.Name,
			Utilization: p.Utilization,
			Remaining:   p.Remaining(),
			PeriodType:  string(p.PeriodType),
			Model:       p.Model,
		}
		if p.ResetsAt != nil {
			periods[i].ResetsAt = p.ResetsAt.Format(time.RFC3339)
		}
	}

	type overageJSON struct {
		Used      float64 `json:"used"`
		Limit     float64 `json:"limit"`
		Remaining float64 `json:"remaining"`
		Currency  string  `json:"currency"`
	}

	var overage *overageJSON
	if s.Overage != nil {
		overage = &overageJSON{
			Used:      s.Overage.Used,
			Limit:     s.Overage.Limit,
			Remaining: s.Overage.Remaining(),
			Currency:  s.Overage.Currency,
		}
	}

	return json.Marshal(struct {
		Provider string                   `json:"provider"`
		Source   string                   `json:"source"`
		Cached   bool                     `json:"cached"`
		Identity *models.ProviderIdentity `json:"identity,omitempty"`
		Periods  []periodJSON             `json:"periods"`
		Overage  *overageJSON             `json:"overage,omitempty"`
	}{
		Provider: s.Provider,
		Source:   s.Source,
		Cached:   s.Cached,
		Identity: s.Identity,
		Periods:  periods,
		Overage:  overage,
	})
}

// multiProviderJSON is the top-level response for multi-provider fetches.
type multiProviderJSON struct {
	Providers map[string]snapshotJSON `json:"providers"`
	Errors    map[string]string       `json:"errors"`
	FetchedAt string                  `json:"fetched_at"`
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
	Fetch            ConfigFetchJSON       `json:"fetch"`
	EnabledProviders []string              `json:"enabled_providers"`
	Display          ConfigDisplayJSON     `json:"display"`
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
	ReuseProviderCredentials bool `json:"reuse_provider_credentials"`
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

// InitStatusJSON represents the init command JSON output.
type InitStatusJSON struct {
	FirstRun            bool     `json:"first_run"`
	ConfiguredProviders int      `json:"configured_providers"`
	AvailableProviders  []string `json:"available_providers"`
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
