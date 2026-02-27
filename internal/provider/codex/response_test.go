package codex

import (
	"encoding/json"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
)

func TestUsageResponse_UnmarshalWithRateLimit(t *testing.T) {
	raw := `{
		"rate_limit": {
			"allowed": true,
			"limit_reached": false,
			"primary_window": {"used_percent": 42.0, "limit_window_seconds": 18000, "reset_after_seconds": 13259, "reset_at": 1740000000},
			"secondary_window": {"used_percent": 75.0, "limit_window_seconds": 604800, "reset_after_seconds": 330020, "reset_at": 1740100000}
		},
		"credits": {"has_credits": true, "balance": 50.0},
		"plan_type": "plus"
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	rl := resp.EffectiveRateLimits()
	if rl == nil {
		t.Fatal("expected rate limits to be present")
	}
	if !rl.Allowed {
		t.Error("expected allowed to be true")
	}
	if rl.LimitReached {
		t.Error("expected limit_reached to be false")
	}

	primary := rl.EffectivePrimary()
	if primary == nil {
		t.Fatal("expected primary window")
	}
	if primary.UsedPercent != 42.0 {
		t.Errorf("primary used_percent = %v, want 42.0", primary.UsedPercent)
	}
	if primary.LimitWindowSeconds != 18000 {
		t.Errorf("primary limit_window_seconds = %v, want 18000", primary.LimitWindowSeconds)
	}
	if primary.ResetAfterSeconds != 13259 {
		t.Errorf("primary reset_after_seconds = %v, want 13259", primary.ResetAfterSeconds)
	}
	if primary.EffectiveResetTimestamp() != 1740000000 {
		t.Errorf("primary reset_at = %v, want 1740000000", primary.EffectiveResetTimestamp())
	}

	secondary := rl.EffectiveSecondary()
	if secondary == nil {
		t.Fatal("expected secondary window")
	}
	if secondary.UsedPercent != 75.0 {
		t.Errorf("secondary used_percent = %v, want 75.0", secondary.UsedPercent)
	}
	if secondary.LimitWindowSeconds != 604800 {
		t.Errorf("secondary limit_window_seconds = %v, want 604800", secondary.LimitWindowSeconds)
	}

	if resp.Credits == nil {
		t.Fatal("expected credits")
	}
	if !resp.Credits.HasCredits {
		t.Error("expected has_credits to be true")
	}
	if resp.Credits.Balance() != 50.0 {
		t.Errorf("balance = %v, want 50.0", resp.Credits.Balance())
	}
	if resp.PlanType != "plus" {
		t.Errorf("plan_type = %q, want %q", resp.PlanType, "plus")
	}
}

func TestUsageResponse_UnmarshalWithStringBalance(t *testing.T) {
	raw := `{
		"rate_limit": {
			"primary_window": {"used_percent": 42.0, "reset_at": 1740000000}
		},
		"credits": {"has_credits": true, "balance": "50.00"},
		"plan_type": "plus"
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Credits == nil {
		t.Fatal("expected credits")
	}
	if resp.Credits.Balance() != 50.0 {
		t.Errorf("balance = %v, want 50.0", resp.Credits.Balance())
	}
}

func TestUsageResponse_UnmarshalWithRateLimits(t *testing.T) {
	raw := `{
		"rate_limits": {
			"primary": {"used_percent": 30.0, "reset_timestamp": 1740000000},
			"secondary": {"used_percent": 60.0, "reset_timestamp": 1740100000}
		}
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	rl := resp.EffectiveRateLimits()
	if rl == nil {
		t.Fatal("expected rate limits to be present")
	}

	primary := rl.EffectivePrimary()
	if primary == nil {
		t.Fatal("expected primary window")
	}
	if primary.UsedPercent != 30.0 {
		t.Errorf("primary used_percent = %v, want 30.0", primary.UsedPercent)
	}
	if primary.EffectiveResetTimestamp() != 1740000000 {
		t.Errorf("primary reset = %v, want 1740000000", primary.EffectiveResetTimestamp())
	}

	secondary := rl.EffectiveSecondary()
	if secondary == nil {
		t.Fatal("expected secondary window")
	}
	if secondary.UsedPercent != 60.0 {
		t.Errorf("secondary used_percent = %v, want 60.0", secondary.UsedPercent)
	}
}

func TestUsageResponse_UnmarshalFullLiveShape(t *testing.T) {
	raw := `{
		"user_id": "user-abc123",
		"account_id": "user-abc123",
		"email": "user@example.com",
		"plan_type": "plus",
		"rate_limit": {
			"allowed": true,
			"limit_reached": false,
			"primary_window": {
				"used_percent": 2,
				"limit_window_seconds": 18000,
				"reset_after_seconds": 13259,
				"reset_at": 1772242545
			},
			"secondary_window": {
				"used_percent": 33,
				"limit_window_seconds": 604800,
				"reset_after_seconds": 330020,
				"reset_at": 1772559306
			}
		},
		"code_review_rate_limit": {
			"allowed": true,
			"limit_reached": false,
			"primary_window": {
				"used_percent": 0,
				"limit_window_seconds": 604800,
				"reset_after_seconds": 604800,
				"reset_at": 1772834086
			},
			"secondary_window": null
		},
		"additional_rate_limits": [
			{
				"limit_name": "GPT-5.3-Codex-Spark",
				"metered_feature": "codex_bengalfox",
				"rate_limit": {
					"allowed": true,
					"limit_reached": false,
					"primary_window": {
						"used_percent": 0,
						"limit_window_seconds": 18000,
						"reset_after_seconds": 18000,
						"reset_at": 1772247286
					},
					"secondary_window": {
						"used_percent": 0,
						"limit_window_seconds": 604800,
						"reset_after_seconds": 604800,
						"reset_at": 1772834086
					}
				}
			}
		],
		"credits": {
			"has_credits": false,
			"unlimited": false,
			"balance": "0",
			"approx_local_messages": [0, 0],
			"approx_cloud_messages": [0, 0]
		},
		"promo": null
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.UserID != "user-abc123" {
		t.Errorf("user_id = %q, want %q", resp.UserID, "user-abc123")
	}
	if resp.AccountID != "user-abc123" {
		t.Errorf("account_id = %q, want %q", resp.AccountID, "user-abc123")
	}
	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", resp.Email, "user@example.com")
	}
	if resp.PlanType != "plus" {
		t.Errorf("plan_type = %q, want %q", resp.PlanType, "plus")
	}

	// Main rate limit
	rl := resp.EffectiveRateLimits()
	if rl == nil {
		t.Fatal("expected rate limits")
	}
	if !rl.Allowed {
		t.Error("expected allowed to be true")
	}
	if rl.LimitReached {
		t.Error("expected limit_reached to be false")
	}

	primary := rl.EffectivePrimary()
	if primary == nil {
		t.Fatal("expected primary window")
	}
	if primary.UsedPercent != 2 {
		t.Errorf("primary used_percent = %v, want 2", primary.UsedPercent)
	}
	if primary.LimitWindowSeconds != 18000 {
		t.Errorf("primary limit_window_seconds = %v, want 18000", primary.LimitWindowSeconds)
	}
	if primary.ResetAfterSeconds != 13259 {
		t.Errorf("primary reset_after_seconds = %v, want 13259", primary.ResetAfterSeconds)
	}

	// Code review rate limit
	cr := resp.CodeReviewRateLimit
	if cr == nil {
		t.Fatal("expected code review rate limit")
	}
	if !cr.Allowed {
		t.Error("expected code review allowed to be true")
	}
	crPrimary := cr.EffectivePrimary()
	if crPrimary == nil {
		t.Fatal("expected code review primary window")
	}
	if crPrimary.UsedPercent != 0 {
		t.Errorf("code review primary used_percent = %v, want 0", crPrimary.UsedPercent)
	}
	if crPrimary.LimitWindowSeconds != 604800 {
		t.Errorf("code review primary limit_window_seconds = %v, want 604800", crPrimary.LimitWindowSeconds)
	}
	if cr.EffectiveSecondary() != nil {
		t.Error("expected code review secondary window to be nil")
	}

	// Additional rate limits
	if len(resp.AdditionalRateLimits) != 1 {
		t.Fatalf("additional_rate_limits length = %d, want 1", len(resp.AdditionalRateLimits))
	}
	arl := resp.AdditionalRateLimits[0]
	if arl.LimitName != "GPT-5.3-Codex-Spark" {
		t.Errorf("limit_name = %q, want %q", arl.LimitName, "GPT-5.3-Codex-Spark")
	}
	if arl.MeteredFeature != "codex_bengalfox" {
		t.Errorf("metered_feature = %q, want %q", arl.MeteredFeature, "codex_bengalfox")
	}
	if arl.RateLimit == nil {
		t.Fatal("expected additional rate limit to have rate_limit")
	}
	if !arl.RateLimit.Allowed {
		t.Error("expected additional rate limit allowed to be true")
	}
	arlPrimary := arl.RateLimit.EffectivePrimary()
	if arlPrimary == nil {
		t.Fatal("expected additional rate limit primary window")
	}
	if arlPrimary.LimitWindowSeconds != 18000 {
		t.Errorf("additional primary limit_window_seconds = %v, want 18000", arlPrimary.LimitWindowSeconds)
	}

	// Credits
	if resp.Credits == nil {
		t.Fatal("expected credits")
	}
	if resp.Credits.HasCredits {
		t.Error("expected has_credits to be false")
	}
	if resp.Credits.Unlimited {
		t.Error("expected unlimited to be false")
	}
	if resp.Credits.Balance() != 0 {
		t.Errorf("balance = %v, want 0", resp.Credits.Balance())
	}
	if resp.Credits.ApproxLocalMessages == nil {
		t.Error("expected approx_local_messages to be present")
	}
	if resp.Credits.ApproxCloudMessages == nil {
		t.Error("expected approx_cloud_messages to be present")
	}
}

func TestUsageResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.EffectiveRateLimits() != nil {
		t.Error("expected nil rate limits")
	}
	if resp.Credits != nil {
		t.Error("expected nil credits")
	}
	if resp.PlanType != "" {
		t.Errorf("plan_type = %q, want empty", resp.PlanType)
	}
}

