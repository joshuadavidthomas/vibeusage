package zai

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestQuotaResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"code": 200,
		"msg": "Operation successful",
		"data": {
			"limits": [
				{
					"type": "TOKENS_LIMIT",
					"unit": 3,
					"number": 5,
					"percentage": 1,
					"nextResetTime": 1771661559241
				},
				{
					"type": "TIME_LIMIT",
					"unit": 5,
					"number": 1,
					"usage": 1000,
					"currentValue": 0,
					"remaining": 1000,
					"percentage": 0,
					"nextResetTime": 1773596236985,
					"usageDetails": [
						{ "modelCode": "search-prime", "usage": 0 },
						{ "modelCode": "web-reader", "usage": 33 },
						{ "modelCode": "zread", "usage": 0 }
					]
				}
			],
			"level": "pro"
		},
		"success": true
	}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Code != 200 {
		t.Errorf("code = %d, want 200", resp.Code)
	}
	if !resp.Success {
		t.Error("expected success = true")
	}
	if resp.Data == nil {
		t.Fatal("expected data")
	}
	if resp.Data.Level != "pro" {
		t.Errorf("level = %q, want %q", resp.Data.Level, "pro")
	}
	if len(resp.Data.Limits) != 2 {
		t.Fatalf("expected 2 limits, got %d", len(resp.Data.Limits))
	}

	tokens := resp.Data.Limits[0]
	if tokens.Type != "TOKENS_LIMIT" {
		t.Errorf("type = %q, want %q", tokens.Type, "TOKENS_LIMIT")
	}
	if tokens.Unit != 3 {
		t.Errorf("unit = %d, want 3", tokens.Unit)
	}
	if tokens.Number != 5 {
		t.Errorf("number = %d, want 5", tokens.Number)
	}
	if tokens.Percentage != 1 {
		t.Errorf("percentage = %d, want 1", tokens.Percentage)
	}
	if tokens.NextResetTime != 1771661559241 {
		t.Errorf("nextResetTime = %d, want 1771661559241", tokens.NextResetTime)
	}

	timeLim := resp.Data.Limits[1]
	if timeLim.Type != "TIME_LIMIT" {
		t.Errorf("type = %q, want %q", timeLim.Type, "TIME_LIMIT")
	}
	if timeLim.Usage != 1000 {
		t.Errorf("usage = %d, want 1000", timeLim.Usage)
	}
	if timeLim.Remaining != 1000 {
		t.Errorf("remaining = %d, want 1000", timeLim.Remaining)
	}
	if len(timeLim.UsageDetails) != 3 {
		t.Fatalf("expected 3 usage details, got %d", len(timeLim.UsageDetails))
	}
	if timeLim.UsageDetails[1].ModelCode != "web-reader" {
		t.Errorf("modelCode = %q, want %q", timeLim.UsageDetails[1].ModelCode, "web-reader")
	}
	if timeLim.UsageDetails[1].Usage != 33 {
		t.Errorf("usage = %d, want 33", timeLim.UsageDetails[1].Usage)
	}
}

func TestQuotaResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{"code": 200, "msg": "ok", "success": true}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Data != nil {
		t.Error("expected nil data")
	}
}

