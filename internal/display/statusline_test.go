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

func TestRenderStatusline(t *testing.T) {
	now := time.Now()
	resetAt := now.Add(24 * time.Hour)

	tests := []struct {
		name         string
		opts         StatuslineOptions
		outcomes     map[string]fetch.FetchOutcome
		wantErr      bool
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "pretty mode with single provider",
			opts: StatuslineOptions{Mode: StatuslineModePretty},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "claude",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{
								Name:        "Weekly",
								Utilization: 50,
								PeriodType:  models.PeriodWeekly,
								ResetsAt:    &resetAt,
							},
						},
					},
				},
			},
			wantContains: []string{"7d", "50%", "░", "█"},
			wantMissing:  []string{"Claude"}, // single provider hides label
		},
		{
			name: "pretty mode with multiple providers shows labels",
			opts: StatuslineOptions{Mode: StatuslineModePretty},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "claude",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{
								Name:        "Weekly",
								Utilization: 50,
								PeriodType:  models.PeriodWeekly,
								ResetsAt:    &resetAt,
							},
						},
					},
				},
				"codex": {
					ProviderID: "codex",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "codex",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{
								Name:        "Weekly",
								Utilization: 30,
								PeriodType:  models.PeriodWeekly,
								ResetsAt:    &resetAt,
							},
						},
					},
				},
			},
			wantContains: []string{"Claude", "Codex"},
		},
		{
			name: "short mode",
			opts: StatuslineOptions{Mode: StatuslineModeShort},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "claude",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{
								Name:        "Weekly",
								Utilization: 75,
								PeriodType:  models.PeriodWeekly,
								ResetsAt:    &resetAt,
							},
						},
					},
				},
			},
			wantContains: []string{"75%", "7d"},
			wantMissing:  []string{"█", "░"}, // no bar in short mode
		},
		{
			name: "json mode",
			opts: StatuslineOptions{Mode: StatuslineModeJSON},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "claude",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{
								Name:        "Weekly",
								Utilization: 50,
								PeriodType:  models.PeriodWeekly,
							},
						},
					},
				},
			},
			wantContains: []string{"claude", "Weekly", "50", "weekly"},
		},
		{
			name: "failed provider is omitted from table modes",
			opts: StatuslineOptions{Mode: StatuslineModeShort},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    false,
					Error:      "not configured",
				},
			},
			wantContains: []string{},
		},
		{
			name: "limit restricts periods",
			opts: StatuslineOptions{Mode: StatuslineModeShort, Limit: 1},
			outcomes: map[string]fetch.FetchOutcome{
				"claude": {
					ProviderID: "claude",
					Success:    true,
					Snapshot: &models.UsageSnapshot{
						Provider:  "claude",
						FetchedAt: now,
						Periods: []models.UsagePeriod{
							{Name: "Session", Utilization: 20, PeriodType: models.PeriodSession, ResetsAt: &resetAt},
							{Name: "Weekly", Utilization: 50, PeriodType: models.PeriodWeekly, ResetsAt: &resetAt},
						},
					},
				},
			},
			wantContains: []string{"20%"},
			wantMissing:  []string{"50%"}, // second period should be cut
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderStatusline(&buf, tt.outcomes, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("RenderStatusline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := buf.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q, got:\n%s", want, got)
				}
			}
			for _, miss := range tt.wantMissing {
				if strings.Contains(got, miss) {
					t.Errorf("output should not contain %q, got:\n%s", miss, got)
				}
			}
		})
	}
}

func TestRenderStatuslineJSONStructure(t *testing.T) {
	now := time.Now()
	resetAt := now.Add(24 * time.Hour)

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: now,
				Periods: []models.UsagePeriod{
					{
						Name:        "Weekly",
						Utilization: 50,
						PeriodType:  models.PeriodWeekly,
						ResetsAt:    &resetAt,
					},
				},
				Overage: &models.OverageUsage{
					Used:      10.5,
					Limit:     100.0,
					Currency:  "USD",
					IsEnabled: true,
				},
			},
		},
		"codex": {
			ProviderID: "codex",
			Success:    false,
			Error:      "not configured",
		},
	}

	var buf bytes.Buffer
	err := RenderStatusline(&buf, outcomes, StatuslineOptions{Mode: StatuslineModeJSON})
	if err != nil {
		t.Fatalf("RenderStatusline() error = %v", err)
	}

	var entries []StatuslineJSON
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v\nOutput: %s", err, buf.String())
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	for _, e := range entries {
		switch e.Provider {
		case "claude":
			if len(e.Periods) != 1 {
				t.Errorf("Expected 1 period for claude, got %d", len(e.Periods))
			}
			if e.Overage == nil {
				t.Error("Expected overage data for claude")
			}
			if e.Error != "" {
				t.Errorf("Expected no error for claude, got %q", e.Error)
			}
		case "codex":
			if e.Error != "not configured" {
				t.Errorf("Expected error for codex, got %q", e.Error)
			}
		default:
			t.Errorf("Unexpected provider: %s", e.Provider)
		}
	}
}

