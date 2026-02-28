package openrouter

// CreditsResponse represents the response from the OpenRouter credits endpoint
// (GET /api/v1/credits).
type CreditsResponse struct {
	Data CreditsData `json:"data"`
}

// CreditsData contains credit balance information.
type CreditsData struct {
	TotalCredits float64 `json:"total_credits"` // total credits purchased
	TotalUsage   float64 `json:"total_usage"`   // total credits consumed
}

// KeyResponse represents the response from the OpenRouter key info endpoint
// (GET /api/v1/key). This provides richer account data than /api/v1/credits
// including tier, usage breakdowns, and rate limits.
type KeyResponse struct {
	Data KeyData `json:"data"`
}

// KeyData contains API key metadata, tier information, and usage breakdowns.
type KeyData struct {
	Label              string   `json:"label"`
	IsFreeTier         bool     `json:"is_free_tier"`
	Limit              *float64 `json:"limit"`
	LimitReset         *string  `json:"limit_reset"`
	LimitRemaining     *float64 `json:"limit_remaining"`
	IncludeBYOKInLimit bool     `json:"include_byok_in_limit"`
	Usage              float64  `json:"usage"`
	UsageDaily         float64  `json:"usage_daily"`
	UsageWeekly        float64  `json:"usage_weekly"`
	UsageMonthly       float64  `json:"usage_monthly"`
	BYOKUsage          float64  `json:"byok_usage"`
	BYOKUsageDaily     float64  `json:"byok_usage_daily"`
	BYOKUsageWeekly    float64  `json:"byok_usage_weekly"`
	BYOKUsageMonthly   float64  `json:"byok_usage_monthly"`
	RateLimit          KeyRate  `json:"rate_limit"`
}

// KeyRate contains rate limit information for the API key.
type KeyRate struct {
	Requests int `json:"requests"`
	Interval int `json:"interval"`
}
