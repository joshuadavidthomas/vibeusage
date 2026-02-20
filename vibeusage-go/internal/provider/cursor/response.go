package cursor

import (
	"encoding/json"
	"time"
)

// UsageSummaryResponse represents the response from the Cursor usage summary endpoint.
type UsageSummaryResponse struct {
	PremiumRequests *PremiumRequests `json:"premium_requests,omitempty"`
	BillingCycle    *BillingCycle    `json:"billing_cycle,omitempty"`
	OnDemandSpend   *OnDemandSpend   `json:"on_demand_spend,omitempty"`
}

// PremiumRequests represents premium request usage.
type PremiumRequests struct {
	Used      float64 `json:"used"`
	Available float64 `json:"available"`
}

// BillingCycle represents billing cycle dates.
// The "end" field can be either an RFC3339 string or a Unix millisecond timestamp.
type BillingCycle struct {
	EndRaw json.RawMessage `json:"end,omitempty"`
}

// EndTime parses the "end" field as a time, handling both string and numeric formats.
func (bc *BillingCycle) EndTime() *time.Time {
	if bc == nil || len(bc.EndRaw) == 0 {
		return nil
	}

	// Try string first
	var s string
	if json.Unmarshal(bc.EndRaw, &s) == nil && s != "" {
		// Normalize Z suffix
		parsed := s
		if len(parsed) > 0 && parsed[len(parsed)-1] == 'Z' {
			parsed = parsed[:len(parsed)-1] + "+00:00"
		}
		if t, err := time.Parse(time.RFC3339, parsed); err == nil {
			return &t
		}
	}

	// Try numeric (milliseconds)
	var f float64
	if json.Unmarshal(bc.EndRaw, &f) == nil && f > 0 {
		t := time.UnixMilli(int64(f)).UTC()
		return &t
	}

	return nil
}

// OnDemandSpend represents on-demand spending limits and usage.
type OnDemandSpend struct {
	LimitCents float64 `json:"limit_cents"`
	UsedCents  float64 `json:"used_cents"`
}

// UserMeResponse represents the response from the Cursor user/me endpoint.
type UserMeResponse struct {
	Email          string `json:"email,omitempty"`
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
			return v
		}
	}
	return ""
}
