package openrouter

import (
	"encoding/json"
	"testing"
)

func TestCreditsResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"data": {
			"total_credits": 50.0,
			"total_usage": 12.75
		}
	}`

	var resp CreditsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Data.TotalCredits != 50.0 {
		t.Errorf("total_credits = %f, want 50.0", resp.Data.TotalCredits)
	}
	if resp.Data.TotalUsage != 12.75 {
		t.Errorf("total_usage = %f, want 12.75", resp.Data.TotalUsage)
	}
}

func TestCreditsResponse_UnmarshalZeroBalance(t *testing.T) {
	raw := `{"data": {"total_credits": 0, "total_usage": 0}}`

	var resp CreditsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Data.TotalCredits != 0 {
		t.Errorf("total_credits = %f, want 0", resp.Data.TotalCredits)
	}
	if resp.Data.TotalUsage != 0 {
		t.Errorf("total_usage = %f, want 0", resp.Data.TotalUsage)
	}
}

func TestKeyResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"data": {
			"label": "my-key",
			"is_free_tier": false,
			"limit": 100.0,
			"limit_reset": "monthly",
			"limit_remaining": 75.5,
			"include_byok_in_limit": false,
			"usage": 24.5,
			"usage_daily": 5.0,
			"usage_weekly": 15.0,
			"usage_monthly": 24.5,
			"byok_usage": 0,
			"byok_usage_daily": 0,
			"byok_usage_weekly": 0,
			"byok_usage_monthly": 0,
			"rate_limit": {
				"requests": 200,
				"interval": 10
			}
		}
	}`

	var resp KeyResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	d := resp.Data
	if d.Label != "my-key" {
		t.Errorf("label = %q, want %q", d.Label, "my-key")
	}
	if d.IsFreeTier {
		t.Error("is_free_tier = true, want false")
	}
	if d.Limit == nil || *d.Limit != 100.0 {
		t.Errorf("limit = %v, want 100.0", d.Limit)
	}
	if d.LimitReset == nil || *d.LimitReset != "monthly" {
		t.Errorf("limit_reset = %v, want %q", d.LimitReset, "monthly")
	}
	if d.LimitRemaining == nil || *d.LimitRemaining != 75.5 {
		t.Errorf("limit_remaining = %v, want 75.5", d.LimitRemaining)
	}
	if d.IncludeBYOKInLimit {
		t.Error("include_byok_in_limit = true, want false")
	}
	if d.Usage != 24.5 {
		t.Errorf("usage = %f, want 24.5", d.Usage)
	}
	if d.UsageDaily != 5.0 {
		t.Errorf("usage_daily = %f, want 5.0", d.UsageDaily)
	}
	if d.UsageWeekly != 15.0 {
		t.Errorf("usage_weekly = %f, want 15.0", d.UsageWeekly)
	}
	if d.UsageMonthly != 24.5 {
		t.Errorf("usage_monthly = %f, want 24.5", d.UsageMonthly)
	}
	if d.BYOKUsage != 0 {
		t.Errorf("byok_usage = %f, want 0", d.BYOKUsage)
	}
	if d.BYOKUsageDaily != 0 {
		t.Errorf("byok_usage_daily = %f, want 0", d.BYOKUsageDaily)
	}
	if d.BYOKUsageWeekly != 0 {
		t.Errorf("byok_usage_weekly = %f, want 0", d.BYOKUsageWeekly)
	}
	if d.BYOKUsageMonthly != 0 {
		t.Errorf("byok_usage_monthly = %f, want 0", d.BYOKUsageMonthly)
	}
	if d.RateLimit.Requests != 200 {
		t.Errorf("rate_limit.requests = %d, want 200", d.RateLimit.Requests)
	}
	if d.RateLimit.Interval != 10 {
		t.Errorf("rate_limit.interval = %d, want 10", d.RateLimit.Interval)
	}
}

func TestKeyResponse_UnmarshalFreeTier(t *testing.T) {
	raw := `{
		"data": {
			"label": "",
			"is_free_tier": true,
			"limit": null,
			"limit_reset": null,
			"limit_remaining": null,
			"include_byok_in_limit": false,
			"usage": 0.5,
			"usage_daily": 0.1,
			"usage_weekly": 0.3,
			"usage_monthly": 0.5,
			"byok_usage": 0,
			"byok_usage_daily": 0,
			"byok_usage_weekly": 0,
			"byok_usage_monthly": 0,
			"rate_limit": {
				"requests": 10,
				"interval": 10
			}
		}
	}`

	var resp KeyResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	d := resp.Data
	if !d.IsFreeTier {
		t.Error("is_free_tier = false, want true")
	}
	if d.Limit != nil {
		t.Errorf("limit = %v, want nil", d.Limit)
	}
	if d.LimitReset != nil {
		t.Errorf("limit_reset = %v, want nil", d.LimitReset)
	}
	if d.LimitRemaining != nil {
		t.Errorf("limit_remaining = %v, want nil", d.LimitRemaining)
	}
}
