package cursor

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUsageSummaryResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"billingCycleStart": "2026-02-14T21:47:25.853Z",
		"billingCycleEnd": "2026-03-14T21:47:25.853Z",
		"membershipType": "pro",
		"limitType": "user",
		"isUnlimited": false,
		"autoModelSelectedDisplayMessage": "You've used 46% of your included total usage",
		"namedModelSelectedDisplayMessage": "You've used 46% of your included API usage",
		"individualUsage": {
			"plan": {
				"enabled": true,
				"used": 2322,
				"limit": 5000,
				"remaining": 2678,
				"breakdown": {
					"included": 2322,
					"bonus": 0,
					"total": 2322
				},
				"autoPercentUsed": 0,
				"apiPercentUsed": 46.44,
				"totalPercentUsed": 46.44
			},
			"onDemand": {
				"enabled": true,
				"used": 1500,
				"limit": 10000,
				"remaining": 8500
			}
		},
		"teamUsage": {
			"onDemand": {
				"enabled": true,
				"used": 3000,
				"limit": 50000,
				"remaining": 47000
			}
		}
	}`

	var resp UsageSummaryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.BillingCycleStart != "2026-02-14T21:47:25.853Z" {
		t.Errorf("billingCycleStart = %q, want %q", resp.BillingCycleStart, "2026-02-14T21:47:25.853Z")
	}
	if resp.BillingCycleEnd != "2026-03-14T21:47:25.853Z" {
		t.Errorf("billingCycleEnd = %q, want %q", resp.BillingCycleEnd, "2026-03-14T21:47:25.853Z")
	}
	if resp.MembershipType != "pro" {
		t.Errorf("membershipType = %q, want %q", resp.MembershipType, "pro")
	}
	if resp.LimitType != "user" {
		t.Errorf("limitType = %q, want %q", resp.LimitType, "user")
	}
	if resp.IsUnlimited == nil || *resp.IsUnlimited != false {
		t.Errorf("isUnlimited = %v, want false", resp.IsUnlimited)
	}
	if resp.AutoModelSelectedDisplayMessage != "You've used 46% of your included total usage" {
		t.Errorf("autoModelSelectedDisplayMessage = %q", resp.AutoModelSelectedDisplayMessage)
	}
	if resp.NamedModelSelectedDisplayMessage != "You've used 46% of your included API usage" {
		t.Errorf("namedModelSelectedDisplayMessage = %q", resp.NamedModelSelectedDisplayMessage)
	}

	if resp.IndividualUsage == nil {
		t.Fatal("expected individualUsage")
	}
	plan := resp.IndividualUsage.Plan
	if plan == nil {
		t.Fatal("expected individualUsage.plan")
	}
	if plan.Enabled == nil || *plan.Enabled != true {
		t.Errorf("plan.enabled = %v, want true", plan.Enabled)
	}
	if plan.Used != 2322 {
		t.Errorf("plan.used = %v, want 2322", plan.Used)
	}
	if plan.Limit != 5000 {
		t.Errorf("plan.limit = %v, want 5000", plan.Limit)
	}
	if plan.Remaining != 2678 {
		t.Errorf("plan.remaining = %v, want 2678", plan.Remaining)
	}
	if plan.Breakdown == nil {
		t.Fatal("expected plan.breakdown")
	}
	if plan.Breakdown.Included != 2322 {
		t.Errorf("breakdown.included = %v, want 2322", plan.Breakdown.Included)
	}
	if plan.Breakdown.Bonus != 0 {
		t.Errorf("breakdown.bonus = %v, want 0", plan.Breakdown.Bonus)
	}
	if plan.Breakdown.Total != 2322 {
		t.Errorf("breakdown.total = %v, want 2322", plan.Breakdown.Total)
	}
	if plan.AutoPercentUsed != 0 {
		t.Errorf("plan.autoPercentUsed = %v, want 0", plan.AutoPercentUsed)
	}
	if plan.APIPercentUsed != 46.44 {
		t.Errorf("plan.apiPercentUsed = %v, want 46.44", plan.APIPercentUsed)
	}
	if plan.TotalPercentUsed != 46.44 {
		t.Errorf("plan.totalPercentUsed = %v, want 46.44", plan.TotalPercentUsed)
	}

	od := resp.IndividualUsage.OnDemand
	if od == nil {
		t.Fatal("expected individualUsage.onDemand")
	}
	if od.Enabled == nil || *od.Enabled != true {
		t.Errorf("onDemand.enabled = %v, want true", od.Enabled)
	}
	if od.Used != 1500 {
		t.Errorf("onDemand.used = %v, want 1500", od.Used)
	}
	if od.Limit == nil || *od.Limit != 10000 {
		t.Errorf("onDemand.limit = %v, want 10000", od.Limit)
	}
	if od.Remaining == nil || *od.Remaining != 8500 {
		t.Errorf("onDemand.remaining = %v, want 8500", od.Remaining)
	}

	if resp.TeamUsage == nil {
		t.Fatal("expected teamUsage")
	}
	teamOD := resp.TeamUsage.OnDemand
	if teamOD == nil {
		t.Fatal("expected teamUsage.onDemand")
	}
	if teamOD.Used != 3000 {
		t.Errorf("teamUsage.onDemand.used = %v, want 3000", teamOD.Used)
	}
	if teamOD.Limit == nil || *teamOD.Limit != 50000 {
		t.Errorf("teamUsage.onDemand.limit = %v, want 50000", teamOD.Limit)
	}
}

func TestUsageSummaryResponse_UnmarshalLiveResponse(t *testing.T) {
	// Captured from live API on 2026-02-27 (free account)
	raw := `{
		"billingCycleStart": "2026-02-14T21:47:25.853Z",
		"billingCycleEnd": "2026-03-14T21:47:25.853Z",
		"membershipType": "free",
		"limitType": "user",
		"isUnlimited": false,
		"autoModelSelectedDisplayMessage": "You've used 0% of your included total usage",
		"namedModelSelectedDisplayMessage": "You've used 0% of your included API usage",
		"individualUsage": {
			"plan": {
				"enabled": true,
				"used": 0,
				"limit": 0,
				"remaining": 0,
				"breakdown": {
					"included": 0,
					"bonus": 0,
					"total": 0
				},
				"autoPercentUsed": 0,
				"apiPercentUsed": 0,
				"totalPercentUsed": 0
			},
			"onDemand": {
				"enabled": false,
				"used": 0,
				"limit": null,
				"remaining": null
			}
		},
		"teamUsage": {}
	}`

	var resp UsageSummaryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.MembershipType != "free" {
		t.Errorf("membershipType = %q, want %q", resp.MembershipType, "free")
	}

	plan := resp.IndividualUsage.Plan
	if plan.Used != 0 {
		t.Errorf("plan.used = %v, want 0", plan.Used)
	}
	if plan.Limit != 0 {
		t.Errorf("plan.limit = %v, want 0", plan.Limit)
	}

	od := resp.IndividualUsage.OnDemand
	if od.Enabled == nil || *od.Enabled != false {
		t.Errorf("onDemand.enabled = %v, want false", od.Enabled)
	}
	if od.Limit != nil {
		t.Errorf("onDemand.limit = %v, want nil", od.Limit)
	}
	if od.Remaining != nil {
		t.Errorf("onDemand.remaining = %v, want nil", od.Remaining)
	}

	if resp.TeamUsage == nil {
		t.Fatal("expected teamUsage (empty object)")
	}
	if resp.TeamUsage.OnDemand != nil {
		t.Error("expected nil teamUsage.onDemand for empty object")
	}
}

func TestUsageSummaryResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp UsageSummaryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.IndividualUsage != nil {
		t.Error("expected nil individualUsage")
	}
	if resp.TeamUsage != nil {
		t.Error("expected nil teamUsage")
	}
	if resp.MembershipType != "" {
		t.Error("expected empty membershipType")
	}
}

func TestBillingCycleEndTime_ISO8601(t *testing.T) {
	resp := UsageSummaryResponse{
		BillingCycleEnd: "2026-03-14T21:47:25.853Z",
	}

	endTime := resp.BillingCycleEndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2026, 3, 14, 21, 47, 25, 853000000, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}
}

func TestBillingCycleEndTime_RFC3339(t *testing.T) {
	resp := UsageSummaryResponse{
		BillingCycleEnd: "2025-03-01T00:00:00Z",
	}

	endTime := resp.BillingCycleEndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}
}

func TestBillingCycleEndTime_UnixMsString(t *testing.T) {
	// Connect RPC format: Unix milliseconds as string
	resp := UsageSummaryResponse{
		BillingCycleEnd: "1740787200000",
	}

	endTime := resp.BillingCycleEndTime()
	if endTime == nil {
		t.Fatal("expected end time")
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !endTime.Equal(expected) {
		t.Errorf("end = %v, want %v", endTime, expected)
	}
}

func TestBillingCycleStartTime(t *testing.T) {
	resp := UsageSummaryResponse{
		BillingCycleStart: "2026-02-14T21:47:25.853Z",
	}

	startTime := resp.BillingCycleStartTime()
	if startTime == nil {
		t.Fatal("expected start time")
	}
	expected := time.Date(2026, 2, 14, 21, 47, 25, 853000000, time.UTC)
	if !startTime.Equal(expected) {
		t.Errorf("start = %v, want %v", startTime, expected)
	}
}

func TestBillingCycleEndTime_Empty(t *testing.T) {
	resp := UsageSummaryResponse{}
	if resp.BillingCycleEndTime() != nil {
		t.Error("expected nil end time for empty response")
	}
}

func TestUserMeResponse_Unmarshal(t *testing.T) {
	raw := `{
		"email": "user@example.com",
		"email_verified": true,
		"name": "Test User",
		"sub": "github|12345",
		"created_at": "2024-01-01T00:00:00.000Z",
		"updated_at": "2026-02-27T00:00:00.000Z",
		"picture": "https://avatars.githubusercontent.com/u/12345"
	}`

	var resp UserMeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", resp.Email, "user@example.com")
	}
	if resp.EmailVerified == nil || *resp.EmailVerified != true {
		t.Errorf("email_verified = %v, want true", resp.EmailVerified)
	}
	if resp.Name != "Test User" {
		t.Errorf("name = %q, want %q", resp.Name, "Test User")
	}
	if resp.Sub != "github|12345" {
		t.Errorf("sub = %q, want %q", resp.Sub, "github|12345")
	}
	if resp.CreatedAt != "2024-01-01T00:00:00.000Z" {
		t.Errorf("created_at = %q", resp.CreatedAt)
	}
	if resp.UpdatedAt != "2026-02-27T00:00:00.000Z" {
		t.Errorf("updated_at = %q", resp.UpdatedAt)
	}
	if resp.Picture != "https://avatars.githubusercontent.com/u/12345" {
		t.Errorf("picture = %q", resp.Picture)
	}
}

func TestUserMeResponse_UnmarshalLegacy(t *testing.T) {
	// Legacy format with membership_type
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
	if resp.Name != "" {
		t.Errorf("name = %q, want empty", resp.Name)
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
