package copilot

import (
	"encoding/json"
	"testing"
)

func TestUserResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"quota_reset_date_utc": "2025-03-01T00:00:00Z",
		"copilot_plan": "pro",
		"quota_snapshots": {
			"premium_interactions": {"entitlement": 100, "remaining": 60, "unlimited": false},
			"chat": {"entitlement": 500, "remaining": 300, "unlimited": false},
			"completions": {"entitlement": 0, "remaining": 0, "unlimited": true}
		}
	}`

	var resp UserResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.QuotaResetDateUTC != "2025-03-01T00:00:00Z" {
		t.Errorf("quota_reset_date_utc = %q, want %q", resp.QuotaResetDateUTC, "2025-03-01T00:00:00Z")
	}
	if resp.CopilotPlan != "pro" {
		t.Errorf("copilot_plan = %q, want %q", resp.CopilotPlan, "pro")
	}
	if resp.QuotaSnapshots == nil {
		t.Fatal("expected quota_snapshots")
	}
	if resp.QuotaSnapshots.PremiumInteractions == nil {
		t.Fatal("expected premium_interactions")
	}
	if resp.QuotaSnapshots.PremiumInteractions.Entitlement != 100 {
		t.Errorf("premium entitlement = %v, want 100", resp.QuotaSnapshots.PremiumInteractions.Entitlement)
	}
	if resp.QuotaSnapshots.PremiumInteractions.Remaining != 60 {
		t.Errorf("premium remaining = %v, want 60", resp.QuotaSnapshots.PremiumInteractions.Remaining)
	}
	if resp.QuotaSnapshots.PremiumInteractions.Unlimited {
		t.Error("expected premium unlimited to be false")
	}

	if resp.QuotaSnapshots.Chat == nil {
		t.Fatal("expected chat quota")
	}
	if resp.QuotaSnapshots.Completions == nil {
		t.Fatal("expected completions quota")
	}
	if !resp.QuotaSnapshots.Completions.Unlimited {
		t.Error("expected completions unlimited to be true")
	}
}

func TestUserResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{}`

	var resp UserResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.QuotaResetDateUTC != "" {
		t.Errorf("quota_reset_date_utc = %q, want empty", resp.QuotaResetDateUTC)
	}
	if resp.CopilotPlan != "" {
		t.Errorf("copilot_plan = %q, want empty", resp.CopilotPlan)
	}
	if resp.QuotaSnapshots != nil {
		t.Error("expected nil quota_snapshots")
	}
}

