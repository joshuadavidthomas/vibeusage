package display

// SnapshotErrorJSON represents a failed fetch outcome.
type SnapshotErrorJSON struct {
	Error ErrorDetailJSON `json:"error"`
}

// ErrorDetailJSON is the nested error detail within SnapshotErrorJSON.
type ErrorDetailJSON struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
}

// SnapshotJSON represents a successful fetch outcome.
type SnapshotJSON struct {
	Provider string        `json:"provider"`
	Source   string        `json:"source"`
	Cached   bool          `json:"cached"`
	Identity *IdentityJSON `json:"identity,omitempty"`
	Periods  []PeriodJSON  `json:"periods"`
	Overage  *OverageJSON  `json:"overage,omitempty"`
}

// IdentityJSON represents provider identity information.
type IdentityJSON struct {
	Email        string `json:"email"`
	Organization string `json:"organization"`
	Plan         string `json:"plan"`
}

// PeriodJSON represents a single usage period.
type PeriodJSON struct {
	Name        string `json:"name"`
	Utilization int    `json:"utilization"`
	Remaining   int    `json:"remaining"`
	PeriodType  string `json:"period_type"`
	ResetsAt    string `json:"resets_at,omitempty"`
	Model       string `json:"model,omitempty"`
}

// OverageJSON represents overage usage information.
type OverageJSON struct {
	Used      float64 `json:"used"`
	Limit     float64 `json:"limit"`
	Remaining float64 `json:"remaining"`
	Currency  string  `json:"currency"`
}

// MultiProviderJSON is the top-level response for multi-provider fetches.
type MultiProviderJSON struct {
	Providers map[string]SnapshotJSON `json:"providers"`
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