func TestQuotaResponse_UnmarshalAuthError(t *testing.T) {
	raw := `{"code":1001,"msg":"Authentication parameter not received in Header","success":false}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Code != 1001 {
		t.Errorf("code = %d, want 1001", resp.Code)
	}
	if resp.Success {
		t.Error("expected success = false")
	}
}

func TestQuotaLimit_PeriodType(t *testing.T) {
	tests := []struct {
		name string
		q    QuotaLimit
		want models.PeriodType
	}{
		{
			name: "5 hour token quota",
			q:    QuotaLimit{Unit: 3, Number: 5},
			want: models.PeriodSession,
		},
		{
			name: "1 month MCP",
			q:    QuotaLimit{Unit: 5, Number: 1},
			want: models.PeriodMonthly,
		},
		{
			name: "24 hour window",
			q:    QuotaLimit{Unit: 3, Number: 24},
			want: models.PeriodDaily,
		},
		{
			name: "unknown unit defaults monthly",
			q:    QuotaLimit{Unit: 99, Number: 1},
			want: models.PeriodMonthly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.PeriodType()
			if got != tt.want {
				t.Errorf("PeriodType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuotaLimit_ResetTime(t *testing.T) {
	tests := []struct {
		name    string
		millis  int64
		wantNil bool
		wantSec int64
	}{
		{
			name:    "valid timestamp",
			millis:  1771661559241,
			wantNil: false,
			wantSec: 1771661559,
		},
		{
			name:    "zero timestamp",
			millis:  0,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := QuotaLimit{NextResetTime: tt.millis}
			got := q.ResetTime()
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

func TestQuotaLimit_DisplayName(t *testing.T) {
	tests := []struct {
		name string
		q    QuotaLimit
		want string
	}{
		// TOKENS_LIMIT: hourly windows use "Session (Nh)" to match Claude/Antigravity.
		// Other periods fall back to a labelled quota.
		{
			name: "5 hour token window",
			q:    QuotaLimit{Type: "TOKENS_LIMIT", Unit: unitHours, Number: 5},
			want: "Session (5h)",
		},
		{
			name: "1 hour token window",
			q:    QuotaLimit{Type: "TOKENS_LIMIT", Unit: unitHours, Number: 1},
			want: "Session (1h)",
		},
		{
			name: "24 hour token window",
			q:    QuotaLimit{Type: "TOKENS_LIMIT", Unit: unitHours, Number: 24},
			want: "Session (24h)",
		},
		{
			name: "monthly token window",
			q:    QuotaLimit{Type: "TOKENS_LIMIT", Unit: unitMonths, Number: 1},
			want: "Monthly Quota",
		},
		// TIME_LIMIT: monthly web-tool quota (web search, reader, zread)
		{
			name: "monthly tools",
			q:    QuotaLimit{Type: "TIME_LIMIT", Unit: unitMonths, Number: 1},
			want: "Monthly Tools",
		},
		{
			name: "daily tools",
			q:    QuotaLimit{Type: "TIME_LIMIT", Unit: unitDays, Number: 1},
			want: "Daily Tools",
		},
		// unknown type falls back to the raw type string
		{
			name: "unknown type",
			q:    QuotaLimit{Type: "OTHER"},
			want: "OTHER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.DisplayName()
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
		{"lite", "Lite"},
		{"pro", "Pro"},
		{"max", "Max"},
		{"enterprise", "enterprise"},
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

func TestParseQuotaResponse_ProTier(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Msg:     "Operation successful",
		Success: true,
		Data: &QuotaData{
			Level: "pro",
			Limits: []QuotaLimit{
				{
					Type:          "TOKENS_LIMIT",
					Unit:          3,
					Number:        5,
					Percentage:    42,
					NextResetTime: 1771661559241,
				},
				{
					Type:          "TIME_LIMIT",
					Unit:          5,
					Number:        1,
					Usage:         1000,
					CurrentValue:  200,
					Remaining:     800,
					Percentage:    20,
					NextResetTime: 1773596236985,
				},
			},
		},
	}

	snapshot := parseQuotaResponse(resp)
	if snapshot == nil {
		t.Fatal("expected snapshot, got nil")
	}

	if snapshot.Provider != "zai" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "zai")
	}
	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Pro")
	}

	if len(snapshot.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(snapshot.Periods))
	}

	tokens := snapshot.Periods[0]
	if tokens.Name != "Session (5h)" {
		t.Errorf("name = %q, want %q", tokens.Name, "Session (5h)")
	}
	if tokens.Utilization != 42 {
		t.Errorf("utilization = %d, want 42", tokens.Utilization)
	}
	if tokens.PeriodType != models.PeriodSession {
		t.Errorf("periodType = %q, want %q", tokens.PeriodType, models.PeriodSession)
	}
	if tokens.ResetsAt == nil {
		t.Fatal("expected resetsAt")
	}
	// Verify milliseconds were correctly converted
	if tokens.ResetsAt.Unix() != 1771661559 {
		t.Errorf("resetsAt unix = %d, want 1771661559", tokens.ResetsAt.Unix())
	}

	tools := snapshot.Periods[1]
	if tools.Name != "Monthly Tools" {
		t.Errorf("name = %q, want %q", tools.Name, "Monthly Tools")
	}
	if tools.Utilization != 20 {
		t.Errorf("utilization = %d, want 20", tools.Utilization)
	}
	if tools.PeriodType != models.PeriodMonthly {
		t.Errorf("periodType = %q, want %q", tools.PeriodType, models.PeriodMonthly)
	}
}

func TestParseQuotaResponse_EmptyData(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Success: true,
		Data:    &QuotaData{Limits: []QuotaLimit{}},
	}

	snapshot := parseQuotaResponse(resp)
	if snapshot != nil {
		t.Error("expected nil snapshot for empty limits")
	}
}

func TestParseQuotaResponse_NilData(t *testing.T) {
	resp := QuotaResponse{Code: 200, Success: true}

	snapshot := parseQuotaResponse(resp)
	if snapshot != nil {
		t.Error("expected nil snapshot for nil data")
	}
}

func TestParseQuotaResponse_UtilizationClamped(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Success: true,
		Data: &QuotaData{
			Limits: []QuotaLimit{
				{Type: "TOKENS_LIMIT", Unit: 3, Number: 5, Percentage: 150},
			},
		},
	}

	snapshot := parseQuotaResponse(resp)
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.Periods[0].Utilization != 100 {
		t.Errorf("utilization = %d, want 100 (clamped)", snapshot.Periods[0].Utilization)
	}
}

func TestParseQuotaResponse_FetchedAtIsRecent(t *testing.T) {
	resp := QuotaResponse{
		Code:    200,
		Success: true,
		Data: &QuotaData{
			Limits: []QuotaLimit{
				{Type: "TOKENS_LIMIT", Unit: 3, Number: 5, Percentage: 0, NextResetTime: 1771661559241},
			},
		},
	}

	before := time.Now().UTC()
	snapshot := parseQuotaResponse(resp)
	after := time.Now().UTC()

	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.FetchedAt.Before(before) || snapshot.FetchedAt.After(after) {
		t.Errorf("fetchedAt = %v, expected between %v and %v", snapshot.FetchedAt, before, after)
	}
}
