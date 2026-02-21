package antigravity

import (
	"encoding/json"
	"testing"
	"time"
)

func ptrFloat64(f float64) *float64 { return &f }

func TestQuotaResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"quota_buckets": [
			{
				"model_id": "models/gemini-2.5-pro",
				"remaining_fraction": 0.75,
				"reset_time": "2026-02-20T00:00:00Z"
			},
			{
				"model_id": "models/gemini-2.0-flash",
				"remaining_fraction": 0.5,
				"reset_time": "2026-02-20T00:00:00Z"
			}
		]
	}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.QuotaBuckets) != 2 {
		t.Fatalf("len(quota_buckets) = %d, want 2", len(resp.QuotaBuckets))
	}

	b := resp.QuotaBuckets[0]
	if b.ModelID != "models/gemini-2.5-pro" {
		t.Errorf("model_id = %q, want %q", b.ModelID, "models/gemini-2.5-pro")
	}
	if b.RemainingFraction == nil || *b.RemainingFraction != 0.75 {
		t.Errorf("remaining_fraction = %v, want 0.75", b.RemainingFraction)
	}
	if b.ResetTime != "2026-02-20T00:00:00Z" {
		t.Errorf("reset_time = %q, want %q", b.ResetTime, "2026-02-20T00:00:00Z")
	}
}

func TestQuotaResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.QuotaBuckets != nil {
		t.Errorf("expected nil quota_buckets, got %v", resp.QuotaBuckets)
	}
}

func TestQuotaBucket_Utilization(t *testing.T) {
	tests := []struct {
		name              string
		remainingFraction *float64
		want              int
	}{
		{"75% remaining", ptrFloat64(0.75), 25},
		{"0% remaining", ptrFloat64(0.0), 100},
		{"100% remaining", ptrFloat64(1.0), 0},
		{"50% remaining", ptrFloat64(0.5), 50},
		{"nil defaults to 0% used", nil, 0},
		{"remaining > 1.0 clamped to 0", ptrFloat64(1.5), 0},
		{"negative remaining clamped to 100", ptrFloat64(-0.5), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := QuotaBucket{RemainingFraction: tt.remainingFraction}
			got := b.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuotaBucket_ResetTimeUTC(t *testing.T) {
	tests := []struct {
		name      string
		resetTime string
		wantNil   bool
		wantTime  time.Time
	}{
		{
			name:      "valid time",
			resetTime: "2026-02-20T00:00:00Z",
			wantNil:   false,
			wantTime:  time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "empty",
			wantNil: true,
		},
		{
			name:      "invalid",
			resetTime: "garbage",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := QuotaBucket{ResetTime: tt.resetTime}
			got := b.ResetTimeUTC()
			if tt.wantNil {
				if got != nil {
					t.Errorf("ResetTimeUTC() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ResetTimeUTC() = nil, want non-nil")
			}
			if !got.Equal(tt.wantTime) {
				t.Errorf("ResetTimeUTC() = %v, want %v", got, tt.wantTime)
			}
		})
	}
}

func TestCodeAssistResponse_Unmarshal(t *testing.T) {
	raw := `{"user_tier": "premium"}`

	var resp CodeAssistResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.UserTier != "premium" {
		t.Errorf("user_tier = %q, want %q", resp.UserTier, "premium")
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

func TestOAuthCredentials_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name  string
		creds OAuthCredentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: OAuthCredentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name:  "expired",
			creds: OAuthCredentials{AccessToken: "tok", ExpiresAt: "2020-01-01T00:00:00Z"},
			want:  true,
		},
		{
			name:  "far future",
			creds: OAuthCredentials{AccessToken: "tok", ExpiresAt: "2099-01-01T00:00:00Z"},
			want:  false,
		},
		{
			name:  "invalid",
			creds: OAuthCredentials{AccessToken: "tok", ExpiresAt: "garbage"},
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

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := OAuthCredentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2026-02-20T00:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded OAuthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestAntigravityCredentials_AccessTokenFormat(t *testing.T) {
	raw := `{
		"access_token": "at-val",
		"refresh_token": "ref-val",
		"expires_at": "2026-02-20T00:00:00Z"
	}`

	var agCreds AntigravityCredentials
	if err := json.Unmarshal([]byte(raw), &agCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := agCreds.ToOAuthCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "at-val" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "at-val")
	}
	if creds.RefreshToken != "ref-val" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "ref-val")
	}
	if creds.ExpiresAt != "2026-02-20T00:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2026-02-20T00:00:00Z")
	}
}

func TestAntigravityCredentials_TokenFormat(t *testing.T) {
	raw := `{
		"token": "tok-val",
		"refresh_token": "ref-val",
		"expiry_date": 1740000000000
	}`

	var agCreds AntigravityCredentials
	if err := json.Unmarshal([]byte(raw), &agCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := agCreds.ToOAuthCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "tok-val" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "tok-val")
	}
	if creds.ExpiresAt == "" {
		t.Error("expected expires_at to be set from expiry_date")
	}
}

func TestAntigravityCredentials_Empty(t *testing.T) {
	raw := `{}`

	var agCreds AntigravityCredentials
	if err := json.Unmarshal([]byte(raw), &agCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := agCreds.ToOAuthCredentials()
	if creds != nil {
		t.Errorf("expected nil credentials, got %+v", creds)
	}
}

func TestParseExpiryDate(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{
			name: "float64 ms timestamp",
			v:    float64(1740000000000),
			want: "2025-02-19T21:20:00Z",
		},
		{
			name: "string",
			v:    "2026-02-20T00:00:00Z",
			want: "2026-02-20T00:00:00Z",
		},
		{
			name: "nil",
			v:    nil,
			want: "",
		},
		{
			name: "zero float64",
			v:    float64(0),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExpiryDate(tt.v)
			if got != tt.want {
				t.Errorf("parseExpiryDate(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestQuotaRequest_Marshal(t *testing.T) {
	req := QuotaRequest{
		Metadata: QuotaRequestMetadata{
			IDEType:    "ANTIGRAVITY",
			Platform:   "PLATFORM_UNSPECIFIED",
			PluginType: "GEMINI",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	want := `{"metadata":{"ideType":"ANTIGRAVITY","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
	if string(data) != want {
		t.Errorf("marshal = %s, want %s", string(data), want)
	}
}
