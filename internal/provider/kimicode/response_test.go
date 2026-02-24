package kimicode

import (
	"encoding/json"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestUsageResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"user": {
			"userId": "d5s8j1fftae5hmncss20",
			"region": "REGION_OVERSEA",
			"membership": {
				"level": "LEVEL_BASIC"
			},
			"businessId": ""
		},
		"usage": {
			"limit": "100",
			"remaining": "100",
			"resetTime": "2026-02-25T04:01:38Z"
		},
		"limits": [
			{
				"window": {
					"duration": 300,
					"timeUnit": "TIME_UNIT_MINUTE"
				},
				"detail": {
					"limit": "100",
					"remaining": "100",
					"resetTime": "2026-02-21T08:01:38Z"
				}
			}
		]
	}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.User == nil {
		t.Fatal("expected user")
	}
	if resp.User.UserID != "d5s8j1fftae5hmncss20" {
		t.Errorf("userId = %q, want %q", resp.User.UserID, "d5s8j1fftae5hmncss20")
	}
	if resp.User.Region != "REGION_OVERSEA" {
		t.Errorf("region = %q, want %q", resp.User.Region, "REGION_OVERSEA")
	}
	if resp.User.Membership == nil {
		t.Fatal("expected membership")
	}
	if resp.User.Membership.Level != "LEVEL_BASIC" {
		t.Errorf("membership.level = %q, want %q", resp.User.Membership.Level, "LEVEL_BASIC")
	}

	if resp.Usage == nil {
		t.Fatal("expected usage")
	}
	if resp.Usage.Limit != "100" {
		t.Errorf("usage.limit = %q, want %q", resp.Usage.Limit, "100")
	}
	if resp.Usage.Remaining != "100" {
		t.Errorf("usage.remaining = %q, want %q", resp.Usage.Remaining, "100")
	}
	if resp.Usage.ResetTime != "2026-02-25T04:01:38Z" {
		t.Errorf("usage.resetTime = %q, want %q", resp.Usage.ResetTime, "2026-02-25T04:01:38Z")
	}

	if len(resp.Limits) != 1 {
		t.Fatalf("expected 1 limit, got %d", len(resp.Limits))
	}
	limit := resp.Limits[0]
	if limit.Window.Duration != 300 {
		t.Errorf("window.duration = %d, want 300", limit.Window.Duration)
	}
	if limit.Window.TimeUnit != "TIME_UNIT_MINUTE" {
		t.Errorf("window.timeUnit = %q, want %q", limit.Window.TimeUnit, "TIME_UNIT_MINUTE")
	}
	if limit.Detail == nil {
		t.Fatal("expected limit detail")
	}
	if limit.Detail.Limit != "100" {
		t.Errorf("detail.limit = %q, want %q", limit.Detail.Limit, "100")
	}
}

func TestUsageResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{}`

	var resp UsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.User != nil {
		t.Error("expected nil user")
	}
	if resp.Usage != nil {
		t.Error("expected nil usage")
	}
	if len(resp.Limits) != 0 {
		t.Errorf("expected 0 limits, got %d", len(resp.Limits))
	}
}

func TestUsageDetail_Utilization(t *testing.T) {
	tests := []struct {
		name string
		d    *UsageDetail
		want int
	}{
		{
			name: "no usage",
			d:    &UsageDetail{Limit: "100", Remaining: "100"},
			want: 0,
		},
		{
			name: "half used",
			d:    &UsageDetail{Limit: "100", Remaining: "50"},
			want: 50,
		},
		{
			name: "fully used",
			d:    &UsageDetail{Limit: "100", Remaining: "0"},
			want: 100,
		},
		{
			name: "nil detail",
			d:    nil,
			want: 0,
		},
		{
			name: "zero limit",
			d:    &UsageDetail{Limit: "0", Remaining: "0"},
			want: 0,
		},
		{
			name: "remaining exceeds limit clamped to 0",
			d:    &UsageDetail{Limit: "100", Remaining: "150"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUsageDetail_ResetTimeUTC(t *testing.T) {
	tests := []struct {
		name    string
		d       *UsageDetail
		wantNil bool
	}{
		{
			name:    "valid time",
			d:       &UsageDetail{Limit: "100", Remaining: "100", ResetTime: "2026-02-25T04:01:38Z"},
			wantNil: false,
		},
		{
			name:    "empty time",
			d:       &UsageDetail{Limit: "100", Remaining: "100"},
			wantNil: true,
		},
		{
			name:    "nil detail",
			d:       nil,
			wantNil: true,
		},
		{
			name:    "invalid time",
			d:       &UsageDetail{Limit: "100", Remaining: "100", ResetTime: "not-a-time"},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.ResetTimeUTC()
			if tt.wantNil && got != nil {
				t.Error("expected nil, got non-nil")
			}
			if !tt.wantNil && got == nil {
				t.Error("expected non-nil, got nil")
			}
		})
	}
}

func TestWindow_PeriodType(t *testing.T) {
	tests := []struct {
		name string
		w    Window
		want models.PeriodType
	}{
		{
			name: "5 hour session",
			w:    Window{Duration: 300, TimeUnit: "TIME_UNIT_MINUTE"},
			want: models.PeriodSession,
		},
		{
			name: "24 hour daily",
			w:    Window{Duration: 24, TimeUnit: "TIME_UNIT_HOUR"},
			want: models.PeriodDaily,
		},
		{
			name: "7 day weekly",
			w:    Window{Duration: 7, TimeUnit: "TIME_UNIT_DAY"},
			want: models.PeriodWeekly,
		},
		{
			name: "30 day monthly",
			w:    Window{Duration: 30, TimeUnit: "TIME_UNIT_DAY"},
			want: models.PeriodMonthly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.w.PeriodType()
			if got != tt.want {
				t.Errorf("PeriodType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWindow_DisplayName(t *testing.T) {
	tests := []struct {
		name string
		w    Window
		want string
	}{
		{
			name: "5 hour session",
			w:    Window{Duration: 300, TimeUnit: "TIME_UNIT_MINUTE"},
			want: "5h",
		},
		{
			name: "30 minutes",
			w:    Window{Duration: 30, TimeUnit: "TIME_UNIT_MINUTE"},
			want: "30m",
		},
		{
			name: "7 days",
			w:    Window{Duration: 7, TimeUnit: "TIME_UNIT_DAY"},
			want: "7d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.w.DisplayName()
			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlanName(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"LEVEL_BASIC", "Basic (Free)"},
		{"LEVEL_PRO", "Pro"},
		{"LEVEL_PREMIUM", "Premium"},
		{"LEVEL_UNKNOWN", "LEVEL_UNKNOWN"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			got := PlanName(tt.level)
			if got != tt.want {
				t.Errorf("PlanName(%q) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestDeviceCodeResponse_Unmarshal(t *testing.T) {
	raw := `{
		"user_code": "ABCD-1234",
		"device_code": "dc-123",
		"verification_uri_complete": "https://auth.kimi.com/device?code=ABCD-1234",
		"interval": 5,
		"expires_in": 600
	}`

	var resp DeviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.UserCode != "ABCD-1234" {
		t.Errorf("user_code = %q, want %q", resp.UserCode, "ABCD-1234")
	}
	if resp.DeviceCode != "dc-123" {
		t.Errorf("device_code = %q, want %q", resp.DeviceCode, "dc-123")
	}
	if resp.VerificationURIComplete != "https://auth.kimi.com/device?code=ABCD-1234" {
		t.Errorf("verification_uri_complete = %q", resp.VerificationURIComplete)
	}
	if resp.Interval != 5 {
		t.Errorf("interval = %d, want 5", resp.Interval)
	}
	if resp.ExpiresIn != 600 {
		t.Errorf("expires_in = %d, want 600", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalSuccess(t *testing.T) {
	raw := `{
		"access_token": "eyJhbG...",
		"refresh_token": "rt-xxx",
		"expires_in": 900,
		"scope": "coding",
		"token_type": "bearer"
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "eyJhbG..." {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "eyJhbG...")
	}
	if resp.RefreshToken != "rt-xxx" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "rt-xxx")
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want 900", resp.ExpiresIn)
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
}

func TestOAuthCredentials_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name string
		c    OAuthCredentials
		want bool
	}{
		{
			name: "no expiry",
			c:    OAuthCredentials{AccessToken: "tok"},
			want: false,
		},
		{
			name: "far future",
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: float64(9999999999)},
			want: false,
		},
		{
			name: "already expired",
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: float64(1000000000)},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.NeedsRefresh()
			if got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := OAuthCredentials{
		AccessToken:  "tok",
		RefreshToken: "rt",
		ExpiresAt:    1740000000.0,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded OAuthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", decoded.AccessToken, original.AccessToken)
	}
	if decoded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", decoded.RefreshToken, original.RefreshToken)
	}
	if decoded.ExpiresAt != original.ExpiresAt {
		t.Errorf("ExpiresAt = %v, want %v", decoded.ExpiresAt, original.ExpiresAt)
	}
}
