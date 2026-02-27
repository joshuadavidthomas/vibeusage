package claude

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
)

// UsagePeriodResponse represents a single usage period from the Claude OAuth API.
type UsagePeriodResponse struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at,omitempty"`
}

// ExtraUsageResponse represents overage/extra usage info from the Claude OAuth API.
// MonthlyLimit is a pointer to distinguish null (no hard limit) from 0 (zero limit).
type ExtraUsageResponse struct {
	IsEnabled    bool     `json:"is_enabled"`
	UsedCredits  float64  `json:"used_credits"`
	MonthlyLimit *float64 `json:"monthly_limit"`
	Utilization  *float64 `json:"utilization"`
}

// OAuthUsageResponse represents the usage response returned by both the OAuth
// endpoint (/api/oauth/usage) and the web session endpoint
// (/api/organizations/{orgID}/usage).
type OAuthUsageResponse struct {
	FiveHour          *UsagePeriodResponse `json:"five_hour,omitempty"`
	SevenDay          *UsagePeriodResponse `json:"seven_day,omitempty"`
	Monthly           *UsagePeriodResponse `json:"monthly,omitempty"`
	SevenDaySonnet    *UsagePeriodResponse `json:"seven_day_sonnet,omitempty"`
	SevenDayOpus      *UsagePeriodResponse `json:"seven_day_opus,omitempty"`
	SevenDayHaiku     *UsagePeriodResponse `json:"seven_day_haiku,omitempty"`
	SevenDayOAuthApps *UsagePeriodResponse `json:"seven_day_oauth_apps,omitempty"`
	SevenDayCowork    *UsagePeriodResponse `json:"seven_day_cowork,omitempty"`
	IguanaNecktie     *UsagePeriodResponse `json:"iguana_necktie,omitempty"`
	ExtraUsage        *ExtraUsageResponse  `json:"extra_usage,omitempty"`
	Plan              string               `json:"plan,omitempty"`
	BillingType       string               `json:"billing_type,omitempty"`
}

// OAuthCredentials is an alias for the shared OAuth credential type.
type OAuthCredentials = oauth.Credentials

// ClaudeCLIOAuth represents the nested OAuth data inside Claude CLI credentials.
type ClaudeCLIOAuth struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	ExpiresAt    float64 `json:"expiresAt,omitempty"` // millisecond timestamp
}

// ToOAuthCredentials converts Claude CLI format to standard OAuthCredentials.
func (c *ClaudeCLIOAuth) ToOAuthCredentials() OAuthCredentials {
	creds := OAuthCredentials{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
	}
	if c.ExpiresAt > 0 {
		t := time.UnixMilli(int64(c.ExpiresAt))
		creds.ExpiresAt = t.UTC().Format(time.RFC3339)
	}
	return creds
}

// ClaudeCLICredentials represents the Claude CLI credentials file format.
type ClaudeCLICredentials struct {
	ClaudeAiOauth *ClaudeCLIOAuth `json:"claudeAiOauth,omitempty"`
}

// WebOverageResponse represents the response from /api/organizations/{orgID}/overage_spend_limit.
type WebOverageResponse struct {
	HasHardLimit bool    `json:"has_hard_limit"`
	CurrentSpend float64 `json:"current_spend"`
	HardLimit    float64 `json:"hard_limit"`
}

// ToOverageUsage converts the web overage response to a models.OverageUsage.
// Returns nil if there is no hard limit.
func (r *WebOverageResponse) ToOverageUsage() *models.OverageUsage {
	if !r.HasHardLimit {
		return nil
	}
	return &models.OverageUsage{
		Used:      r.CurrentSpend / 100.0,
		Limit:     r.HardLimit / 100.0,
		Currency:  "USD",
		IsEnabled: true,
	}
}

// WebOrganization represents a single organization from /api/organizations.
type WebOrganization struct {
	UUID         string   `json:"uuid,omitempty"`
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// OrgID returns the best identifier for this organization, preferring UUID over ID.
func (o *WebOrganization) OrgID() string {
	if o.UUID != "" {
		return o.UUID
	}
	return o.ID
}

// HasCapability reports whether the organization has the given capability.
func (o *WebOrganization) HasCapability(cap string) bool {
	for _, c := range o.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// WebSessionCredentials represents stored web session credentials.
type WebSessionCredentials struct {
	SessionKey string `json:"session_key"`
}
