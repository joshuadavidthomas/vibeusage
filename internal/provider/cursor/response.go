package cursor

import (
	"encoding/json"
	"strings"
	"time"
)

// UsageSummaryResponse represents the response from the Cursor usage summary endpoint.
// The API returns cents-based usage with individual and team breakdowns.
type UsageSummaryResponse struct {
	BillingCycleStart                  string           `json:"billingCycleStart,omitempty"`
	BillingCycleEnd                    string           `json:"billingCycleEnd,omitempty"`
	MembershipType                     string           `json:"membershipType,omitempty"`
	LimitType                          string           `json:"limitType,omitempty"`
	IsUnlimited                        *bool            `json:"isUnlimited,omitempty"`
	AutoModelSelectedDisplayMessage    string           `json:"autoModelSelectedDisplayMessage,omitempty"`
	NamedModelSelectedDisplayMessage   string           `json:"namedModelSelectedDisplayMessage,omitempty"`
	IndividualUsage                    *IndividualUsage `json:"individualUsage,omitempty"`
	TeamUsage                          *TeamUsage       `json:"teamUsage,omitempty"`
}

// BillingCycleEndTime parses the billing cycle end as a time.
// Handles both ISO 8601 strings and Unix millisecond timestamps (as strings).
func (r *UsageSummaryResponse) BillingCycleEndTime() *time.Time {
	return parseFlexibleTime(r.BillingCycleEnd)
}

// BillingCycleStartTime parses the billing cycle start as a time.
func (r *UsageSummaryResponse) BillingCycleStartTime() *time.Time {
	return parseFlexibleTime(r.BillingCycleStart)
}

// parseFlexibleTime parses a time string that may be ISO 8601 or Unix ms (as string or number).
func parseFlexibleTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}

	// Try ISO 8601 with fractional seconds
	if t, err := time.Parse("2006-01-02T15:04:05.999Z", raw); err == nil {
		return &t
	}
	// Try ISO 8601 without fractional seconds
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t
	}

	// Try as Unix millisecond string (Connect RPC format)
	var ms float64
	if json.Unmarshal([]byte(raw), &ms) == nil && ms > 0 {
		t := time.UnixMilli(int64(ms)).UTC()
		return &t
	}

	return nil
}

// IndividualUsage represents per-user usage data.
type IndividualUsage struct {
	Plan     *PlanUsage     `json:"plan,omitempty"`
	OnDemand *OnDemandUsage `json:"onDemand,omitempty"`
}

// PlanUsage represents included plan usage (amounts in cents).
type PlanUsage struct {
	Enabled         *bool          `json:"enabled,omitempty"`
	Used            float64        `json:"used"`
	Limit           float64        `json:"limit"`
	Remaining       float64        `json:"remaining"`
	Breakdown       *PlanBreakdown `json:"breakdown,omitempty"`
	AutoPercentUsed float64        `json:"autoPercentUsed"`
	APIPercentUsed  float64        `json:"apiPercentUsed"`
	TotalPercentUsed float64       `json:"totalPercentUsed"`
}

// PlanBreakdown breaks down plan usage into included and bonus credits (cents).
type PlanBreakdown struct {
	Included float64 `json:"included"`
	Bonus    float64 `json:"bonus"`
	Total    float64 `json:"total"`
}

// OnDemandUsage represents on-demand spending (amounts in cents).
// Limit and Remaining are nullable (nil when on-demand is disabled or unlimited).
type OnDemandUsage struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	Used      float64  `json:"used"`
	Limit     *float64 `json:"limit"`
	Remaining *float64 `json:"remaining"`
}

// TeamUsage represents team-level usage data.
type TeamUsage struct {
	OnDemand *OnDemandUsage `json:"onDemand,omitempty"`
}

// UserMeResponse represents the response from the Cursor auth/me endpoint.
type UserMeResponse struct {
	Email         string `json:"email,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	Sub           string `json:"sub,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	Picture       string `json:"picture,omitempty"`

	// Legacy field: some responses include membership_type here.
	MembershipType string `json:"membership_type,omitempty"`
}

// SessionCredentials represents stored session credentials for Cursor.
// Multiple key names are supported for backward compatibility.
type SessionCredentials struct {
	SessionToken string `json:"session_token,omitempty"`
	Token        string `json:"token,omitempty"`
	SessionKey   string `json:"session_key,omitempty"`
	Session      string `json:"session,omitempty"`
}

// EffectiveToken returns the first non-empty session token found.
func (c *SessionCredentials) EffectiveToken() string {
	for _, v := range []string{c.SessionToken, c.Token, c.SessionKey, c.Session} {
		if v != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
