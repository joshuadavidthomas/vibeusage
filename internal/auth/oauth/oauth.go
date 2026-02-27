// Package oauth provides shared OAuth credential types, expiration checking,
// and token refresh logic used by providers with OAuth-based authentication.
package oauth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

// RefreshBuffer is the duration before token expiry at which a refresh is
// triggered. All providers share this value so refresh behavior is consistent.
const RefreshBuffer = 5 * time.Minute

// Credentials represents stored OAuth credentials in the canonical vibeusage
// format. All providers normalize to this type for storage and refresh.
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"` // RFC3339
}

// NeedsRefresh reports whether the credentials should be refreshed.
// Returns true if ExpiresAt is within RefreshBuffer of now, already past,
// or unparseable. Returns false if ExpiresAt is empty (non-expiring token).
func (c Credentials) NeedsRefresh() bool {
	if c.ExpiresAt == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().Add(RefreshBuffer).After(expiry)
}

// TokenResponse represents the response from an OAuth token refresh endpoint.
type TokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// RefreshConfig contains the provider-specific parameters for token refresh.
type RefreshConfig struct {
	// TokenURL is the OAuth token endpoint (e.g. "https://api.anthropic.com/oauth/token").
	TokenURL string
	// FormFields are additional form parameters beyond grant_type and refresh_token
	// (e.g. client_id, client_secret).
	FormFields map[string]string
	// Headers are provider-specific request options (e.g. anthropic-beta header).
	Headers []httpclient.RequestOption
	// ProviderID is used to determine the credential save path.
	ProviderID string
	// HTTPTimeout is the request timeout in seconds.
	HTTPTimeout float64
}

// Refresh exchanges a refresh token for a new access token and saves the
// updated credentials to disk. Returns nil if the refresh fails for any reason.
func Refresh(ctx context.Context, refreshToken string, cfg RefreshConfig) *Credentials {
	if refreshToken == "" {
		return nil
	}

	form := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}
	for k, v := range cfg.FormFields {
		form[k] = v
	}

	client := httpclient.NewFromConfig(cfg.HTTPTimeout)
	var tokenResp TokenResponse
	resp, err := client.PostFormCtx(ctx, cfg.TokenURL, form, &tokenResp, cfg.Headers...)
	if err != nil || resp.StatusCode != 200 || resp.JSONErr != nil {
		return nil
	}

	if tokenResp.AccessToken == "" {
		return nil
	}

	updated := &Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}

	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	// Preserve the old refresh token if the server didn't issue a new one
	if updated.RefreshToken == "" {
		updated.RefreshToken = refreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath(cfg.ProviderID, "oauth"), content)

	return updated
}
