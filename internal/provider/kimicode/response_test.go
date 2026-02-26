package kimicode

import (
	"encoding/json"
	"testing"
	"time"

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
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: "2099-01-01T00:00:00Z"},
			want: false,
		},
		{
			name: "already expired",
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: "2020-01-01T00:00:00Z"},
			want: true,
		},
		{
			name: "within buffer",
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339)},
			want: true,
		},
		{
			name: "outside buffer",
			c:    OAuthCredentials{AccessToken: "tok", ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)},
			want: false,
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
		ExpiresAt:    "2025-02-19T21:20:00Z",
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
		t.Errorf("ExpiresAt = %q, want %q", decoded.ExpiresAt, original.ExpiresAt)
	}
}

func TestMigrateCredentials(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantNil bool
		wantAt  string
	}{
		{
			name:   "legacy float64 timestamp",
			data:   `{"access_token":"tok","refresh_token":"rt","expires_at":1740000000}`,
			wantAt: "2025-02-19T21:20:00Z",
		},
		{
			name:    "already RFC3339",
			data:    `{"access_token":"tok","refresh_token":"rt","expires_at":"2025-02-19T21:20:00Z"}`,
			wantNil: true,
		},
		{
			name:    "no access token",
			data:    `{"refresh_token":"rt","expires_at":1740000000}`,
			wantNil: true,
		},
		{
			name:    "no expires_at",
			data:    `{"access_token":"tok","refresh_token":"rt"}`,
			wantNil: true,
		},
		{
			name:    "invalid json",
			data:    `not json`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := migrateCredentials([]byte(tt.data))
			if tt.wantNil {
				if got != nil {
					t.Errorf("migrateCredentials() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("migrateCredentials() = nil, want non-nil")
			}
			if got.ExpiresAt != tt.wantAt {
				t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, tt.wantAt)
			}
			if got.AccessToken != "tok" {
				t.Errorf("AccessToken = %q, want %q", got.AccessToken, "tok")
			}
			if got.RefreshToken != "rt" {
				t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "rt")
			}
		})
	}
}
