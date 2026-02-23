package display

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// These tests verify SnapshotToJSON returns typed structs, not map[string]any.
// They use direct type assertions, so they will fail to compile if the
// function returns map[string]any.

func TestSnapshotToJSON_FailedOutcome_ReturnsSnapshotErrorJSON(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    false,
		Error:      "token expired",
	}

	result := SnapshotToJSON(outcome)

	// This will only compile if SnapshotToJSON returns any (or the typed struct).
	// After migration, this direct assertion validates the concrete type.
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

func TestSnapshotToJSON_NilSnapshot_ReturnsSnapshotErrorJSON(t *testing.T) {
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

func TestSnapshotToJSON_Success_ReturnsSnapshotJSON(t *testing.T) {
	reset := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Source:     "oauth",
		Cached:     true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{
				Email:        "user@example.com",
				Organization: "Acme",
				Plan:         "pro",
			},
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

	if snap.Provider != "claude" {
		t.Errorf("provider = %q, want %q", snap.Provider, "claude")
	}
	if snap.Source != "oauth" {
		t.Errorf("source = %q, want %q", snap.Source, "oauth")
	}
	if !snap.Cached {
		t.Error("cached should be true")
	}

	// Identity
	if snap.Identity == nil {
		t.Fatal("identity should not be nil")
	}
	if snap.Identity.Email != "user@example.com" {
		t.Errorf("identity.email = %q, want %q", snap.Identity.Email, "user@example.com")
	}
	if snap.Identity.Organization != "Acme" {
		t.Errorf("identity.organization = %q, want %q", snap.Identity.Organization, "Acme")
	}
	if snap.Identity.Plan != "pro" {
		t.Errorf("identity.plan = %q, want %q", snap.Identity.Plan, "pro")
	}

	// Periods
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
	if p0.ResetsAt != reset.Format(time.RFC3339) {
		t.Errorf("period[0].resets_at = %q, want %q", p0.ResetsAt, reset.Format(time.RFC3339))
	}

	p1 := snap.Periods[1]
	if p1.Model != "claude-3-sonnet" {
		t.Errorf("period[1].model = %q, want %q", p1.Model, "claude-3-sonnet")
	}
	if p1.ResetsAt != "" {
		t.Errorf("period[1].resets_at should be empty, got %q", p1.ResetsAt)
	}

	// Overage
	if snap.Overage == nil {
		t.Fatal("overage should not be nil")
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

func TestSnapshotToJSON_NoIdentity_OmittedInJSON(t *testing.T) {
	outcome := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Snapshot: &models.UsageSnapshot{
			Provider: "claude",
		},
	}

	result := SnapshotToJSON(outcome)
	snap, ok := result.(SnapshotJSON)
	if !ok {
		t.Fatalf("expected SnapshotJSON, got %T", result)
	}

	if snap.Identity != nil {
		t.Error("identity should be nil")
	}

	// Verify omitempty works in serialized form
	b, _ := json.Marshal(snap)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	if _, ok := raw["identity"]; ok {
		t.Error("identity should be omitted from JSON when nil")
	}
}

func TestSnapshotToJSON_OverageDisabled_OmittedInJSON(t *testing.T) {
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

func TestSnapshotToJSON_OverageNil_OmittedInJSON(t *testing.T) {
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
		t.Error("overage should be nil")
	}
}

// OutputMultiProviderJSON typed struct tests

func TestOutputMultiProviderJSON_TypedStruct(t *testing.T) {
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

	var result MultiProviderJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal into MultiProviderJSON failed: %v", err)
	}

	if result.FetchedAt == "" {
		t.Error("fetched_at should not be empty")
	}

	if _, ok := result.Providers["claude"]; !ok {
		t.Error("expected 'claude' in providers")
	}

	if errMsg, ok := result.Errors["cursor"]; !ok {
		t.Error("expected 'cursor' in errors")
	} else if errMsg != "auth failed" {
		t.Errorf("errors.cursor = %q, want %q", errMsg, "auth failed")
	}
}

func TestOutputMultiProviderJSON_ErrorFallback_TypedStruct(t *testing.T) {
	var buf bytes.Buffer

	outcomes := map[string]fetch.FetchOutcome{
		"broken": {
			ProviderID: "broken",
			Success:    false,
			Error:      "",
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	var result MultiProviderJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal into MultiProviderJSON failed: %v", err)
	}

	if result.Errors["broken"] != "Unknown error" {
		t.Errorf("expected 'Unknown error', got %q", result.Errors["broken"])
	}
}

// OutputStatusJSON typed struct tests

func TestOutputStatusJSON_TypedStruct(t *testing.T) {
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

	var result map[string]StatusEntryJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal into map[string]StatusEntryJSON failed: %v", err)
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
		t.Error("claude.updated_at should not be empty")
	}

	copilot, ok := result["copilot"]
	if !ok {
		t.Fatal("missing 'copilot' key")
	}
	if copilot.UpdatedAt != "" {
		t.Errorf("copilot.updated_at should be empty, got %q", copilot.UpdatedAt)
	}
}
