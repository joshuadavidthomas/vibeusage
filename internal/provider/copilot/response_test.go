package copilot

import (
	"encoding/json"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
)

func TestUserResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"login": "testuser",
		"access_type_sku": "free_engaged_oss_quota",
		"analytics_tracking_id": "abc123",
		"assigned_date": "2022-06-21T14:59:45-05:00",
		"can_signup_for_limited": false,
		"chat_enabled": true,
		"copilotignore_enabled": false,
		"copilot_plan": "pro",
		"is_mcp_enabled": true,
		"organization_login_list": [],
		"organization_list": [],
		"restricted_telemetry": true,
		"endpoints": {
			"api": "https://api.individual.githubcopilot.com",
			"origin-tracker": "https://origin-tracker.individual.githubcopilot.com",
			"proxy": "https://proxy.individual.githubcopilot.com",
			"telemetry": "https://telemetry.individual.githubcopilot.com"
		},
		"quota_reset_date": "2025-03-01",
		"quota_snapshots": {
			"premium_interactions": {
				"entitlement": 100,
				"overage_count": 0,
				"overage_permitted": false,
				"percent_remaining": 60.0,
				"quota_id": "premium_interactions",
				"quota_remaining": 60.0,
				"remaining": 60,
				"unlimited": false,
				"timestamp_utc": "2025-02-20T12:00:00.000Z"
			},
			"chat": {
				"entitlement": 500,
				"overage_count": 0,
				"overage_permitted": false,
				"percent_remaining": 60.0,
				"quota_id": "chat",
				"quota_remaining": 300.0,
				"remaining": 300,
				"unlimited": false,
				"timestamp_utc": "2025-02-20T12:00:00.000Z"
			},
			"completions": {
				"entitlement": 0,
				"overage_count": 0,
				"overage_permitted": false,
				"percent_remaining": 100.0,
				"quota_id": "completions",
				"quota_remaining": 0.0,
				"remaining": 0,
				"unlimited": true,
				"timestamp_utc": "2025-02-20T12:00:00.000Z"
			}
		},
		"quota_reset_date_utc": "2025-03-01T00:00:00Z"
	}`

	var resp UserResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Top-level fields
	if resp.Login != "testuser" {
		t.Errorf("login = %q, want %q", resp.Login, "testuser")
	}
	if resp.AccessTypeSku != "free_engaged_oss_quota" {
		t.Errorf("access_type_sku = %q, want %q", resp.AccessTypeSku, "free_engaged_oss_quota")
	}
	if resp.AnalyticsTrackingID != "abc123" {
		t.Errorf("analytics_tracking_id = %q, want %q", resp.AnalyticsTrackingID, "abc123")
	}
	if resp.AssignedDate != "2022-06-21T14:59:45-05:00" {
		t.Errorf("assigned_date = %q, want %q", resp.AssignedDate, "2022-06-21T14:59:45-05:00")
	}
	if resp.CanSignupForLimited {
		t.Error("expected can_signup_for_limited to be false")
	}
	if !resp.ChatEnabled {
		t.Error("expected chat_enabled to be true")
	}
	if resp.CopilotignoreEnabled {
		t.Error("expected copilotignore_enabled to be false")
	}
	if !resp.IsMcpEnabled {
		t.Error("expected is_mcp_enabled to be true")
	}
	if !resp.RestrictedTelemetry {
		t.Error("expected restricted_telemetry to be true")
	}
	if resp.QuotaResetDateUTC != "2025-03-01T00:00:00Z" {
		t.Errorf("quota_reset_date_utc = %q, want %q", resp.QuotaResetDateUTC, "2025-03-01T00:00:00Z")
	}
	if resp.QuotaResetDate != "2025-03-01" {
		t.Errorf("quota_reset_date = %q, want %q", resp.QuotaResetDate, "2025-03-01")
	}
	if resp.CopilotPlan != "pro" {
		t.Errorf("copilot_plan = %q, want %q", resp.CopilotPlan, "pro")
	}

	// Endpoints
	if resp.Endpoints == nil {
		t.Fatal("expected endpoints")
	}
	if resp.Endpoints.API != "https://api.individual.githubcopilot.com" {
		t.Errorf("endpoints.api = %q, want %q", resp.Endpoints.API, "https://api.individual.githubcopilot.com")
	}
	if resp.Endpoints.OriginTracker != "https://origin-tracker.individual.githubcopilot.com" {
		t.Errorf("endpoints.origin-tracker = %q, want %q", resp.Endpoints.OriginTracker, "https://origin-tracker.individual.githubcopilot.com")
	}
	if resp.Endpoints.Proxy != "https://proxy.individual.githubcopilot.com" {
		t.Errorf("endpoints.proxy = %q, want %q", resp.Endpoints.Proxy, "https://proxy.individual.githubcopilot.com")
	}
	if resp.Endpoints.Telemetry != "https://telemetry.individual.githubcopilot.com" {
		t.Errorf("endpoints.telemetry = %q, want %q", resp.Endpoints.Telemetry, "https://telemetry.individual.githubcopilot.com")
	}

	// Quota snapshots
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
	if resp.QuotaSnapshots.PremiumInteractions.OverageCount != 0 {
		t.Errorf("premium overage_count = %d, want 0", resp.QuotaSnapshots.PremiumInteractions.OverageCount)
	}
	if resp.QuotaSnapshots.PremiumInteractions.OveragePermitted {
		t.Error("expected premium overage_permitted to be false")
	}
	if resp.QuotaSnapshots.PremiumInteractions.PercentRemaining != 60.0 {
		t.Errorf("premium percent_remaining = %v, want 60.0", resp.QuotaSnapshots.PremiumInteractions.PercentRemaining)
	}
	if resp.QuotaSnapshots.PremiumInteractions.QuotaID != "premium_interactions" {
		t.Errorf("premium quota_id = %q, want %q", resp.QuotaSnapshots.PremiumInteractions.QuotaID, "premium_interactions")
	}
	if resp.QuotaSnapshots.PremiumInteractions.QuotaRemaining != 60.0 {
		t.Errorf("premium quota_remaining = %v, want 60.0", resp.QuotaSnapshots.PremiumInteractions.QuotaRemaining)
	}
	if resp.QuotaSnapshots.PremiumInteractions.TimestampUTC != "2025-02-20T12:00:00.000Z" {
		t.Errorf("premium timestamp_utc = %q, want %q", resp.QuotaSnapshots.PremiumInteractions.TimestampUTC, "2025-02-20T12:00:00.000Z")
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

func TestOAuthCredentials_UnmarshalLegacy(t *testing.T) {
	// Legacy format: only access_token, no refresh or expiry
	raw := `{"access_token": "gho_xxxx"}`

	var creds oauth.Credentials
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

	var creds oauth.Credentials
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
	original := oauth.Credentials{
		AccessToken:  "ghu_xxxx",
		RefreshToken: "ghr_xxxx",
		ExpiresAt:    "2025-02-20T06:00:00Z",
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
