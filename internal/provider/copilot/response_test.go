package copilot

import (
	"encoding/json"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
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
