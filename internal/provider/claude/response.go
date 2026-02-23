package claude

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// UsagePeriodResponse represents a single usage period from the Claude OAuth API.
type UsagePeriodResponse struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at,omitempty"`
}

// ExtraUsageResponse represents overage/extra usage info from the Claude OAuth API.
type ExtraUsageResponse struct {
	IsEnabled    bool    `json:"is_enabled"`
	UsedCredits  float64 `json:"used_credits"`
	MonthlyLimit float64 `json:"monthly_limit"`
}

// OAuthUsageResponse represents the full response from /api/oauth/usage.
type OAuthUsageResponse struct {
	FiveHour       *UsagePeriodResponse `json:"five_hour,omitempty"`
	SevenDay       *UsagePeriodResponse `json:"seven_day,omitempty"`
	Monthly        *UsagePeriodResponse `json:"monthly,omitempty"`
	SevenDaySonnet *UsagePeriodResponse `json:"seven_day_sonnet,omitempty"`
	SevenDayOpus   *UsagePeriodResponse `json:"seven_day_opus,omitempty"`
	SevenDayHaiku  *UsagePeriodResponse `json:"seven_day_haiku,omitempty"`
	ExtraUsage     *ExtraUsageResponse  `json:"extra_usage,omitempty"`
	Plan           string               `json:"plan,omitempty"`
	BillingType    string               `json:"billing_type,omitempty"`
}

// OAuthTokenResponse represents the response from /oauth/token.
type OAuthTokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	TokenType    string  `json:"token_type,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials (vibeusage format).
type OAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

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

// NeedsRefresh reports whether the credentials have expired or have an unparseable expiry.
func (c OAuthCredentials) NeedsRefresh() bool {
	if c.ExpiresAt == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(expiry)
}

// ClaudeCLICredentials represents the Claude CLI credentials file format.
type ClaudeCLICredentials struct {
	ClaudeAiOauth *ClaudeCLIOAuth `json:"claudeAiOauth,omitempty"`
}

// WebUsageResponse represents the response from /api/organizations/{orgID}/usage.
type WebUsageResponse struct {
	UsageAmount  float64 `json:"usage_amount"`
	UsageLimit   float64 `json:"usage_limit"`
	PeriodEnd    string  `json:"period_end,omitempty"`
	Email        string  `json:"email,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Plan         string  `json:"plan,omitempty"`
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
