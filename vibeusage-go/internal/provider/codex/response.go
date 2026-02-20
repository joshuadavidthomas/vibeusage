package codex

import "time"

// UsageResponse represents the response from the Codex/ChatGPT usage endpoint.
// The API uses alternate key names: "rate_limit" vs "rate_limits".
type UsageResponse struct {
	RateLimit  *RateLimits `json:"rate_limit,omitempty"`
	RateLimits *RateLimits `json:"rate_limits,omitempty"`
	Credits    *Credits    `json:"credits,omitempty"`
	PlanType   string      `json:"plan_type,omitempty"`
}

// EffectiveRateLimits returns whichever rate limits field is populated.
func (r *UsageResponse) EffectiveRateLimits() *RateLimits {
	if r.RateLimit != nil {
		return r.RateLimit
	}
	return r.RateLimits
}

// RateLimits contains primary and secondary rate windows.
// The API uses alternate key names: "primary_window" vs "primary",
// "secondary_window" vs "secondary".
type RateLimits struct {
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
	UsedPercent    float64 `json:"used_percent"`
	ResetAt        float64 `json:"reset_at,omitempty"`
	ResetTimestamp float64 `json:"reset_timestamp,omitempty"`
}

// EffectiveResetTimestamp returns whichever reset timestamp field is populated.
func (w *RateWindow) EffectiveResetTimestamp() float64 {
	if w.ResetAt != 0 {
		return w.ResetAt
	}
	return w.ResetTimestamp
}

// Credits represents available credits on the account.
type Credits struct {
	HasCredits bool    `json:"has_credits"`
	Balance    float64 `json:"balance"`
}

// TokenResponse represents the response from the OAuth token refresh endpoint.
type TokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// Credentials represents stored OAuth credentials for Codex.
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// NeedsRefresh reports whether the credentials need refreshing.
// Returns true if expires_at is in the past or within 8 days, or if it's unparseable.
func (c Credentials) NeedsRefresh() bool {
	if c.ExpiresAt == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	threshold := time.Now().UTC().AddDate(0, 0, 8)
	return threshold.After(expiry)
}

// CLICredentials represents the Codex CLI credential file format,
// which may nest tokens under a "tokens" key or be flat.
type CLICredentials struct {
	Tokens       *Credentials `json:"tokens,omitempty"`
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresAt    string       `json:"expires_at,omitempty"`
}

// EffectiveCredentials returns the credentials from whichever format is present.
// Returns nil if no access token is found.
func (c *CLICredentials) EffectiveCredentials() *Credentials {
	if c.Tokens != nil && c.Tokens.AccessToken != "" {
		return c.Tokens
	}
	if c.AccessToken != "" {
		return &Credentials{
			AccessToken:  c.AccessToken,
			RefreshToken: c.RefreshToken,
			ExpiresAt:    c.ExpiresAt,
		}
	}
	return nil
}