func TestFormatDurationCompact(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"days and hours", 7*24*time.Hour + 5*time.Hour, "7d5h"},
		{"days only", 3 * 24 * time.Hour, "3d"},
		{"hours only", 5 * time.Hour, "5h"},
		{"minutes only", 30 * time.Minute, "30m"},
		{"less than minute", 30 * time.Second, "<1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDurationCompact(&tt.duration)
			if got != tt.want {
				t.Errorf("formatDurationCompact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDurationCompactNil(t *testing.T) {
	got := formatDurationCompact(nil)
	if got != "" {
		t.Errorf("formatDurationCompact(nil) = %q, want empty string", got)
	}
}

func TestPeriodDurationLabel(t *testing.T) {
	tests := []struct {
		pt   models.PeriodType
		want string
	}{
		{models.PeriodSession, "5h"},
		{models.PeriodDaily, "24h"},
		{models.PeriodWeekly, "7d"},
		{models.PeriodMonthly, "30d"},
		{models.PeriodType("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pt), func(t *testing.T) {
			got := periodDurationLabel(tt.pt)
			if got != tt.want {
				t.Errorf("periodDurationLabel(%q) = %q, want %q", tt.pt, got, tt.want)
			}
		})
	}
}

func TestPeriodNameParts(t *testing.T) {
	tests := []struct {
		name          string
		p             models.UsagePeriod
		wantQualifier string
		wantDuration  string
	}{
		{"generic weekly", models.UsagePeriod{Name: "Weekly", PeriodType: models.PeriodWeekly}, "", "7d"},
		{"generic session", models.UsagePeriod{Name: "Session (5h)", PeriodType: models.PeriodSession}, "", "5h"},
		{"all models", models.UsagePeriod{Name: "All Models", PeriodType: models.PeriodWeekly}, "", "7d"},
		{"qualified monthly", models.UsagePeriod{Name: "Monthly (Premium)", PeriodType: models.PeriodMonthly}, "Prem", "30d"},
		{"qualified chat", models.UsagePeriod{Name: "Monthly (Chat)", PeriodType: models.PeriodMonthly}, "Chat", "30d"},
		{"long qualifier truncated", models.UsagePeriod{Name: "Monthly (Completions)", PeriodType: models.PeriodMonthly}, "Comp", "30d"},
		{"named period no parens", models.UsagePeriod{Name: "Coding Plan", PeriodType: models.PeriodSession}, "", "5h"},
		{"daily free quota", models.UsagePeriod{Name: "Daily Free Quota", PeriodType: models.PeriodDaily}, "", "24h"},
		{"monthly tools", models.UsagePeriod{Name: "Monthly Tools", PeriodType: models.PeriodMonthly}, "", "30d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQ, gotD := periodNameParts(tt.p)
			if gotQ != tt.wantQualifier || gotD != tt.wantDuration {
				t.Errorf("periodNameParts(%q) = (%q, %q), want (%q, %q)",
					tt.p.Name, gotQ, gotD, tt.wantQualifier, tt.wantDuration)
			}
		})
	}
}

func TestStatuslinePeriods(t *testing.T) {
	snap := models.UsageSnapshot{
		Periods: []models.UsagePeriod{
			{Name: "Session", PeriodType: models.PeriodSession, Model: ""},
			{Name: "Daily", PeriodType: models.PeriodDaily, Model: ""},
			{Name: "Weekly", PeriodType: models.PeriodWeekly, Model: ""},
			{Name: "Monthly", PeriodType: models.PeriodMonthly, Model: ""},
			{Name: "Model-specific", PeriodType: models.PeriodWeekly, Model: "gpt-4"},
		},
	}

	got := statuslinePeriods(snap)

	if len(got) != 4 {
		t.Errorf("Expected 4 periods, got %d", len(got))
	}

	expectedOrder := []models.PeriodType{
		models.PeriodSession,
		models.PeriodDaily,
		models.PeriodWeekly,
		models.PeriodMonthly,
	}

	for i, want := range expectedOrder {
		if i >= len(got) {
			break
		}
		if got[i].PeriodType != want {
			t.Errorf("Period %d: expected %v, got %v", i, want, got[i].PeriodType)
		}
	}
}

func TestStatuslinePeriodsPicksHighestUtilization(t *testing.T) {
	snap := models.UsageSnapshot{
		Periods: []models.UsagePeriod{
			{Name: "Weekly (Standard)", Utilization: 30, PeriodType: models.PeriodWeekly},
			{Name: "Weekly (Premium)", Utilization: 80, PeriodType: models.PeriodWeekly},
		},
	}

	got := statuslinePeriods(snap)

	if len(got) != 1 {
		t.Fatalf("Expected 1 period, got %d", len(got))
	}
	if got[0].Utilization != 80 {
		t.Errorf("Expected highest utilization (80), got %d", got[0].Utilization)
	}
}
