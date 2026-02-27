package googleauth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
)

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

func TestOAuthCredentials_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "my-token",
		"refresh_token": "my-refresh",
		"expires_at": "2025-02-20T00:00:00Z"
	}`

	var creds oauth.Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "my-token" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "my-token")
	}
}

func TestOAuthCredentials_NeedsRefresh(t *testing.T) {
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
			name:  "invalid",
			creds: oauth.Credentials{AccessToken: "tok", ExpiresAt: "garbage"},
			want:  true,
		},
		{
			name: "within buffer",
			creds: oauth.Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "outside buffer",
			creds: oauth.Credentials{
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

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := oauth.Credentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-20T00:00:00Z",
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

func TestExtractAPIError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "standard google error",
			body: `{"error":{"code":401,"message":"Request had invalid authentication credentials.","status":"UNAUTHENTICATED"}}`,
			want: "Request had invalid authentication credentials.",
		},
		{
			name: "empty body",
			body: "",
			want: "empty response",
		},
		{
			name: "non-json body",
			body: "Service Unavailable",
			want: "Service Unavailable",
		},
		{
			name: "json without error field",
			body: `{"status":"fail"}`,
			want: `{"status":"fail"}`,
		},
		{
			name: "long body is truncated",
			body: string(make([]byte, 300)),
			want: string(make([]byte, 200)) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAPIError([]byte(tt.body))
			if got != tt.want {
				t.Errorf("ExtractAPIError() = %q, want %q", got, tt.want)
			}
		})
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
			v:    "2025-02-20T00:00:00Z",
			want: "2025-02-20T00:00:00Z",
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
			got := ParseExpiryDate(tt.v)
			if got != tt.want {
				t.Errorf("ParseExpiryDate(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}