func TestUsageResponse_NoCredits(t *testing.T) {
	raw := `{
		"rate_limit": {
			"primary_window": {"used_percent": 10.0}
		},
		"credits": {"has_credits": false, "balance": 0}
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Credits == nil {
		t.Fatal("expected credits to be present")
	}
	if resp.Credits.HasCredits {
		t.Error("expected has_credits to be false")
	}
}

func TestRateWindow_EffectiveResetTimestamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want float64
	}{
		{
			name: "reset_at",
			raw:  `{"used_percent": 50.0, "reset_at": 1740000000}`,
			want: 1740000000,
		},
		{
			name: "reset_timestamp",
			raw:  `{"used_percent": 50.0, "reset_timestamp": 1740000000}`,
			want: 1740000000,
		},
		{
			name: "both prefer reset_at",
			raw:  `{"used_percent": 50.0, "reset_at": 1740000000, "reset_timestamp": 1740100000}`,
			want: 1740000000,
		},
		{
			name: "neither",
			raw:  `{"used_percent": 50.0}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w RateWindow
			if err := json.Unmarshal([]byte(tt.raw), &w); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got := w.EffectiveResetTimestamp(); got != tt.want {
				t.Errorf("EffectiveResetTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateWindow_AllFields(t *testing.T) {
	raw := `{
		"used_percent": 33,
		"limit_window_seconds": 604800,
		"reset_after_seconds": 330020,
		"reset_at": 1772559306
	}`

	var w RateWindow
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if w.UsedPercent != 33 {
		t.Errorf("used_percent = %v, want 33", w.UsedPercent)
	}
	if w.LimitWindowSeconds != 604800 {
		t.Errorf("limit_window_seconds = %v, want 604800", w.LimitWindowSeconds)
	}
	if w.ResetAfterSeconds != 330020 {
		t.Errorf("reset_after_seconds = %v, want 330020", w.ResetAfterSeconds)
	}
	if w.ResetAt != 1772559306 {
		t.Errorf("reset_at = %v, want 1772559306", w.ResetAt)
	}
}

func TestRateLimits_StatusFields(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantAllowed bool
		wantReached bool
	}{
		{
			name:        "allowed and not reached",
			raw:         `{"allowed": true, "limit_reached": false, "primary_window": {"used_percent": 10}}`,
			wantAllowed: true,
			wantReached: false,
		},
		{
			name:        "not allowed and reached",
			raw:         `{"allowed": false, "limit_reached": true, "primary_window": {"used_percent": 100}}`,
			wantAllowed: false,
			wantReached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rl RateLimits
			if err := json.Unmarshal([]byte(tt.raw), &rl); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if rl.Allowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v", rl.Allowed, tt.wantAllowed)
			}
			if rl.LimitReached != tt.wantReached {
				t.Errorf("limit_reached = %v, want %v", rl.LimitReached, tt.wantReached)
			}
		})
	}
}

