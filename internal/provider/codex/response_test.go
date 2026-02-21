package codex

import (
	"encoding/json"
	"testing"
)

func TestUsageResponse_UnmarshalWithRateLimit(t *testing.T) {
	raw := `{
		"rate_limit": {
			"primary_window": {"used_percent": 42.0, "reset_at": 1740000000},
			"secondary_window": {"used_percent": 75.0, "reset_at": 1740100000}
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

	primary := rl.EffectivePrimary()
	if primary == nil {
		t.Fatal("expected primary window")
	}
	if primary.UsedPercent != 42.0 {
		t.Errorf("primary used_percent = %v, want 42.0", primary.UsedPercent)
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

func TestTokenResponse_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "new-token",
		"refresh_token": "new-refresh",
		"expires_in": 3600
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "new-token" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "new-token")
	}
	if resp.RefreshToken != "new-refresh" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "new-refresh")
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("expires_in = %v, want 3600", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{"access_token": "tok"}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "tok" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "tok")
	}
	if resp.ExpiresIn != 0 {
		t.Errorf("expires_in = %v, want 0", resp.ExpiresIn)
	}
}

func TestCredentials_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "my-token",
		"refresh_token": "my-refresh",
		"expires_at": "2025-02-19T22:00:00Z"
	}`

	var creds Credentials
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
		creds Credentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: Credentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name:  "expired",
			creds: Credentials{AccessToken: "tok", ExpiresAt: "2020-01-01T00:00:00Z"},
			want:  true,
		},
		{
			name:  "far future",
			creds: Credentials{AccessToken: "tok", ExpiresAt: "2099-01-01T00:00:00Z"},
			want:  false,
		},
		{
			name:  "invalid date",
			creds: Credentials{AccessToken: "tok", ExpiresAt: "garbage"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	original := Credentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-19T22:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Credentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}
