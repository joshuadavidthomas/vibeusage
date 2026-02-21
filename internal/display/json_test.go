package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestOutputJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	OutputJSON(&buf, map[string]string{"key": "value"})

	output := buf.String()
	if !strings.Contains(output, `"key"`) {
		t.Errorf("expected key in output, got: %s", output)
	}
	if !strings.Contains(output, `"value"`) {
		t.Errorf("expected value in output, got: %s", output)
	}
}

func TestOutputJSON_PrettyPrints(t *testing.T) {
	var buf bytes.Buffer
	OutputJSON(&buf, map[string]string{"a": "1"})

	output := buf.String()
	if !strings.Contains(output, "  ") {
		t.Errorf("expected indented output, got: %s", output)
	}
}

func TestOutputStatusJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "All systems normal",
		},
	}

	OutputStatusJSON(&buf, statuses)

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Errorf("expected provider in output, got: %s", output)
	}
	if !strings.Contains(output, "operational") {
		t.Errorf("expected level in output, got: %s", output)
	}
}

func TestOutputMultiProviderJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now()
	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: now,
			},
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Errorf("expected provider in output, got: %s", output)
	}
	if !strings.Contains(output, "providers") {
		t.Errorf("expected providers key in output, got: %s", output)
	}
}

func TestOutputMultiProviderJSON_IncludesErrors(t *testing.T) {
	var buf bytes.Buffer

	outcomes := map[string]fetch.FetchOutcome{
		"cursor": {
			ProviderID: "cursor",
			Success:    false,
			Error:      "auth failed",
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	output := buf.String()
	if !strings.Contains(output, "errors") {
		t.Errorf("expected errors key in output, got: %s", output)
	}
	if !strings.Contains(output, "auth failed") {
		t.Errorf("expected error message in output, got: %s", output)
	}
}

// SnapshotToJSON structural tests

func TestSnapshotToJSON_FailedOutcome(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    false,
		Error:      "token expired",
	}

	data := SnapshotToJSON(outcome)

	errMap, ok := data["error"].(map[string]any)
	if !ok {
		t.Fatal("expected 'error' key with map value")
	}
	if errMap["message"] != "token expired" {
		t.Errorf("error.message = %v, want %q", errMap["message"], "token expired")
	}
	if errMap["provider"] != "claude" {
		t.Errorf("error.provider = %v, want %q", errMap["provider"], "claude")
	}
}

func TestSnapshotToJSON_NilSnapshot(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot:   nil,
	}

	data := SnapshotToJSON(outcome)
	if _, ok := data["error"]; !ok {
		t.Error("expected error map when snapshot is nil")
	}
}

func TestSnapshotToJSON_SuccessBaseFields(t *testing.T) {
	now := time.Now()
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Source:     "oauth",
		Cached:     false,
		Snapshot: &models.UsageSnapshot{
			Provider:  "claude",
			FetchedAt: now,
			Periods:   []models.UsagePeriod{},
		},
	}

	data := SnapshotToJSON(outcome)

	if data["provider"] != "claude" {
		t.Errorf("provider = %v, want %q", data["provider"], "claude")
	}
	if data["source"] != "oauth" {
		t.Errorf("source = %v, want %q", data["source"], "oauth")
	}
	if data["cached"] != false {
		t.Errorf("cached = %v, want false", data["cached"])
	}
}

func TestSnapshotToJSON_WithIdentity(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{
				Email:        "user@example.com",
				Organization: "Acme Corp",
				Plan:         "pro",
			},
		},
	}

	data := SnapshotToJSON(outcome)

	identity, ok := data["identity"].(map[string]any)
	if !ok {
		t.Fatal("expected 'identity' key")
	}
	if identity["email"] != "user@example.com" {
		t.Errorf("identity.email = %v, want %q", identity["email"], "user@example.com")
	}
	if identity["organization"] != "Acme Corp" {
		t.Errorf("identity.organization = %v, want %q", identity["organization"], "Acme Corp")
	}
	if identity["plan"] != "pro" {
		t.Errorf("identity.plan = %v, want %q", identity["plan"], "pro")
	}
}

func TestSnapshotToJSON_NoIdentity(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Identity: nil,
		},
	}

	data := SnapshotToJSON(outcome)
	if _, ok := data["identity"]; ok {
		t.Error("identity should be absent when nil")
	}
}

