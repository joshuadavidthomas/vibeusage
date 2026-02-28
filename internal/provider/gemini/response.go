package gemini

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/google"
	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// QuotaResponse represents the response from the Gemini quota endpoint.
type QuotaResponse struct {
	Buckets []QuotaBucket `json:"buckets,omitempty"`
}

// QuotaBucket represents a single quota bucket for a model.
type QuotaBucket struct {
	ResetTime         string   `json:"resetTime,omitempty"`
	TokenType         string   `json:"tokenType,omitempty"`
	ModelID           string   `json:"modelId,omitempty"`
	RemainingFraction *float64 `json:"remainingFraction,omitempty"`
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
	return models.ParseRFC3339Ptr(b.ResetTime)
}

// CodeAssistResponse represents the response from the Gemini code assist endpoint.
type CodeAssistResponse struct {
	CurrentTier             *CodeAssistTier  `json:"currentTier,omitempty"`
	AllowedTiers            []CodeAssistTier `json:"allowedTiers,omitempty"`
	PaidTier                *CodeAssistTier  `json:"paidTier,omitempty"`
	CloudAICompanionProject string           `json:"cloudaicompanionProject,omitempty"`
	GCPManaged              bool             `json:"gcpManaged,omitempty"`
	ManageSubscriptionURI   string           `json:"manageSubscriptionUri,omitempty"`
}

// CodeAssistTier represents a tier in the code assist response.
type CodeAssistTier struct {
	ID                                 string         `json:"id,omitempty"`
	Name                               string         `json:"name,omitempty"`
	Description                        string         `json:"description,omitempty"`
	UserDefinedCloudAICompanionProject bool           `json:"userDefinedCloudaicompanionProject,omitempty"`
	PrivacyNotice                      map[string]any `json:"privacyNotice,omitempty"`
	IsDefault                          bool           `json:"isDefault,omitempty"`
	UsesGCPTos                         bool           `json:"usesGcpTos,omitempty"`
}

// UserTier returns a display name for the current tier.
// Returns empty string if no current tier is set.
func (r *CodeAssistResponse) UserTier() string {
	if r == nil || r.CurrentTier == nil {
		return ""
	}
	return r.CurrentTier.Name
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
func (g *GeminiCLIInstalled) ToOAuthCredentials() *oauth.Credentials {
	accessToken := g.Token
	if accessToken == "" {
		accessToken = g.AccessToken
	}
	if accessToken == "" {
		return nil
	}
	creds := &oauth.Credentials{
		AccessToken:  accessToken,
		RefreshToken: g.RefreshToken,
	}
	creds.ExpiresAt = google.ParseExpiryDate(g.ExpiryDate)
	return creds
}

// EffectiveCredentials returns OAuthCredentials from whichever format is present.
func (g *GeminiCLICredentials) EffectiveCredentials() *oauth.Credentials {
	// Try "installed" nested format first
	if g.Installed != nil {
		return g.Installed.ToOAuthCredentials()
	}
	// Try "token" + "refresh_token" format
	if g.Token != "" {
		creds := &oauth.Credentials{
			AccessToken:  g.Token,
			RefreshToken: g.RefreshToken,
		}
		creds.ExpiresAt = google.ParseExpiryDate(g.ExpiryDate)
		return creds
	}
	// Try "access_token" format
	if g.AccessToken != "" {
		return &oauth.Credentials{
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
