package gemini

import "time"

// QuotaResponse represents the response from the Gemini quota endpoint.
type QuotaResponse struct {
	QuotaBuckets []QuotaBucket `json:"quota_buckets,omitempty"`
}

// QuotaBucket represents a single quota bucket for a model.
type QuotaBucket struct {
	ModelID           string   `json:"model_id,omitempty"`
	RemainingFraction *float64 `json:"remaining_fraction,omitempty"`
	ResetTime         string   `json:"reset_time,omitempty"`
}

// Utilization returns the usage percentage (0-100).
// If remaining_fraction is absent, assumes full quota remaining (0% used).
func (b *QuotaBucket) Utilization() int {
	rf := 1.0
	if b.RemainingFraction != nil {
		rf = *b.RemainingFraction
	}
	return int((1 - rf) * 100)
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

// CodeAssistResponse represents the response from the Gemini code assist endpoint.
type CodeAssistResponse struct {
	UserTier string `json:"user_tier,omitempty"`
}

// TokenResponse represents the response from the Google OAuth token refresh endpoint.
type TokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials for Gemini.
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

// GeminiCLICredentials represents the Gemini CLI "installed" format.
type GeminiCLICredentials struct {
	Installed *GeminiCLIInstalled `json:"installed,omitempty"`
	// Alternate flat format
	Token        string `json:"token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiryDate   any    `json:"expiry_date,omitempty"` // Can be float64 (ms) or string
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// GeminiCLIInstalled represents the nested "installed" format from Gemini CLI.
type GeminiCLIInstalled struct {
	Token        string `json:"token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiryDate   any    `json:"expiry_date,omitempty"`
}

// ToOAuthCredentials converts the CLI installed format to OAuthCredentials.
func (g *GeminiCLIInstalled) ToOAuthCredentials() *OAuthCredentials {
	accessToken := g.Token
	if accessToken == "" {
		accessToken = g.AccessToken
	}
	if accessToken == "" {
		return nil
	}
	creds := &OAuthCredentials{
		AccessToken:  accessToken,
		RefreshToken: g.RefreshToken,
	}
	creds.ExpiresAt = parseExpiryDate(g.ExpiryDate)
	return creds
}

// EffectiveCredentials returns OAuthCredentials from whichever format is present.
func (g *GeminiCLICredentials) EffectiveCredentials() *OAuthCredentials {
	// Try "installed" nested format first
	if g.Installed != nil {
		return g.Installed.ToOAuthCredentials()
	}
	// Try "token" + "refresh_token" format
	if g.Token != "" {
		creds := &OAuthCredentials{
			AccessToken:  g.Token,
			RefreshToken: g.RefreshToken,
		}
		creds.ExpiresAt = parseExpiryDate(g.ExpiryDate)
		return creds
	}
	// Try "access_token" format
	if g.AccessToken != "" {
		return &OAuthCredentials{
			AccessToken:  g.AccessToken,
			RefreshToken: g.RefreshToken,
			ExpiresAt:    g.ExpiresAt,
		}
	}
	return nil
}

// ModelsResponse represents the response from the Gemini models list endpoint.
type ModelsResponse struct {
	Models []GeminiModel `json:"models,omitempty"`
}

// GeminiModel represents a single model from the models endpoint.
type GeminiModel struct {
	Name string `json:"name,omitempty"`
}

// googleIncident represents a single incident from the Google Apps Status API.
type googleIncident struct {
	Title    string `json:"title,omitempty"`
	Severity string `json:"severity,omitempty"`
	EndTime  string `json:"end_time,omitempty"`
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
