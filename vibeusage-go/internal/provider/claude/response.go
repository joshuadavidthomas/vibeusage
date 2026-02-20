package claude

import "time"

// UsagePeriodResponse represents a single usage period from the Claude OAuth API.
type UsagePeriodResponse struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at,omitempty"`
}

// ExtraUsageResponse represents overage/extra usage info from the Claude OAuth API.
type ExtraUsageResponse struct {
	IsEnabled    bool    `json:"is_enabled"`
	UsedCredits  float64 `json:"used_credits"`
	MonthlyLimit float64 `json:"monthly_limit"`
}

// OAuthUsageResponse represents the full response from /api/oauth/usage.
type OAuthUsageResponse struct {
	FiveHour       *UsagePeriodResponse `json:"five_hour,omitempty"`
	SevenDay       *UsagePeriodResponse `json:"seven_day,omitempty"`
	Monthly        *UsagePeriodResponse `json:"monthly,omitempty"`
	SevenDaySonnet *UsagePeriodResponse `json:"seven_day_sonnet,omitempty"`
	SevenDayOpus   *UsagePeriodResponse `json:"seven_day_opus,omitempty"`
	SevenDayHaiku  *UsagePeriodResponse `json:"seven_day_haiku,omitempty"`
	ExtraUsage     *ExtraUsageResponse  `json:"extra_usage,omitempty"`
}

// OAuthTokenResponse represents the response from /oauth/token.
type OAuthTokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	TokenType    string  `json:"token_type,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
}

// OAuthCredentials represents stored OAuth credentials (vibeusage format).
type OAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// ClaudeCLIOAuth represents the nested OAuth data inside Claude CLI credentials.
type ClaudeCLIOAuth struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	ExpiresAt    float64 `json:"expiresAt,omitempty"` // millisecond timestamp
}

// ToOAuthCredentials converts Claude CLI format to standard OAuthCredentials.
func (c *ClaudeCLIOAuth) ToOAuthCredentials() OAuthCredentials {
	creds := OAuthCredentials{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
	}
	if c.ExpiresAt > 0 {
		t := time.UnixMilli(int64(c.ExpiresAt))
		creds.ExpiresAt = t.UTC().Format(time.RFC3339)
	}
	return creds
}

// NeedsRefresh reports whether the credentials have expired or have an unparseable expiry.
func (c OAuthCredentials) NeedsRefresh() bool {
	if c.ExpiresAt == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(expiry)
}

// ClaudeCLICredentials represents the Claude CLI credentials file format.
type ClaudeCLICredentials struct {
	ClaudeAiOauth *ClaudeCLIOAuth `json:"claudeAiOauth,omitempty"`
}