func TestAdditionalRateLimits_Unmarshal(t *testing.T) {
	raw := `{
		"rate_limit": {
			"primary_window": {"used_percent": 10}
		},
		"additional_rate_limits": [
			{
				"limit_name": "GPT-5.3-Codex-Spark",
				"metered_feature": "codex_bengalfox",
				"rate_limit": {
					"allowed": true,
					"limit_reached": false,
					"primary_window": {"used_percent": 5, "limit_window_seconds": 18000},
					"secondary_window": {"used_percent": 15, "limit_window_seconds": 604800}
				}
			},
			{
				"limit_name": "o3-pro",
				"metered_feature": "codex_o3pro",
				"rate_limit": {
					"allowed": true,
					"limit_reached": false,
					"primary_window": {"used_percent": 0, "limit_window_seconds": 86400}
				}
			}
		]
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.AdditionalRateLimits) != 2 {
		t.Fatalf("additional_rate_limits length = %d, want 2", len(resp.AdditionalRateLimits))
	}

	first := resp.AdditionalRateLimits[0]
	if first.LimitName != "GPT-5.3-Codex-Spark" {
		t.Errorf("first limit_name = %q, want %q", first.LimitName, "GPT-5.3-Codex-Spark")
	}
	if first.MeteredFeature != "codex_bengalfox" {
		t.Errorf("first metered_feature = %q, want %q", first.MeteredFeature, "codex_bengalfox")
	}
	if first.RateLimit == nil {
		t.Fatal("first rate_limit is nil")
	}
	if !first.RateLimit.Allowed {
		t.Error("expected first rate limit allowed to be true")
	}
	if fp := first.RateLimit.EffectivePrimary(); fp == nil {
		t.Error("expected first primary window")
	} else if fp.UsedPercent != 5 {
		t.Errorf("first primary used_percent = %v, want 5", fp.UsedPercent)
	}
	if fs := first.RateLimit.EffectiveSecondary(); fs == nil {
		t.Error("expected first secondary window")
	} else if fs.UsedPercent != 15 {
		t.Errorf("first secondary used_percent = %v, want 15", fs.UsedPercent)
	}

	second := resp.AdditionalRateLimits[1]
	if second.LimitName != "o3-pro" {
		t.Errorf("second limit_name = %q, want %q", second.LimitName, "o3-pro")
	}
	if second.RateLimit.EffectiveSecondary() != nil {
		t.Error("expected second secondary window to be nil")
	}
}

func TestCredits_AllFields(t *testing.T) {
	raw := `{
		"has_credits": false,
		"unlimited": false,
		"balance": "0",
		"approx_local_messages": [0, 0],
		"approx_cloud_messages": [0, 0]
	}`

	var c Credits
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if c.HasCredits {
		t.Error("expected has_credits to be false")
	}
	if c.Unlimited {
		t.Error("expected unlimited to be false")
	}
	if c.Balance() != 0 {
		t.Errorf("balance = %v, want 0", c.Balance())
	}
	if c.ApproxLocalMessages == nil {
		t.Error("expected approx_local_messages to be present")
	}
	if c.ApproxCloudMessages == nil {
		t.Error("expected approx_cloud_messages to be present")
	}
}

func TestCredits_Unlimited(t *testing.T) {
	raw := `{
		"has_credits": true,
		"unlimited": true,
		"balance": "999"
	}`

	var c Credits
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !c.HasCredits {
		t.Error("expected has_credits to be true")
	}
	if !c.Unlimited {
		t.Error("expected unlimited to be true")
	}
	if c.Balance() != 999 {
		t.Errorf("balance = %v, want 999", c.Balance())
	}
}

func TestCredentials_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "my-token",
		"refresh_token": "my-refresh",
		"expires_at": "2025-02-19T22:00:00Z"
	}`

	var creds oauth.Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "my-token" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "my-token")
	}
	if creds.RefreshToken != "my-refresh" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "my-refresh")
	}
	if creds.ExpiresAt != "2025-02-19T22:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2025-02-19T22:00:00Z")
	}
}

