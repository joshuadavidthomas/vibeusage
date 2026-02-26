package deviceflow

import (
	"encoding/json"
	"testing"
)

func TestDeviceCodeResponse_Unmarshal(t *testing.T) {
	raw := `{
		"device_code": "dc-123",
		"user_code": "ABCD1234",
		"verification_uri": "https://github.com/login/device",
		"interval": 5
	}`

	var resp deviceCodeResponse
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

	var resp deviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Interval != 0 {
		t.Errorf("interval = %v, want 0 (default)", resp.Interval)
	}
}

func TestDeviceCodeResponse_UnmarshalComplete(t *testing.T) {
	raw := `{
		"user_code": "ABCD-1234",
		"device_code": "dc-123",
		"verification_uri_complete": "https://auth.kimi.com/device?code=ABCD-1234",
		"interval": 5,
		"expires_in": 600
	}`

	var resp deviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.VerificationURIComplete != "https://auth.kimi.com/device?code=ABCD-1234" {
		t.Errorf("verification_uri_complete = %q", resp.VerificationURIComplete)
	}
	if resp.ExpiresIn != 600 {
		t.Errorf("expires_in = %d, want 600", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalSuccess(t *testing.T) {
	raw := `{
		"access_token": "gho_xxxx",
		"token_type": "bearer",
		"scope": "read:user"
	}`

	var resp tokenResponse
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

	var resp tokenResponse
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
}

func TestTokenResponse_UnmarshalError(t *testing.T) {
	raw := `{
		"error": "authorization_pending",
		"error_description": "The authorization request is still pending."
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "" {
		t.Errorf("access_token = %q, want empty", resp.AccessToken)
	}
	if resp.Error != "authorization_pending" {
		t.Errorf("error = %q, want %q", resp.Error, "authorization_pending")
	}
	if resp.ErrorDesc != "The authorization request is still pending." {
		t.Errorf("error_description = %q, want %q", resp.ErrorDesc, "The authorization request is still pending.")
	}
}

func TestTokenResponse_UnmarshalIntegerExpiresIn(t *testing.T) {
	// KimiCode returns expires_in as an integer; ensure float64 handles it.
	raw := `{
		"access_token": "eyJhbG...",
		"refresh_token": "rt-xxx",
		"expires_in": 900
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.ExpiresIn != 900 {
		t.Errorf("expires_in = %v, want 900", resp.ExpiresIn)
	}
}

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := oauthCredentials{
		AccessToken:  "ghu_xxxx",
		RefreshToken: "ghr_xxxx",
		ExpiresAt:    "2025-02-20T06:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded oauthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}
