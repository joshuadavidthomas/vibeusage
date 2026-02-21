package cursor

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUsageSummaryResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"premium_requests": {"used": 42.0, "available": 58.0},
		"billing_cycle": {"end": "2025-03-01T00:00:00Z"},
		"on_demand_spend": {"limit_cents": 5000, "used_cents": 1500}
	}`

	var resp UsageSummaryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.PremiumRequests == nil {
		t.Fatal("expected premium_requests")
	}
	if resp.PremiumRequests.Used != 42.0 {
		t.Errorf("used = %v, want 42.0", resp.PremiumRequests.Used)
	}
	if resp.PremiumRequests.Available != 58.0 {
		t.Errorf("available = %v, want 58.0", resp.PremiumRequests.Available)
	}

	if resp.BillingCycle == nil {
		t.Fatal("expected billing_cycle")
	}
	endTime := resp.BillingCycle.EndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}

	if resp.OnDemandSpend == nil {
		t.Fatal("expected on_demand_spend")
	}
	if resp.OnDemandSpend.LimitCents != 5000 {
		t.Errorf("limit_cents = %v, want 5000", resp.OnDemandSpend.LimitCents)
	}
	if resp.OnDemandSpend.UsedCents != 1500 {
		t.Errorf("used_cents = %v, want 1500", resp.OnDemandSpend.UsedCents)
	}
}

func TestUsageSummaryResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp UsageSummaryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.PremiumRequests != nil {
		t.Error("expected nil premium_requests")
	}
	if resp.BillingCycle != nil {
		t.Error("expected nil billing_cycle")
	}
	if resp.OnDemandSpend != nil {
		t.Error("expected nil on_demand_spend")
	}
}

func TestBillingCycle_EndTime_StringFormat(t *testing.T) {
	raw := `{"end": "2025-03-01T00:00:00Z"}`

	var bc BillingCycle
	if err := json.Unmarshal([]byte(raw), &bc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	endTime := bc.EndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}
}

func TestBillingCycle_EndTime_NumericFormat(t *testing.T) {
	// 1740787200000 = 2025-03-01T00:00:00Z in milliseconds
	raw := `{"end": 1740787200000}`

	var bc BillingCycle
	if err := json.Unmarshal([]byte(raw), &bc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	endTime := bc.EndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}
}

func TestBillingCycle_EndTime_Nil(t *testing.T) {
	bc := BillingCycle{}
	if bc.EndTime() != nil {
		t.Error("expected nil end time for empty billing cycle")
	}
}

func TestBillingCycle_EndTime_NoEndField(t *testing.T) {
	raw := `{}`

	var bc BillingCycle
	if err := json.Unmarshal([]byte(raw), &bc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bc.EndTime() != nil {
		t.Error("expected nil end time when no end field")
	}
}

func TestUserMeResponse_Unmarshal(t *testing.T) {
	raw := `{"email": "user@example.com", "membership_type": "pro"}`

	var resp UserMeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", resp.Email, "user@example.com")
	}
	if resp.MembershipType != "pro" {
		t.Errorf("membership_type = %q, want %q", resp.MembershipType, "pro")
	}
}

func TestUserMeResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp UserMeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Email != "" {
		t.Errorf("email = %q, want empty", resp.Email)
	}
	if resp.MembershipType != "" {
		t.Errorf("membership_type = %q, want empty", resp.MembershipType)
	}
}

func TestSessionCredentials_EffectiveToken(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "session_token",
			raw:  `{"session_token": "tok-1"}`,
			want: "tok-1",
		},
		{
			name: "token",
			raw:  `{"token": "tok-2"}`,
			want: "tok-2",
		},
		{
			name: "session_key",
			raw:  `{"session_key": "tok-3"}`,
			want: "tok-3",
		},
		{
			name: "session",
			raw:  `{"session": "tok-4"}`,
			want: "tok-4",
		},
		{
			name: "preference order",
			raw:  `{"session_token": "first", "token": "second"}`,
			want: "first",
		},
		{
			name: "empty",
			raw:  `{}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var creds SessionCredentials
			if err := json.Unmarshal([]byte(tt.raw), &creds); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			got := creds.EffectiveToken()
			if got != tt.want {
				t.Errorf("EffectiveToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
