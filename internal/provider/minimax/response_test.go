package minimax

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestCodingPlanResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"model_remains": [
			{
				"start_time": 1771650000000,
				"end_time": 1771668000000,
				"remains_time": 8196068,
				"current_interval_total_count": 1500,
				"current_interval_usage_count": 1500,
				"model_name": "MiniMax-M2"
			},
			{
				"start_time": 1771650000000,
				"end_time": 1771668000000,
				"remains_time": 8196068,
				"current_interval_total_count": 1500,
				"current_interval_usage_count": 1500,
				"model_name": "MiniMax-M2.1"
			},
			{
				"start_time": 1771650000000,
				"end_time": 1771668000000,
				"remains_time": 8196068,
				"current_interval_total_count": 1500,
				"current_interval_usage_count": 1500,
				"model_name": "MiniMax-M2.5"
			}
		],
		"base_resp": {
			"status_code": 0,
			"status_msg": "success"
		}
	}`

	var resp CodingPlanResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.BaseResp.StatusCode != 0 {
		t.Errorf("status_code = %d, want 0", resp.BaseResp.StatusCode)
	}
	if resp.BaseResp.StatusMsg != "success" {
		t.Errorf("status_msg = %q, want %q", resp.BaseResp.StatusMsg, "success")
	}
	if len(resp.ModelRemains) != 3 {
		t.Fatalf("expected 3 model_remains, got %d", len(resp.ModelRemains))
	}

	m := resp.ModelRemains[0]
	if m.ModelName != "MiniMax-M2" {
		t.Errorf("model_name = %q, want %q", m.ModelName, "MiniMax-M2")
	}
	if m.StartTime != 1771650000000 {
		t.Errorf("start_time = %d, want 1771650000000", m.StartTime)
	}
	if m.EndTime != 1771668000000 {
		t.Errorf("end_time = %d, want 1771668000000", m.EndTime)
	}
	if m.RemainsTime != 8196068 {
		t.Errorf("remains_time = %d, want 8196068", m.RemainsTime)
	}
	if m.CurrentIntervalTotalCount != 1500 {
		t.Errorf("total_count = %d, want 1500", m.CurrentIntervalTotalCount)
	}
	if m.CurrentIntervalUsageCount != 1500 {
		t.Errorf("usage_count = %d, want 1500", m.CurrentIntervalUsageCount)
	}
}

func TestCodingPlanResponse_UnmarshalErrorResponse(t *testing.T) {
	raw := `{"status_code":1004,"status_msg":"cookie is missing, log in again"}`

	// Error responses from Minimax use a flat structure with status_code at the
	// top level (not nested under base_resp).
	var resp CodingPlanResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.ModelRemains) != 0 {
		t.Errorf("expected 0 model_remains, got %d", len(resp.ModelRemains))
	}
	if resp.StatusCode != 1004 {
		t.Errorf("status_code = %d, want 1004", resp.StatusCode)
	}
	if resp.StatusMsg != "cookie is missing, log in again" {
		t.Errorf("status_msg = %q, want %q", resp.StatusMsg, "cookie is missing, log in again")
	}
	// base_resp should be zero-value since it's not in the response
	if resp.BaseResp.StatusCode != 0 {
		t.Errorf("base_resp.status_code = %d, want 0", resp.BaseResp.StatusCode)
	}
}

func TestModelRemain_Utilization(t *testing.T) {
	// The endpoint is /coding_plan/remains — current_interval_usage_count is the
	// REMAINING quota, not the consumed count.
	// utilization = (total - remaining) / total * 100
	tests := []struct {
		name string
		m    ModelRemain
		want int
	}{
		{
			name: "fresh quota (all remaining)",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 1500},
			want: 0,
		},
		{
			name: "half remaining",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 750},
			want: 50,
		},
		{
			name: "fully exhausted",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 0},
			want: 100,
		},
		{
			name: "remaining exceeds total (clamp to 0)",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 2000},
			want: 0,
		},
		{
			name: "zero total zero remaining",
			m:    ModelRemain{CurrentIntervalTotalCount: 0, CurrentIntervalUsageCount: 0},
			want: 0,
		},
		{
			name: "starter tier quarter remaining",
			m:    ModelRemain{CurrentIntervalTotalCount: 500, CurrentIntervalUsageCount: 100},
			want: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestModelRemain_ResetTime(t *testing.T) {
	tests := []struct {
		name    string
		endTime int64
		wantNil bool
		wantSec int64
	}{
		{
			name:    "valid timestamp",
			endTime: 1771668000000,
			wantNil: false,
			wantSec: 1771668000,
		},
		{
			name:    "zero timestamp",
			endTime: 0,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ModelRemain{EndTime: tt.endTime}
			got := m.ResetTime()
			if tt.wantNil {
				if got != nil {
					t.Error("expected nil, got non-nil")
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil, got nil")
			}
			if got.Unix() != tt.wantSec {
				t.Errorf("Unix() = %d, want %d", got.Unix(), tt.wantSec)
			}
		})
	}
}

func TestModelRemain_Remaining(t *testing.T) {
	// current_interval_usage_count IS the remaining count (endpoint: /remains).
	tests := []struct {
		name string
		m    ModelRemain
		want int
	}{
		{
			name: "1000 remaining",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 1000},
			want: 1000,
		},
		{
			name: "fully exhausted",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 0},
			want: 0,
		},
		{
			name: "fresh quota",
			m:    ModelRemain{CurrentIntervalTotalCount: 1500, CurrentIntervalUsageCount: 1500},
			want: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.Remaining()
			if got != tt.want {
				t.Errorf("Remaining() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestModelRemain_ToUsagePeriod(t *testing.T) {
	m := ModelRemain{
		StartTime:                 1771650000000,
		EndTime:                   1771668000000,
		CurrentIntervalTotalCount: 1500,
		CurrentIntervalUsageCount: 750,
		ModelName:                 "MiniMax-M2.5",
	}

	period := m.ToUsagePeriod()

	if period.Name != "MiniMax-M2.5" {
		t.Errorf("name = %q, want %q", period.Name, "MiniMax-M2.5")
	}
	if period.Utilization != 50 {
		t.Errorf("utilization = %d, want 50", period.Utilization)
	}
	if period.PeriodType != models.PeriodSession {
		t.Errorf("periodType = %q, want %q", period.PeriodType, models.PeriodSession)
	}
	if period.Model != "MiniMax-M2.5" {
		t.Errorf("model = %q, want %q", period.Model, "MiniMax-M2.5")
	}
	if period.ResetsAt == nil {
		t.Fatal("expected resetsAt")
	}
	if period.ResetsAt.Unix() != 1771668000 {
		t.Errorf("resetsAt unix = %d, want 1771668000", period.ResetsAt.Unix())
	}
}

func TestInferPlan(t *testing.T) {
	tests := []struct {
		total int
		want  string
	}{
		{500, "Starter"},
		{100, "Starter"},
		{1500, "Plus"},
		{1000, "Plus"},
		{5000, "Max"},
		{3000, "Max"},
		{10000, ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := InferPlan(tt.total)
			if got != tt.want {
				t.Errorf("InferPlan(%d) = %q, want %q", tt.total, got, tt.want)
			}
		})
	}
}

func TestParseResponse_MultiModel(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{
			{
				StartTime:                 1771650000000,
				EndTime:                   1771668000000,
				RemainsTime:               8196068,
				CurrentIntervalTotalCount: 1500,
				CurrentIntervalUsageCount: 750,
				ModelName:                 "MiniMax-M2",
			},
			{
				StartTime:                 1771650000000,
				EndTime:                   1771668000000,
				RemainsTime:               8196068,
				CurrentIntervalTotalCount: 1500,
				CurrentIntervalUsageCount: 1500,
				ModelName:                 "MiniMax-M2.5",
			},
		},
		BaseResp: BaseResp{StatusCode: 0, StatusMsg: "success"},
	}

	snapshot := parseResponse(resp)
	if snapshot == nil {
		t.Fatal("expected snapshot, got nil")
	}

	if snapshot.Provider != "minimax" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "minimax")
	}

	// Summary + 2 model periods
	if len(snapshot.Periods) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(snapshot.Periods))
	}

	// Summary takes highest utilization across models.
	// M2: 750 remaining / 1500 total → 50% used.
	// M2.5: 1500 remaining / 1500 total → 0% used (fresh quota).
	// Highest = 50%.
	summary := snapshot.Periods[0]
	if summary.Name != "Coding Plan" {
		t.Errorf("summary name = %q, want %q", summary.Name, "Coding Plan")
	}
	if summary.Utilization != 50 {
		t.Errorf("summary utilization = %d, want 50", summary.Utilization)
	}
	if summary.PeriodType != models.PeriodSession {
		t.Errorf("summary periodType = %q, want %q", summary.PeriodType, models.PeriodSession)
	}

	// Per-model periods
	m1 := snapshot.Periods[1]
	if m1.Name != "MiniMax-M2" {
		t.Errorf("m1 name = %q, want %q", m1.Name, "MiniMax-M2")
	}
	if m1.Utilization != 50 {
		t.Errorf("m1 utilization = %d, want 50", m1.Utilization)
	}

	m2 := snapshot.Periods[2]
	if m2.Name != "MiniMax-M2.5" {
		t.Errorf("m2 name = %q, want %q", m2.Name, "MiniMax-M2.5")
	}
	if m2.Utilization != 0 {
		t.Errorf("m2 utilization = %d, want 0 (fresh quota: 1500 remaining / 1500 total)", m2.Utilization)
	}

	// Identity
	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Plus" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Plus")
	}
}

func TestParseResponse_SingleModel(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{
			{
				StartTime:                 1771650000000,
				EndTime:                   1771668000000,
				CurrentIntervalTotalCount: 500,
				CurrentIntervalUsageCount: 100,
				ModelName:                 "MiniMax-M2.5",
			},
		},
		BaseResp: BaseResp{StatusCode: 0, StatusMsg: "success"},
	}

	snapshot := parseResponse(resp)
	if snapshot == nil {
		t.Fatal("expected snapshot, got nil")
	}

	// Single model: just one period named "Coding Plan"
	if len(snapshot.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(snapshot.Periods))
	}

	period := snapshot.Periods[0]
	if period.Name != "Coding Plan" {
		t.Errorf("name = %q, want %q", period.Name, "Coding Plan")
	}
	if period.Utilization != 80 {
		// 100 remaining / 500 total → 400 used → 80%
		t.Errorf("utilization = %d, want 80", period.Utilization)
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Starter" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Starter")
	}
}

func TestParseResponse_Empty(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{},
		BaseResp:     BaseResp{StatusCode: 0, StatusMsg: "success"},
	}

	snapshot := parseResponse(resp)
	if snapshot != nil {
		t.Error("expected nil snapshot for empty model_remains")
	}
}

func TestParseResponse_FetchedAtIsRecent(t *testing.T) {
	resp := CodingPlanResponse{
		ModelRemains: []ModelRemain{
			{
				EndTime:                   1771668000000,
				CurrentIntervalTotalCount: 1500,
				CurrentIntervalUsageCount: 0,
				ModelName:                 "MiniMax-M2.5",
			},
		},
		BaseResp: BaseResp{StatusCode: 0, StatusMsg: "success"},
	}

	before := time.Now().UTC()
	snapshot := parseResponse(resp)
	after := time.Now().UTC()

	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.FetchedAt.Before(before) || snapshot.FetchedAt.After(after) {
		t.Errorf("fetchedAt = %v, expected between %v and %v", snapshot.FetchedAt, before, after)
	}
}
