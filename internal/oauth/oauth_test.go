package oauth

import (
	"encoding/json"
	"testing"
	"time"
)

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
			name: "far future",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "2099-01-01T00:00:00Z",
			},
			want: false,
		},
		{
			name: "expired",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "2020-01-01T00:00:00Z",
			},
			want: true,
		},
		{
			name: "invalid date",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "garbage",
			},
			want: true,
		},
		{
			name: "within buffer",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "outside buffer",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
			},
			want: false,
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

func TestCredentials_Roundtrip(t *testing.T) {
	original := Credentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-20T00:00:00Z",
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
