// Package googleauth provides shared Google OAuth types and helpers
// used by both the Gemini and Antigravity providers.
package google

import (
	"context"
	"encoding/json"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
)

const TokenURL = "https://oauth2.googleapis.com/token"

// TokenResponse represents the response from the Google OAuth token refresh endpoint.
type TokenResponse = oauth.TokenResponse

// RefreshConfig contains the provider-specific parameters for token refresh.
type RefreshConfig struct {
	ClientID     string
	ClientSecret string
	ProviderID   string // e.g. "gemini" or "antigravity"
	HTTPTimeout  float64
}

// RefreshToken refreshes an expired Google OAuth token and saves the updated
// credentials to disk. Returns nil if the refresh fails.
func RefreshToken(ctx context.Context, creds *oauth.Credentials, cfg RefreshConfig) *oauth.Credentials {
	return oauth.Refresh(ctx, creds.RefreshToken, oauth.RefreshConfig{
		TokenURL: TokenURL,
		FormFields: map[string]string{
			"client_id":     cfg.ClientID,
			"client_secret": cfg.ClientSecret,
		},
		ProviderID:  cfg.ProviderID,
		HTTPTimeout: cfg.HTTPTimeout,
	})
}

// ExtractAPIError returns a human-readable error description from a Google API
// error response body. Google APIs return errors as:
//
//	{"error":{"code":401,"message":"...","status":"UNAUTHENTICATED"}}
//
// Returns the message field if present, otherwise returns a truncated snippet
// of the raw body for debugging.
func ExtractAPIError(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		return envelope.Error.Message
	}
	// Fall back to a raw snippet for non-standard error shapes.
	s := string(body)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	if s == "" {
		return "empty response"
	}
	return s
}

// ParseExpiryDate converts a mixed-type expiry_date (float64 ms or string)
// to an RFC3339 string.
func ParseExpiryDate(v any) string {
	switch val := v.(type) {
	case float64:
		if val > 0 {
			return time.UnixMilli(int64(val)).UTC().Format(time.RFC3339)
		}
	case string:
		return val
	}
	return ""
}