func TestQuota_Utilization(t *testing.T) {
	tests := []struct {
		name string
		q    Quota
		want int
	}{
		{
			name: "normal usage",
			q:    Quota{Entitlement: 100, Remaining: 60, Unlimited: false},
			want: 40,
		},
		{
			name: "fully used",
			q:    Quota{Entitlement: 100, Remaining: 0, Unlimited: false},
			want: 100,
		},
		{
			name: "unlimited with zero entitlement",
			q:    Quota{Entitlement: 0, Remaining: 0, Unlimited: true},
			want: 0,
		},
		{
			name: "zero entitlement not unlimited",
			q:    Quota{Entitlement: 0, Remaining: 0, Unlimited: false},
			want: 0,
		},
		{
			name: "remaining exceeds entitlement clamped to 0",
			q:    Quota{Entitlement: 100, Remaining: 150, Unlimited: false},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuota_HasUsage(t *testing.T) {
	tests := []struct {
		name string
		q    Quota
		want bool
	}{
		{
			name: "unlimited zero entitlement",
			q:    Quota{Entitlement: 0, Remaining: 0, Unlimited: true},
			want: true,
		},
		{
			name: "has entitlement",
			q:    Quota{Entitlement: 100, Remaining: 50, Unlimited: false},
			want: true,
		},
		{
			name: "zero everything",
			q:    Quota{Entitlement: 0, Remaining: 0, Unlimited: false},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.HasUsage()
			if got != tt.want {
				t.Errorf("HasUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceCodeResponse_Unmarshal(t *testing.T) {
	raw := `{
		"device_code": "dc-123",
		"user_code": "ABCD1234",
		"verification_uri": "https://github.com/login/device",
		"interval": 5
	}`

	var resp DeviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.DeviceCode != "dc-123" {
		t.Errorf("device_code = %q, want %q", resp.DeviceCode, "dc-123")
	}
	if resp.UserCode != "ABCD1234" {
		t.Errorf("user_code = %q, want %q", resp.UserCode, "ABCD1234")
	}
	if resp.VerificationURI != "https://github.com/login/device" {
		t.Errorf("verification_uri = %q, want %q", resp.VerificationURI, "https://github.com/login/device")
	}
	if resp.Interval != 5 {
		t.Errorf("interval = %v, want 5", resp.Interval)
	}
}

func TestDeviceCodeResponse_UnmarshalDefaultInterval(t *testing.T) {
	raw := `{"device_code": "dc", "user_code": "UC", "verification_uri": "https://example.com"}`

	var resp DeviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Interval != 0 {
		t.Errorf("interval = %v, want 0 (default)", resp.Interval)
	}
}

func TestTokenResponse_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "gho_xxxx",
		"token_type": "bearer",
		"scope": "read:user"
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "gho_xxxx" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "gho_xxxx")
	}
	if resp.RefreshToken != "" {
		t.Errorf("refresh_token = %q, want empty", resp.RefreshToken)
	}
	if resp.ExpiresIn != 0 {
		t.Errorf("expires_in = %v, want 0", resp.ExpiresIn)
	}
	if resp.Error != "" {
		t.Errorf("error = %q, want empty", resp.Error)
	}
}

func TestTokenResponse_UnmarshalWithRefresh(t *testing.T) {
	raw := `{
		"access_token": "ghu_xxxx",
		"refresh_token": "ghr_xxxx",
		"expires_in": 28800,
		"refresh_token_expires_in": 15897600,
		"token_type": "bearer",
		"scope": "read:user"
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "ghu_xxxx" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "ghu_xxxx")
	}
	if resp.RefreshToken != "ghr_xxxx" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "ghr_xxxx")
	}
	if resp.ExpiresIn != 28800 {
		t.Errorf("expires_in = %v, want 28800", resp.ExpiresIn)
	}
	if resp.RefreshTokenExpiresIn != 15897600 {
		t.Errorf("refresh_token_expires_in = %v, want 15897600", resp.RefreshTokenExpiresIn)
	}
}

func TestTokenResponse_UnmarshalError(t *testing.T) {
	raw := `{
		"error": "authorization_pending",
		"error_description": "The authorization request is still pending."
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "" {
		t.Errorf("access_token = %q, want empty", resp.AccessToken)
	}
	if resp.Error != "authorization_pending" {
		t.Errorf("error = %q, want %q", resp.Error, "authorization_pending")
	}
	if resp.ErrorDescription != "The authorization request is still pending." {
		t.Errorf("error_description = %q, want %q", resp.ErrorDescription, "The authorization request is still pending.")
	}
}

func TestOAuthCredentials_UnmarshalLegacy(t *testing.T) {
	// Legacy format: only access_token, no refresh or expiry
	raw := `{"access_token": "gho_xxxx"}`

	var creds OAuthCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "gho_xxxx" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "gho_xxxx")
	}
	if creds.RefreshToken != "" {
		t.Errorf("refresh_token = %q, want empty", creds.RefreshToken)
	}
	if creds.ExpiresAt != "" {
		t.Errorf("expires_at = %q, want empty", creds.ExpiresAt)
	}
	// Legacy tokens with no ExpiresAt should never trigger refresh
	if creds.NeedsRefresh() {
		t.Error("legacy credentials with no expiry should not need refresh")
	}
}

func TestOAuthCredentials_UnmarshalFull(t *testing.T) {
	raw := `{
		"access_token": "ghu_xxxx",
		"refresh_token": "ghr_xxxx",
		"expires_at": "2025-02-20T06:00:00Z"
	}`

	var creds OAuthCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "ghu_xxxx" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "ghu_xxxx")
	}
	if creds.RefreshToken != "ghr_xxxx" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "ghr_xxxx")
	}
	if creds.ExpiresAt != "2025-02-20T06:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2025-02-20T06:00:00Z")
	}
}

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := OAuthCredentials{
		AccessToken:  "ghu_xxxx",
		RefreshToken: "ghr_xxxx",
		ExpiresAt:    "2025-02-20T06:00:00Z",
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
