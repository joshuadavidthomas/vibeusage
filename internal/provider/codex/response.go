package codex

import (
	"encoding/json"
	"strconv"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
)

// UsageResponse represents the response from the Codex/ChatGPT usage endpoint.
// The API uses alternate key names: "rate_limit" vs "rate_limits".
type UsageResponse struct {
	UserID               string                `json:"user_id,omitempty"`
	AccountID            string                `json:"account_id,omitempty"`
	Email                string                `json:"email,omitempty"`
	PlanType             string                `json:"plan_type,omitempty"`
	RateLimit            *RateLimits           `json:"rate_limit,omitempty"`
	RateLimits           *RateLimits           `json:"rate_limits,omitempty"`
	CodeReviewRateLimit  *RateLimits           `json:"code_review_rate_limit,omitempty"`
	AdditionalRateLimits []AdditionalRateLimit `json:"additional_rate_limits,omitempty"`
	Credits              *Credits              `json:"credits,omitempty"`
	Promo                json.RawMessage       `json:"promo,omitempty"`
}

// AdditionalRateLimit represents a named, model-specific rate limit entry.
type AdditionalRateLimit struct {
	LimitName      string      `json:"limit_name,omitempty"`
	MeteredFeature string      `json:"metered_feature,omitempty"`
	RateLimit      *RateLimits `json:"rate_limit,omitempty"`
}

// EffectiveRateLimits returns whichever rate limits field is populated.
func (r *UsageResponse) EffectiveRateLimits() *RateLimits {
	if r.RateLimit != nil {
		return r.RateLimit
	}
	return r.RateLimits
}

// RateLimits contains primary and secondary rate windows plus limit status flags.
// The API uses alternate key names: "primary_window" vs "primary",
// "secondary_window" vs "secondary".
type RateLimits struct {
	Allowed         bool        `json:"allowed,omitempty"`
	LimitReached    bool        `json:"limit_reached,omitempty"`
	PrimaryWindow   *RateWindow `json:"primary_window,omitempty"`
	Primary         *RateWindow `json:"primary,omitempty"`
	SecondaryWindow *RateWindow `json:"secondary_window,omitempty"`
	Secondary       *RateWindow `json:"secondary,omitempty"`
}

// EffectivePrimary returns whichever primary window field is populated.
func (r *RateLimits) EffectivePrimary() *RateWindow {
	if r.PrimaryWindow != nil {
		return r.PrimaryWindow
	}
	return r.Primary
}

// EffectiveSecondary returns whichever secondary window field is populated.
func (r *RateLimits) EffectiveSecondary() *RateWindow {
	if r.SecondaryWindow != nil {
		return r.SecondaryWindow
	}
	return r.Secondary
}

// RateWindow represents a single rate limit window with usage percentage and reset time.
// The API uses alternate key names: "reset_at" vs "reset_timestamp".
type RateWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds float64 `json:"limit_window_seconds,omitempty"`
	ResetAfterSeconds  float64 `json:"reset_after_seconds,omitempty"`
	ResetAt            float64 `json:"reset_at,omitempty"`
	ResetTimestamp     float64 `json:"reset_timestamp,omitempty"`
}

// EffectiveResetTimestamp returns whichever reset timestamp field is populated.
func (w *RateWindow) EffectiveResetTimestamp() float64 {
	if w.ResetAt != 0 {
		return w.ResetAt
	}
	return w.ResetTimestamp
}

// Credits represents available credits on the account.
// Balance can be a number or string in the API response.
type Credits struct {
	HasCredits          bool            `json:"has_credits"`
	Unlimited           bool            `json:"unlimited,omitempty"`
	RawBalance          json.RawMessage `json:"balance"`
	ApproxLocalMessages json.RawMessage `json:"approx_local_messages,omitempty"`
	ApproxCloudMessages json.RawMessage `json:"approx_cloud_messages,omitempty"`
}

// Balance parses the balance field, handling both number and string representations.
func (c *Credits) Balance() float64 {
	if c.RawBalance == nil {
		return 0
	}
	var f float64
	if err := json.Unmarshal(c.RawBalance, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(c.RawBalance, &s); err == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return 0
}

// Credentials is an alias for the shared OAuth credential type.

// CLICredentials represents the Codex CLI credential file format,
// which may nest tokens under a "tokens" key or be flat.
type CLICredentials struct {
	Tokens       *oauth.Credentials `json:"tokens,omitempty"`
	AccessToken  string             `json:"access_token,omitempty"`
	RefreshToken string             `json:"refresh_token,omitempty"`
	ExpiresAt    string             `json:"expires_at,omitempty"`
}

// EffectiveCredentials returns the credentials from whichever format is present.
// Returns nil if no access token is found.
func (c *CLICredentials) EffectiveCredentials() *oauth.Credentials {
	if c.Tokens != nil && c.Tokens.AccessToken != "" {
		return c.Tokens
	}
	if c.AccessToken != "" {
		return &oauth.Credentials{
			AccessToken:  c.AccessToken,
			RefreshToken: c.RefreshToken,
			ExpiresAt:    c.ExpiresAt,
		}
	}
	return nil
}
