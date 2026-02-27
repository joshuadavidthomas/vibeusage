package copilot

// UserResponse represents the response from the Copilot user API endpoint.
type UserResponse struct {
	Login                 string          `json:"login,omitempty"`
	AccessTypeSku         string          `json:"access_type_sku,omitempty"`
	AnalyticsTrackingID   string          `json:"analytics_tracking_id,omitempty"`
	AssignedDate          string          `json:"assigned_date,omitempty"`
	CanSignupForLimited   bool            `json:"can_signup_for_limited,omitempty"`
	ChatEnabled           bool            `json:"chat_enabled,omitempty"`
	CopilotignoreEnabled  bool            `json:"copilotignore_enabled,omitempty"`
	CopilotPlan           string          `json:"copilot_plan,omitempty"`
	IsMcpEnabled          bool            `json:"is_mcp_enabled,omitempty"`
	OrganizationLoginList []string        `json:"organization_login_list,omitempty"`
	OrganizationList      []string        `json:"organization_list,omitempty"`
	RestrictedTelemetry   bool            `json:"restricted_telemetry,omitempty"`
	Endpoints             *Endpoints      `json:"endpoints,omitempty"`
	QuotaResetDate        string          `json:"quota_reset_date,omitempty"`
	QuotaSnapshots        *QuotaSnapshots `json:"quota_snapshots,omitempty"`
	QuotaResetDateUTC     string          `json:"quota_reset_date_utc,omitempty"`
}

// Endpoints contains the API endpoint URLs for Copilot services.
type Endpoints struct {
	API           string `json:"api,omitempty"`
	OriginTracker string `json:"origin-tracker,omitempty"`
	Proxy         string `json:"proxy,omitempty"`
	Telemetry     string `json:"telemetry,omitempty"`
}

// QuotaSnapshots contains quota information for different interaction types.
type QuotaSnapshots struct {
	PremiumInteractions *Quota `json:"premium_interactions,omitempty"`
	Chat                *Quota `json:"chat,omitempty"`
	Completions         *Quota `json:"completions,omitempty"`
}

// Quota represents a single quota type with entitlement, remaining, and unlimited status.
type Quota struct {
	Entitlement      float64 `json:"entitlement"`
	OverageCount     int     `json:"overage_count,omitempty"`
	OveragePermitted bool    `json:"overage_permitted,omitempty"`
	PercentRemaining float64 `json:"percent_remaining,omitempty"`
	QuotaID          string  `json:"quota_id,omitempty"`
	QuotaRemaining   float64 `json:"quota_remaining,omitempty"`
	Remaining        float64 `json:"remaining"`
	Unlimited        bool    `json:"unlimited"`
	TimestampUTC     string  `json:"timestamp_utc,omitempty"`
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

// Legacy credentials with only access_token (no refresh_token/expires_at)
// are a valid subset and load transparently.