func TestSnapshotToJSON_Periods(t *testing.T) {
	reset := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Periods: []models.UsagePeriod{
				{
					Name:        "Monthly",
					Utilization: 75,
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    &reset,
				},
				{
					Name:        "Sonnet Daily",
					Utilization: 30,
					PeriodType:  models.PeriodDaily,
					Model:       "claude-3-sonnet",
				},
			},
		},
	}

	data := SnapshotToJSON(outcome)

	periods, ok := data["periods"].([]map[string]any)
	if !ok {
		t.Fatal("expected 'periods' as []map[string]any")
	}
	if len(periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(periods))
	}

	// Period 0: Monthly with reset
	p0 := periods[0]
	if p0["name"] != "Monthly" {
		t.Errorf("period[0].name = %v, want %q", p0["name"], "Monthly")
	}
	if p0["utilization"] != 75 {
		t.Errorf("period[0].utilization = %v, want 75", p0["utilization"])
	}
	if p0["remaining"] != 25 {
		t.Errorf("period[0].remaining = %v, want 25", p0["remaining"])
	}
	if p0["period_type"] != "monthly" {
		t.Errorf("period[0].period_type = %v, want %q", p0["period_type"], "monthly")
	}
	if _, ok := p0["resets_at"]; !ok {
		t.Error("period[0] should have 'resets_at'")
	}

	// Period 1: model-specific, no reset
	p1 := periods[1]
	if p1["model"] != "claude-3-sonnet" {
		t.Errorf("period[1].model = %v, want %q", p1["model"], "claude-3-sonnet")
	}
	if _, ok := p1["resets_at"]; ok {
		t.Error("period[1] should not have 'resets_at' when nil")
	}
}

func TestSnapshotToJSON_WithOverage(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Overage: &models.OverageUsage{
				Used:      15.50,
				Limit:     100.00,
				Currency:  "USD",
				IsEnabled: true,
			},
		},
	}

	data := SnapshotToJSON(outcome)

	overage, ok := data["overage"].(map[string]any)
	if !ok {
		t.Fatal("expected 'overage' key")
	}
	if overage["used"] != 15.50 {
		t.Errorf("overage.used = %v, want 15.50", overage["used"])
	}
	if overage["limit"] != 100.00 {
		t.Errorf("overage.limit = %v, want 100.00", overage["limit"])
	}
	if overage["remaining"] != 84.50 {
		t.Errorf("overage.remaining = %v, want 84.50", overage["remaining"])
	}
	if overage["currency"] != "USD" {
		t.Errorf("overage.currency = %v, want %q", overage["currency"], "USD")
	}
}

func TestSnapshotToJSON_OverageDisabled(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Overage: &models.OverageUsage{
				Used:      5.0,
				Limit:     100.0,
				IsEnabled: false,
			},
		},
	}

	data := SnapshotToJSON(outcome)
	if _, ok := data["overage"]; ok {
		t.Error("overage should be absent when disabled")
	}
}

func TestSnapshotToJSON_OverageNil(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Overage:  nil,
		},
	}

	data := SnapshotToJSON(outcome)
	if _, ok := data["overage"]; ok {
		t.Error("overage should be absent when nil")
	}
}

// OutputMultiProviderJSON structural tests

func TestOutputMultiProviderJSON_Structure(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now()
	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Source:     "oauth",
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: now,
				Periods:   []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
			},
		},
		"cursor": {
			ProviderID: "cursor",
			Success:    false,
			Error:      "auth failed",
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Top-level keys
	if _, ok := parsed["providers"]; !ok {
		t.Error("missing 'providers' key")
	}
	if _, ok := parsed["errors"]; !ok {
		t.Error("missing 'errors' key")
	}
	if _, ok := parsed["fetched_at"]; !ok {
		t.Error("missing 'fetched_at' key")
	}

	// Providers section
	providers, _ := parsed["providers"].(map[string]any)
	if _, ok := providers["claude"]; !ok {
		t.Error("missing 'claude' in providers")
	}

	// Errors section
	errors, _ := parsed["errors"].(map[string]any)
	if _, ok := errors["cursor"]; !ok {
		t.Error("missing 'cursor' in errors")
	}
	if errors["cursor"] != "auth failed" {
		t.Errorf("errors.cursor = %v, want %q", errors["cursor"], "auth failed")
	}
}

func TestOutputMultiProviderJSON_EmptyOutcomes(t *testing.T) {
	var buf bytes.Buffer
	OutputMultiProviderJSON(&buf, map[string]fetch.FetchOutcome{})

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	providers, _ := parsed["providers"].(map[string]any)
	if len(providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(providers))
	}
}

func TestOutputMultiProviderJSON_ErrorWithEmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	outcomes := map[string]fetch.FetchOutcome{
		"broken": {
			ProviderID: "broken",
			Success:    false,
			Error:      "",
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	errors, _ := parsed["errors"].(map[string]any)
	if errors["broken"] != "Unknown error" {
		t.Errorf("expected 'Unknown error' for empty message, got: %v", errors["broken"])
	}
}

// OutputStatusJSON structural tests

func TestOutputStatusJSON_Structure(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "All systems go",
			UpdatedAt:   &now,
		},
		"copilot": {
			Level:       models.StatusDegraded,
			Description: "Partial issues",
			UpdatedAt:   nil,
		},
	}

	OutputStatusJSON(&buf, statuses)

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	claude, ok := parsed["claude"].(map[string]any)
	if !ok {
		t.Fatal("missing 'claude' key")
	}
	if claude["level"] != "operational" {
		t.Errorf("claude.level = %v, want %q", claude["level"], "operational")
	}
	if _, ok := claude["updated_at"]; !ok {
		t.Error("claude should have 'updated_at'")
	}

	copilot, ok := parsed["copilot"].(map[string]any)
	if !ok {
		t.Fatal("missing 'copilot' key")
	}
	if _, ok := copilot["updated_at"]; ok {
		t.Error("copilot should not have 'updated_at' when nil")
	}
}
