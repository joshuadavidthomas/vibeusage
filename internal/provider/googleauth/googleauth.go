// Package googleauth provides shared Google OAuth types and helpers
// used by both the Gemini and Antigravity providers.
package googleauth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

const TokenURL = "https://oauth2.googleapis.com/token"

// TokenResponse represents the response from the Google OAuth token refresh endpoint.
type TokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// OAuthCredentials represents stored Google OAuth credentials.
type OAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// NeedsRefresh reports whether the credentials need refreshing.
// Returns true if the token expires within 5 minutes.
func (c OAuthCredentials) NeedsRefresh() bool {
	if c.ExpiresAt == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().Add(5 * time.Minute).After(expiry)
}

// RefreshConfig contains the provider-specific parameters for token refresh.
type RefreshConfig struct {
	ClientID     string
	ClientSecret string
	ProviderID   string // e.g. "gemini" or "antigravity"
}

// RefreshToken refreshes an expired Google OAuth token and saves the updated
// credentials to disk. Returns nil if the refresh fails.
func RefreshToken(ctx context.Context, creds *OAuthCredentials, cfg RefreshConfig) *OAuthCredentials {
	if creds.RefreshToken == "" {
		return nil
	}

	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	var tokenResp TokenResponse
	resp, err := client.PostFormCtx(ctx, TokenURL,
		map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": creds.RefreshToken,
			"client_id":     cfg.ClientID,
			"client_secret": cfg.ClientSecret,
		},
		&tokenResp,
	)
	if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
		return nil
	}

	updated := &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}

	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	// Preserve refresh token if the server didn't issue a new one
	if updated.RefreshToken == "" {
		updated.RefreshToken = creds.RefreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath(cfg.ProviderID, "oauth"), content)

	return updated
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
