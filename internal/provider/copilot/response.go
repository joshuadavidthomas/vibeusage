package copilot

import "github.com/joshuadavidthomas/vibeusage/internal/oauth"

// UserResponse represents the response from the Copilot user API endpoint.
type UserResponse struct {
	QuotaResetDateUTC string          `json:"quota_reset_date_utc,omitempty"`
	CopilotPlan       string          `json:"copilot_plan,omitempty"`
	QuotaSnapshots    *QuotaSnapshots `json:"quota_snapshots,omitempty"`
}

// QuotaSnapshots contains quota information for different interaction types.
type QuotaSnapshots struct {
	PremiumInteractions *Quota `json:"premium_interactions,omitempty"`
	Chat                *Quota `json:"chat,omitempty"`
	Completions         *Quota `json:"completions,omitempty"`
}

// Quota represents a single quota type with entitlement, remaining, and unlimited status.
type Quota struct {
	Entitlement float64 `json:"entitlement"`
	Remaining   float64 `json:"remaining"`
	Unlimited   bool    `json:"unlimited"`
}

// Utilization returns the percentage of quota used, clamped to [0, 100].
func (q *Quota) Utilization() int {
	if q.Unlimited && q.Entitlement == 0 {
		return 0
	}
	if q.Entitlement > 0 {
		used := q.Entitlement - q.Remaining
		pct := int((used / q.Entitlement) * 100)
		return max(0, min(pct, 100))
	}
	return 0
}

// HasUsage reports whether this quota should be displayed (has entitlement or is unlimited).
func (q *Quota) HasUsage() bool {
	return q.Unlimited || q.Entitlement > 0
}

// OAuthCredentials is an alias for the shared OAuth credential type.
// Legacy credentials with only access_token (no refresh_token/expires_at)
// are a valid subset and load transparently.
type OAuthCredentials = oauth.Credentials
