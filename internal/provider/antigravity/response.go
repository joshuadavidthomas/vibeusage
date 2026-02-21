package antigravity

import "time"

// QuotaRequest represents the request body for the Antigravity quota endpoint.
// Unlike Gemini which sends an empty body, Antigravity requires IDE metadata.
type QuotaRequest struct {
	Metadata QuotaRequestMetadata `json:"metadata"`
}

// QuotaRequestMetadata identifies the requesting IDE to the quota API.
type QuotaRequestMetadata struct {
	IDEType    string `json:"ideType"`
	Platform   string `json:"platform"`
	PluginType string `json:"pluginType"`
}

// QuotaResponse represents the response from the quota endpoint.
type QuotaResponse struct {
	QuotaBuckets []QuotaBucket `json:"quota_buckets,omitempty"`
}

// QuotaBucket represents a single quota bucket for a model.
type QuotaBucket struct {
	ModelID           string   `json:"model_id,omitempty"`
	RemainingFraction *float64 `json:"remaining_fraction,omitempty"`
	ResetTime         string   `json:"reset_time,omitempty"`
}

// Utilization returns the usage percentage, clamped to [0, 100].
// If remaining_fraction is absent, assumes full quota remaining (0% used).
func (b *QuotaBucket) Utilization() int {
	rf := 1.0
	if b.RemainingFraction != nil {
		rf = *b.RemainingFraction
	}
	pct := int((1 - rf) * 100)
	return max(0, min(pct, 100))
}

// ResetTimeUTC parses the reset_time as a time.Time.
func (b *QuotaBucket) ResetTimeUTC() *time.Time {
	if b.ResetTime == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, b.ResetTime); err == nil {
		return &t
	}
	return nil
}

// CodeAssistResponse represents the response from the code assist endpoint.
type CodeAssistResponse struct {
	UserTier string `json:"user_tier,omitempty"`
}

// TokenResponse represents the response from the Google OAuth token refresh endpoint.
type TokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials for Antigravity.
type OAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// NeedsRefresh reports whether the credentials need refreshing.
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

// AntigravityCredentials represents the credential file format stored by
// the Antigravity IDE. The exact format needs verification, but it likely
// follows the Google OAuth pattern used by Gemini CLI.
type AntigravityCredentials struct {
	// Flat format
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiryDate   any    `json:"expiry_date,omitempty"` // Can be float64 (ms) or string
	Token        string `json:"token,omitempty"`
}

// ToOAuthCredentials converts the Antigravity credential format to OAuthCredentials.
func (a *AntigravityCredentials) ToOAuthCredentials() *OAuthCredentials {
	accessToken := a.AccessToken
	if accessToken == "" {
		accessToken = a.Token
	}
	if accessToken == "" {
		return nil
	}
	creds := &OAuthCredentials{
		AccessToken:  accessToken,
		RefreshToken: a.RefreshToken,
		ExpiresAt:    a.ExpiresAt,
	}
	if creds.ExpiresAt == "" {
		creds.ExpiresAt = parseExpiryDate(a.ExpiryDate)
	}
	return creds
}

// parseExpiryDate converts a mixed-type expiry_date to an RFC3339 string.
func parseExpiryDate(v any) string {
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
