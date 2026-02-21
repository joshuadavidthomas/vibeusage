package gemini

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/provider/googleauth"
)

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

// CodeAssistResponse represents the response from the Gemini code assist endpoint.
type CodeAssistResponse struct {
	UserTier string `json:"user_tier,omitempty"`
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
func (g *GeminiCLIInstalled) ToOAuthCredentials() *googleauth.OAuthCredentials {
	accessToken := g.Token
	if accessToken == "" {
		accessToken = g.AccessToken
	}
	if accessToken == "" {
		return nil
	}
	creds := &googleauth.OAuthCredentials{
		AccessToken:  accessToken,
		RefreshToken: g.RefreshToken,
	}
	creds.ExpiresAt = googleauth.ParseExpiryDate(g.ExpiryDate)
	return creds
}

// EffectiveCredentials returns OAuthCredentials from whichever format is present.
func (g *GeminiCLICredentials) EffectiveCredentials() *googleauth.OAuthCredentials {
	// Try "installed" nested format first
	if g.Installed != nil {
		return g.Installed.ToOAuthCredentials()
	}
	// Try "token" + "refresh_token" format
	if g.Token != "" {
		creds := &googleauth.OAuthCredentials{
			AccessToken:  g.Token,
			RefreshToken: g.RefreshToken,
		}
		creds.ExpiresAt = googleauth.ParseExpiryDate(g.ExpiryDate)
		return creds
	}
	// Try "access_token" format
	if g.AccessToken != "" {
		return &googleauth.OAuthCredentials{
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