func TestCredentials_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name  string
		creds oauth.Credentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: oauth.Credentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name:  "expired",
			creds: oauth.Credentials{AccessToken: "tok", ExpiresAt: "2020-01-01T00:00:00Z"},
			want:  true,
		},
		{
			name:  "far future",
			creds: oauth.Credentials{AccessToken: "tok", ExpiresAt: "2099-01-01T00:00:00Z"},
			want:  false,
		},
		{
			name:  "invalid date",
			creds: oauth.Credentials{AccessToken: "tok", ExpiresAt: "garbage"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// oauth.Credentials uses a
			// 5-minute refresh buffer (shared across all providers).
			got := tt.creds.NeedsRefresh()
			if got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCLICredentials_UnmarshalNested(t *testing.T) {
	raw := `{
		"tokens": {
			"access_token": "nested-token",
			"refresh_token": "nested-refresh",
			"expires_at": "2025-02-19T22:00:00Z"
		}
	}`

	var cliCreds CLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cliCreds.Tokens == nil {
		t.Fatal("expected tokens to be present")
	}
	if cliCreds.Tokens.AccessToken != "nested-token" {
		t.Errorf("access_token = %q, want %q", cliCreds.Tokens.AccessToken, "nested-token")
	}
}

func TestCLICredentials_UnmarshalFlat(t *testing.T) {
	raw := `{
		"access_token": "flat-token",
		"refresh_token": "flat-refresh"
	}`

	var cliCreds CLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Tokens field should be nil for flat format
	if cliCreds.Tokens != nil {
		t.Error("expected tokens to be nil for flat format")
	}
	// The flat fields should be populated
	if cliCreds.AccessToken != "flat-token" {
		t.Errorf("access_token = %q, want %q", cliCreds.AccessToken, "flat-token")
	}
}

func TestCLICredentials_EffectiveCredentials(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantTok string
		wantNil bool
	}{
		{
			name:    "nested format",
			raw:     `{"tokens": {"access_token": "nested-tok", "refresh_token": "ref"}}`,
			wantTok: "nested-tok",
		},
		{
			name:    "flat format",
			raw:     `{"access_token": "flat-tok", "refresh_token": "ref"}`,
			wantTok: "flat-tok",
		},
		{
			name:    "empty",
			raw:     `{}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cliCreds CLICredentials
			if err := json.Unmarshal([]byte(tt.raw), &cliCreds); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			creds := cliCreds.EffectiveCredentials()
			if tt.wantNil {
				if creds != nil {
					t.Errorf("EffectiveCredentials() = %+v, want nil", creds)
				}
				return
			}
			if creds == nil {
				t.Fatal("EffectiveCredentials() = nil, want non-nil")
			}
			if creds.AccessToken != tt.wantTok {
				t.Errorf("access_token = %q, want %q", creds.AccessToken, tt.wantTok)
			}
		})
	}
}

func TestCredentials_Roundtrip(t *testing.T) {
	original := oauth.Credentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-19T22:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded oauth.Credentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}
