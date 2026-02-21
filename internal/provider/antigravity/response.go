package antigravity

import "time"

// FetchAvailableModelsResponse represents the response from the
// cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels endpoint.
type FetchAvailableModelsResponse struct {
	Models map[string]ModelInfo `json:"models,omitempty"`
}

// ModelInfo represents a single model's info from the fetchAvailableModels response.
type ModelInfo struct {
	DisplayName string     `json:"displayName,omitempty"`
	QuotaInfo   *QuotaInfo `json:"quotaInfo,omitempty"`
	Recommended bool       `json:"recommended,omitempty"`
}

// QuotaInfo contains the remaining fraction and reset time for a model.
type QuotaInfo struct {
	RemainingFraction *float64 `json:"remainingFraction,omitempty"`
	ResetTime         string   `json:"resetTime,omitempty"`
}

// Utilization returns the usage percentage, clamped to [0, 100].
// If remainingFraction is absent, assumes full quota remaining (0% used).
func (q *QuotaInfo) Utilization() int {
	if q == nil {
		return 0
	}
	rf := 1.0
	if q.RemainingFraction != nil {
		rf = *q.RemainingFraction
	}
	pct := int((1 - rf) * 100)
	return max(0, min(pct, 100))
}

// ResetTimeUTC parses the resetTime as a time.Time.
func (q *QuotaInfo) ResetTimeUTC() *time.Time {
	if q == nil || q.ResetTime == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, q.ResetTime); err == nil {
		return &t
	}
	return nil
}

// CodeAssistResponse represents the response from the
// cloudcode-pa.googleapis.com/v1internal:loadCodeAssist endpoint.
type CodeAssistResponse struct {
	CurrentTier *TierInfo `json:"currentTier,omitempty"`
	UserTier    string    `json:"user_tier,omitempty"` // fallback field
}

// TierInfo represents subscription tier information.
type TierInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// EffectiveTier returns the user's tier name from whichever field is present.
func (c *CodeAssistResponse) EffectiveTier() string {
	if c == nil {
		return ""
	}
	if c.CurrentTier != nil && c.CurrentTier.Name != "" {
		return c.CurrentTier.Name
	}
	if c.CurrentTier != nil && c.CurrentTier.ID != "" {
		return c.CurrentTier.ID
	}
	return c.UserTier
}

// CodeAssistRequest represents the request body for the loadCodeAssist endpoint.
type CodeAssistRequest struct {
	Metadata *CodeAssistRequestMetadata `json:"metadata,omitempty"`
}

// CodeAssistRequestMetadata identifies the requesting IDE.
type CodeAssistRequestMetadata struct {
	IDEType    string `json:"ideType"`
	Platform   string `json:"platform"`
	PluginType string `json:"pluginType"`
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

// AntigravityCredentials represents a JSON credential file format.
type AntigravityCredentials struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiryDate   any    `json:"expiry_date,omitempty"`
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

// VscdbAuthStatus represents the JSON blob stored in Antigravity's VS Code
// state database under the "antigravityAuthStatus" key.
type VscdbAuthStatus struct {
	Name                        string `json:"name,omitempty"`
	APIKey                      string `json:"apiKey,omitempty"`
	Email                       string `json:"email,omitempty"`
	UserStatusProtoBinaryBase64 string `json:"userStatusProtoBinaryBase64,omitempty"`
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
