package kimicode

import (
	"strconv"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// UsageResponse represents the response from the Kimi usage API endpoint.
type UsageResponse struct {
	User   *User        `json:"user,omitempty"`
	Usage  *UsageDetail `json:"usage,omitempty"`
	Limits []Limit      `json:"limits,omitempty"`
}

// User represents the user information in the usage response.
type User struct {
	UserID     string      `json:"userId,omitempty"`
	Region     string      `json:"region,omitempty"`
	Membership *Membership `json:"membership,omitempty"`
	BusinessID string      `json:"businessId,omitempty"`
}

// Membership represents the user's subscription tier.
type Membership struct {
	Level string `json:"level,omitempty"`
}

// UsageDetail represents usage quota info with limit, remaining, and reset time.
type UsageDetail struct {
	Limit     string `json:"limit"`
	Remaining string `json:"remaining"`
	ResetTime string `json:"resetTime,omitempty"`
}

// Utilization returns the usage percentage, clamped to [0, 100].
// Computed as (limit - remaining) / limit * 100.
func (u *UsageDetail) Utilization() int {
	if u == nil {
		return 0
	}
	limit, err := strconv.Atoi(u.Limit)
	if err != nil || limit <= 0 {
		return 0
	}
	remaining, err := strconv.Atoi(u.Remaining)
	if err != nil {
		return 0
	}
	used := limit - remaining
	pct := int(float64(used) / float64(limit) * 100)
	return max(0, min(pct, 100))
}

// ResetTimeUTC parses the resetTime as a time.Time.
func (u *UsageDetail) ResetTimeUTC() *time.Time {
	if u == nil {
		return nil
	}
	return models.ParseRFC3339Ptr(u.ResetTime)
}

// Limit represents a per-window usage limit.
type Limit struct {
	Window Window       `json:"window"`
	Detail *UsageDetail `json:"detail,omitempty"`
}

// Window describes the time window for a limit.
type Window struct {
	Duration int    `json:"duration"`
	TimeUnit string `json:"timeUnit"`
}

// PeriodType maps a Kimi window to a vibeusage period type.
func (w Window) PeriodType() models.PeriodType {
	minutes := w.DurationMinutes()
	switch {
	case minutes <= 300: // 5 hours
		return models.PeriodSession
	case minutes <= 1440: // 24 hours
		return models.PeriodDaily
	case minutes <= 10080: // 7 days
		return models.PeriodWeekly
	default:
		return models.PeriodMonthly
	}
}

// DurationMinutes converts the window to total minutes.
func (w Window) DurationMinutes() int {
	switch w.TimeUnit {
	case "TIME_UNIT_MINUTE":
		return w.Duration
	case "TIME_UNIT_HOUR":
		return w.Duration * 60
	case "TIME_UNIT_DAY":
		return w.Duration * 60 * 24
	default:
		return w.Duration
	}
}

// DisplayName returns a human-readable name for the window.
func (w Window) DisplayName() string {
	minutes := w.DurationMinutes()
	switch {
	case minutes < 60:
		return strconv.Itoa(minutes) + "m"
	case minutes < 1440:
		h := minutes / 60
		return strconv.Itoa(h) + "h"
	case minutes < 10080:
		d := minutes / 1440
		return strconv.Itoa(d) + "d"
	default:
		d := minutes / 1440
		return strconv.Itoa(d) + "d"
	}
}

// DeviceCodeResponse represents the response from the Kimi device authorization endpoint.
type DeviceCodeResponse struct {
	UserCode                string `json:"user_code"`
	DeviceCode              string `json:"device_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval,omitempty"`
	ExpiresIn               int    `json:"expires_in,omitempty"`
}

// TokenResponse represents the response from the Kimi OAuth token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials for Kimi.
type OAuthCredentials struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresAt    float64 `json:"expires_at,omitempty"` // Unix timestamp (matches kimi-cli format)
}

// NeedsRefresh reports whether the credentials need refreshing.
// Kimi tokens expire in ~15 minutes. We refresh 2 minutes early.
func (c OAuthCredentials) NeedsRefresh() bool {
	if c.ExpiresAt == 0 {
		return false
	}
	return time.Now().UTC().Unix() >= int64(c.ExpiresAt)-120
}

// PlanName returns a human-readable plan name from the membership level.
func PlanName(level string) string {
	switch level {
	case "LEVEL_BASIC":
		return "Basic (Free)"
	case "LEVEL_PRO":
		return "Pro"
	case "LEVEL_PREMIUM":
		return "Premium"
	default:
		if level != "" {
			return level
		}
		return ""
	}
}
