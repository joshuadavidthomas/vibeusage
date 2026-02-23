package display

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestOutputJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	if err := OutputJSON(&buf, map[string]string{"key": "value"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	if err := OutputJSON(&buf, map[string]string{"a": "1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "  ") {
		t.Errorf("expected indented output, got: %s", output)
	}
}

func TestOutputJSON_ReturnsErrorOnMarshalFailure(t *testing.T) {
	var buf bytes.Buffer
	// Channels cannot be marshaled to JSON.
	err := OutputJSON(&buf, map[string]any{"bad": make(chan int)})
	if err == nil {
		t.Fatal("expected error for unmarshalable type, got nil")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestOutputJSON_ReturnsErrorOnWriteFailure(t *testing.T) {
	err := OutputJSON(failWriter{}, map[string]string{"a": "1"})
	if err == nil {
		t.Fatal("expected error for failed writer, got nil")
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

	if err := OutputStatusJSON(&buf, statuses); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	if err := OutputMultiProviderJSON(&buf, outcomes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	if err := OutputMultiProviderJSON(&buf, outcomes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	result := SnapshotToJSON(outcome)

	errResult, ok := result.(SnapshotErrorJSON)
	if !ok {
		t.Fatalf("expected SnapshotErrorJSON, got %T", result)
	}
	if errResult.Error.Message != "token expired" {
		t.Errorf("error.message = %q, want %q", errResult.Error.Message, "token expired")
	}
	if errResult.Error.Provider != "claude" {
		t.Errorf("error.provider = %q, want %q", errResult.Error.Provider, "claude")
	}
}

func TestSnapshotToJSON_NilSnapshot(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot:   nil,
	}

	result := SnapshotToJSON(outcome)
	if _, ok := result.(SnapshotErrorJSON); !ok {
		t.Fatalf("expected SnapshotErrorJSON when snapshot is nil, got %T", result)
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Provider != "claude" {
		t.Errorf("provider = %q, want %q", snap.Provider, "claude")
	}
	if snap.Source != "oauth" {
		t.Errorf("source = %q, want %q", snap.Source, "oauth")
	}
	if snap.Cached {
		t.Error("cached should be false")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Identity == nil {
		t.Fatal("expected identity to be set")
	}
	if snap.Identity.Email != "user@example.com" {
		t.Errorf("identity.email = %q, want %q", snap.Identity.Email, "user@example.com")
	}
	if snap.Identity.Organization != "Acme Corp" {
		t.Errorf("identity.organization = %q, want %q", snap.Identity.Organization, "Acme Corp")
	}
	if snap.Identity.Plan != "pro" {
		t.Errorf("identity.plan = %q, want %q", snap.Identity.Plan, "pro")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Identity != nil {
		t.Error("identity should be nil when not provided")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if len(snap.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(snap.Periods))
	}

	p0 := snap.Periods[0]
	if p0.Name != "Monthly" {
		t.Errorf("period[0].name = %q, want %q", p0.Name, "Monthly")
	}
	if p0.Utilization != 75 {
		t.Errorf("period[0].utilization = %d, want 75", p0.Utilization)
	}
	if p0.Remaining != 25 {
		t.Errorf("period[0].remaining = %d, want 25", p0.Remaining)
	}
	if p0.PeriodType != "monthly" {
		t.Errorf("period[0].period_type = %q, want %q", p0.PeriodType, "monthly")
	}
	if p0.ResetsAt == "" {
		t.Error("period[0] should have resets_at")
	}

	p1 := snap.Periods[1]
	if p1.Model != "claude-3-sonnet" {
		t.Errorf("period[1].model = %q, want %q", p1.Model, "claude-3-sonnet")
	}
	if p1.ResetsAt != "" {
		t.Error("period[1] should not have resets_at when nil")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Overage == nil {
		t.Fatal("expected overage to be set")
	}
	if snap.Overage.Used != 15.50 {
		t.Errorf("overage.used = %v, want 15.50", snap.Overage.Used)
	}
	if snap.Overage.Limit != 100.00 {
		t.Errorf("overage.limit = %v, want 100.00", snap.Overage.Limit)
	}
	if snap.Overage.Remaining != 84.50 {
		t.Errorf("overage.remaining = %v, want 84.50", snap.Overage.Remaining)
	}
	if snap.Overage.Currency != "USD" {
		t.Errorf("overage.currency = %q, want %q", snap.Overage.Currency, "USD")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Overage != nil {
		t.Error("overage should be nil when disabled")
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

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Overage != nil {
		t.Error("overage should be nil when nil")
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

	if err := OutputMultiProviderJSON(&buf, outcomes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result MultiProviderJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result.FetchedAt == "" {
		t.Error("missing fetched_at")
	}

	if _, ok := result.Providers["claude"]; !ok {
		t.Error("missing 'claude' in providers")
	}

	if errMsg, ok := result.Errors["cursor"]; !ok {
		t.Error("missing 'cursor' in errors")
	} else if errMsg != "auth failed" {
		t.Errorf("errors.cursor = %q, want %q", errMsg, "auth failed")
	}
}

func TestOutputMultiProviderJSON_EmptyOutcomes(t *testing.T) {
	var buf bytes.Buffer
	if err := OutputMultiProviderJSON(&buf, map[string]fetch.FetchOutcome{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result MultiProviderJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(result.Providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(result.Providers))
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

	if err := OutputMultiProviderJSON(&buf, outcomes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result MultiProviderJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result.Errors["broken"] != "Unknown error" {
		t.Errorf("expected 'Unknown error' for empty message, got: %v", result.Errors["broken"])
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

	if err := OutputStatusJSON(&buf, statuses); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]StatusEntryJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	claude, ok := result["claude"]
	if !ok {
		t.Fatal("missing 'claude' key")
	}
	if claude.Level != "operational" {
		t.Errorf("claude.level = %q, want %q", claude.Level, "operational")
	}
	if claude.Description != "All systems go" {
		t.Errorf("claude.description = %q, want %q", claude.Description, "All systems go")
	}
	if claude.UpdatedAt == "" {
		t.Error("claude should have updated_at")
	}

	copilot, ok := result["copilot"]
	if !ok {
		t.Fatal("missing 'copilot' key")
	}
	if copilot.UpdatedAt != "" {
		t.Errorf("copilot should not have updated_at, got %q", copilot.UpdatedAt)
	}
}
