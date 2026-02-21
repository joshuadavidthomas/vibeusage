package copilot

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

// DeviceCodeResponse represents the response from the GitHub device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval,omitempty"`
}

// TokenResponse represents the response from the GitHub OAuth token endpoint.
// It can contain either a successful token or an error.
type TokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials for Copilot.
type OAuthCredentials struct {
	AccessToken string `json:"access_token"`
}
