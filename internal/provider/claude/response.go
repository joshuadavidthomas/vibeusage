package claude

import (
	"encoding/json"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
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
	Currency     string   `json:"currency,omitempty"`
}

// OAuthUsageResponse represents the usage response returned by both the OAuth
// endpoint (/api/oauth/usage) and the web session endpoint
// (/api/organizations/{orgID}/usage).
type OAuthUsageResponse struct {
	FiveHour            *UsagePeriodResponse `json:"five_hour,omitempty"`
	SevenDay            *UsagePeriodResponse `json:"seven_day,omitempty"`
	SevenDaySonnet      *UsagePeriodResponse `json:"seven_day_sonnet,omitempty"`
	SevenDayOpus        *UsagePeriodResponse `json:"seven_day_opus,omitempty"`
	SevenDayOAuthApps   *UsagePeriodResponse `json:"seven_day_oauth_apps,omitempty"`
	SevenDayCowork      *UsagePeriodResponse `json:"seven_day_cowork,omitempty"`
	SevenDayOmelette    *UsagePeriodResponse `json:"seven_day_omelette,omitempty"`
	OmelettePromotional *UsagePeriodResponse `json:"omelette_promotional,omitempty"`
	IguanaNecktie       *UsagePeriodResponse `json:"iguana_necktie,omitempty"`
	ExtraUsage          *ExtraUsageResponse  `json:"extra_usage,omitempty"`
}

// ClaudeCLIOAuth represents the nested OAuth data inside Claude CLI credentials.
type ClaudeCLIOAuth struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	ExpiresAt    float64 `json:"expiresAt,omitempty"` // millisecond timestamp
}

// ToOAuthCredentials converts Claude CLI format to standard oauth.Credentials.
func (c *ClaudeCLIOAuth) ToOAuthCredentials() oauth.Credentials {
	creds := oauth.Credentials{
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

// OAuthAccountResponse represents the response from /api/oauth/account.
type OAuthAccountResponse struct {
	EmailAddress string                   `json:"email_address"`
	Memberships  []OAuthAccountMembership `json:"memberships"`
}

// OAuthAccountMembership represents a single membership in the account response.
type OAuthAccountMembership struct {
	Organization OAuthAccountOrganization `json:"organization"`
}

// OAuthAccountOrganization represents the organization data within an account membership.
type OAuthAccountOrganization struct {
	Name          string   `json:"name,omitempty"`
	RateLimitTier string   `json:"rate_limit_tier,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	BillingType   string   `json:"billing_type,omitempty"`
}

// HasCapability reports whether the organization has the given capability.
func (o *OAuthAccountOrganization) HasCapability(cap string) bool {
	for _, c := range o.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// WebPrepaidCreditsResponse represents the response from
// /api/organizations/{orgID}/prepaid/credits.
type WebPrepaidCreditsResponse struct {
	Amount                    int             `json:"amount"`               // cents, can be negative
	Currency                  string          `json:"currency"`             // "USD"
	AutoReloadSettings        json.RawMessage `json:"auto_reload_settings"` // null = off
	PendingInvoiceAmountCents *int            `json:"pending_invoice_amount_cents,omitempty"`
}

// RoutinesBudgetResponse represents the response from
// https://claude.ai/v1/code/routines/run-budget. The limit and used fields
// are returned as strings by the API.
type RoutinesBudgetResponse struct {
	Limit                 json.Number `json:"limit"`
	Used                  json.Number `json:"used"`
	UnifiedBillingEnabled bool        `json:"unified_billing_enabled"`
}

// IsAutoReloadEnabled reports whether auto-reload is configured.
// A null or absent auto_reload_settings means auto-reload is off.
func (r *WebPrepaidCreditsResponse) IsAutoReloadEnabled() bool {
	return len(r.AutoReloadSettings) > 0 && string(r.AutoReloadSettings) != "null"
}

// ToBillingDetail converts the prepaid credits response to a models.BillingDetail.
func (r *WebPrepaidCreditsResponse) ToBillingDetail() *models.BillingDetail {
	balance := float64(r.Amount) / 100.0
	autoReload := r.IsAutoReloadEnabled()
	return &models.BillingDetail{
		Balance:    &balance,
		AutoReload: &autoReload,
	}
}
