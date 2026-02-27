package display

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func timePtr(t time.Time) *time.Time { return &t }

func TestStatusSymbol_NoColor_NoANSI(t *testing.T) {
	levels := []models.StatusLevel{
		models.StatusOperational,
		models.StatusDegraded,
		models.StatusPartialOutage,
		models.StatusMajorOutage,
		"unknown",
	}

	for _, level := range levels {
		result := StatusSymbol(level, true)
		if strings.Contains(result, "\x1b[") {
			t.Errorf("StatusSymbol(%q, true) should not contain ANSI codes, got: %q", level, result)
		}
		if result == "" {
			t.Errorf("StatusSymbol(%q, true) should not be empty", level)
		}
	}
}

func TestStatusSymbol_NoColor_ReturnsCorrectSymbols(t *testing.T) {
	tests := []struct {
		level models.StatusLevel
		want  string
	}{
		{models.StatusOperational, "●"},
		{models.StatusDegraded, "◐"},
		{models.StatusPartialOutage, "◑"},
		{models.StatusMajorOutage, "○"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		result := StatusSymbol(tt.level, true)
		if result != tt.want {
			t.Errorf("StatusSymbol(%q, true) = %q, want %q", tt.level, result, tt.want)
		}
	}
}

func TestStatusSymbol_WithColor_ReturnsNonEmpty(t *testing.T) {
	levels := []models.StatusLevel{
		models.StatusOperational,
		models.StatusDegraded,
		models.StatusPartialOutage,
		models.StatusMajorOutage,
	}

	for _, level := range levels {
		result := StatusSymbol(level, false)
		if result == "" {
			t.Errorf("StatusSymbol(%q, false) should not be empty", level)
		}
	}
}

// RenderBar tests

func TestRenderBar_Boundaries(t *testing.T) {
	tests := []struct {
		name        string
		utilization int
		width       int
		wantFilled  int
		wantEmpty   int
	}{
		{"0% utilization", 0, 20, 0, 20},
		{"100% utilization", 100, 20, 20, 0},
		{"50% utilization", 50, 20, 10, 10},
		{"25% utilization", 25, 20, 5, 15},
		{"negative clamped to 0", -10, 20, 0, 20},
		{"over 100 clamped to width", 150, 20, 20, 0},
		{"width 10", 50, 10, 5, 5},
		{"width 1 at 100%", 100, 1, 1, 0},
		{"width 1 at 0%", 0, 1, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderBar(tt.utilization, tt.width, "")
			// Count filled and empty runes (no color = plain text)
			filled := strings.Count(result, "█")
			empty := strings.Count(result, "░")

			if filled != tt.wantFilled {
				t.Errorf("filled blocks = %d, want %d", filled, tt.wantFilled)
			}
			if empty != tt.wantEmpty {
				t.Errorf("empty blocks = %d, want %d", empty, tt.wantEmpty)
			}
		})
	}
}

func TestRenderBar_TotalRunesEqualWidth(t *testing.T) {
	for util := 0; util <= 100; util += 10 {
		result := RenderBar(util, 20, "")
		filled := strings.Count(result, "█")
		empty := strings.Count(result, "░")
		total := filled + empty
		if total != 20 {
			t.Errorf("RenderBar(%d, 20): total runes = %d, want 20", util, total)
		}
	}
}

func TestRenderBar_ColorDoesNotAffectContent(t *testing.T) {
	// With color, the string is wrapped in ANSI escape codes,
	// but should still contain the bar characters
	for _, color := range []string{"green", "yellow", "red"} {
		result := RenderBar(50, 10, color)
		if !strings.Contains(result, "█") {
			t.Errorf("RenderBar with color=%q should contain filled block", color)
		}
		if !strings.Contains(result, "░") {
			t.Errorf("RenderBar with color=%q should contain empty block", color)
		}
	}
}

// FormatPeriodLine tests

func TestFormatPeriodLine_ContainsName(t *testing.T) {
	period := models.UsagePeriod{
		Name:        "Monthly",
		Utilization: 42,
		PeriodType:  models.PeriodMonthly,
	}

	result := FormatPeriodLine(period, 16)
	if !strings.Contains(result, "Monthly") {
		t.Errorf("expected period name in output, got: %q", result)
	}
}

func TestFormatPeriodLine_ContainsPercentage(t *testing.T) {
	period := models.UsagePeriod{
		Name:        "Daily",
		Utilization: 75,
		PeriodType:  models.PeriodDaily,
	}

	result := FormatPeriodLine(period, 16)
	if !strings.Contains(result, "75%") {
		t.Errorf("expected '75%%' in output, got: %q", result)
	}
}

func TestFormatPeriodLine_ContainsBar(t *testing.T) {
	period := models.UsagePeriod{
		Name:        "Session",
		Utilization: 50,
		PeriodType:  models.PeriodSession,
	}

	result := FormatPeriodLine(period, 16)
	if !strings.Contains(result, "█") {
		t.Errorf("expected filled bar character in output, got: %q", result)
	}
	if !strings.Contains(result, "░") {
		t.Errorf("expected empty bar character in output, got: %q", result)
	}
}

func TestFormatPeriodLine_TruncatesLongName(t *testing.T) {
	period := models.UsagePeriod{
		Name:        "VeryLongPeriodNameThatExceedsWidth",
		Utilization: 10,
		PeriodType:  models.PeriodDaily,
	}

	result := FormatPeriodLine(period, 10)
	// The full original name should NOT appear
	if strings.Contains(result, "VeryLongPeriodNameThatExceedsWidth") {
		t.Errorf("expected name to be truncated, got: %q", result)
	}
	// But the truncated portion should
	if !strings.Contains(result, "VeryLongPe") {
		t.Errorf("expected truncated name 'VeryLongPe' in output, got: %q", result)
	}
}

func TestFormatPeriodLine_IncludesResetCountdown(t *testing.T) {
	reset := time.Now().Add(3 * time.Hour)
	period := models.UsagePeriod{
		Name:        "Daily",
		Utilization: 60,
		PeriodType:  models.PeriodDaily,
		ResetsAt:    &reset,
	}

	result := FormatPeriodLine(period, 16)
	if !strings.Contains(result, "resets in") {
		t.Errorf("expected 'resets in' countdown, got: %q", result)
	}
}

func TestFormatPeriodLine_NoResetWhenNil(t *testing.T) {
	period := models.UsagePeriod{
		Name:        "Monthly",
		Utilization: 30,
		PeriodType:  models.PeriodMonthly,
		ResetsAt:    nil,
	}

	result := FormatPeriodLine(period, 16)
	if strings.Contains(result, "resets in") {
		t.Errorf("should not contain 'resets in' when ResetsAt is nil, got: %q", result)
	}
}

// FormatStatusUpdated tests

func TestFormatStatusUpdated(t *testing.T) {
	tests := []struct {
		name string
		time *time.Time
		want string
	}{
		{"nil returns unknown", nil, "unknown"},
		{"just now", timePtr(time.Now()), "just now"},
		{"5 minutes ago", timePtr(time.Now().Add(-5 * time.Minute)), "5m ago"},
		{"1 minute ago", timePtr(time.Now().Add(-1 * time.Minute)), "1m ago"},
		{"2 hours ago", timePtr(time.Now().Add(-2 * time.Hour)), "2h ago"},
		{"1 hour ago", timePtr(time.Now().Add(-1 * time.Hour)), "1h ago"},
		{"2 days ago", timePtr(time.Now().Add(-48 * time.Hour)), "2d ago"},
		{"1 day ago", timePtr(time.Now().Add(-24 * time.Hour)), "1d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatusUpdated(tt.time)
			if got != tt.want {
				t.Errorf("FormatStatusUpdated() = %q, want %q", got, tt.want)
			}
		})
	}
}

// formatAge tests

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"under a minute", 30 * time.Second, "<1m"},
		{"1 minute", 1 * time.Minute, "1m"},
		{"45 minutes", 45 * time.Minute, "45m"},
		{"1 hour", 61 * time.Minute, "1h"},
		{"3 hours", 3 * time.Hour, "3h"},
		{"1 day", 25 * time.Hour, "1d"},
		{"2 days", 50 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(tt.d)
			if got != tt.want {
				t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// RenderSingleProvider tests

func TestRenderSingleProvider_ContainsProviderName(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if !strings.Contains(result, "Claude") {
		t.Errorf("expected title-cased provider name 'Claude', got: %q", result)
	}
}

func TestRenderSingleProvider_HasPanelBorder(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╰") {
		t.Errorf("expected panel border characters, got: %q", result)
	}
}

func TestRenderSingleProvider_SessionAndLongerPeriods(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "cursor",
		Periods: []models.UsagePeriod{
			{Name: "Session", Utilization: 80, PeriodType: models.PeriodSession},
			{Name: "Monthly", Utilization: 40, PeriodType: models.PeriodMonthly},
		},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if !strings.Contains(result, "80%") {
		t.Errorf("expected session utilization '80%%', got: %q", result)
	}
	if !strings.Contains(result, "40%") {
		t.Errorf("expected monthly utilization '40%%', got: %q", result)
	}
	if !strings.Contains(result, "Monthly") {
		t.Errorf("expected 'Monthly' section header, got: %q", result)
	}
}

func TestRenderSingleProvider_WithOverage(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 90, PeriodType: models.PeriodMonthly}},
		Overage: &models.OverageUsage{
			Used:      5.50,
			Limit:     100.00,
			Currency:  "USD",
			IsEnabled: true,
		},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if !strings.Contains(result, "Extra Usage") {
		t.Errorf("expected 'Extra Usage' for overage, got: %q", result)
	}
	if !strings.Contains(result, "$5.50") {
		t.Errorf("expected '$5.50' in overage, got: %q", result)
	}
	if !strings.Contains(result, "$100.00") {
		t.Errorf("expected '$100.00' in overage, got: %q", result)
	}
}

func TestRenderSingleProvider_NoOverageWhenDisabled(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
		Overage: &models.OverageUsage{
			Used:      5.0,
			Limit:     100.0,
			Currency:  "USD",
			IsEnabled: false,
		},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if strings.Contains(result, "Extra Usage") {
		t.Errorf("should not show overage when disabled, got: %q", result)
	}
}

func TestRenderSingleProvider_NoPeriods(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "empty",
		Periods:  nil,
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if !strings.Contains(result, "Empty") {
		t.Errorf("expected title-cased provider name, got: %q", result)
	}
}

func TestRenderSingleProvider_CachedIndicator(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now().Add(-2 * time.Hour),
		Periods:   []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderSingleProvider(snap, true, DetailOptions{})
	if !strings.Contains(result, "2h ago") {
		t.Errorf("expected '2h ago' age indicator for stale data, got: %q", result)
	}
}

func TestRenderSingleProvider_NoAgeIndicatorWhenFresh(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Periods:   []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	if strings.Contains(result, "ago") {
		t.Errorf("should not show age indicator for fresh data, got: %q", result)
	}
}

// RenderProviderPanel tests

func TestRenderProviderPanel_ContainsProviderTitle(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "copilot",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 60, PeriodType: models.PeriodMonthly}},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if !strings.Contains(result, "Copilot") {
		t.Errorf("expected title-cased provider name 'Copilot', got: %q", result)
	}
}

func TestRenderProviderPanel_HasBorder(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	// Rounded border characters
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╰") {
		t.Errorf("expected rounded border characters, got: %q", result)
	}
}

func TestRenderProviderPanel_FiltersModelSpecificPeriods(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods: []models.UsagePeriod{
			{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly, Model: ""},
			{Name: "Sonnet", Utilization: 70, PeriodType: models.PeriodMonthly, Model: "claude-3-sonnet"},
		},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if !strings.Contains(result, "50%") {
		t.Errorf("expected general period '50%%', got: %q", result)
	}
	// Model-specific period should be filtered in compact view
	if strings.Contains(result, "Sonnet") {
		t.Errorf("should not include model-specific period 'Sonnet' in panel, got: %q", result)
	}
}

func TestRenderProviderPanel_RenamesWeeklyDaily(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "gemini",
		Periods: []models.UsagePeriod{
			{Name: "some_weekly_label", Utilization: 30, PeriodType: models.PeriodWeekly},
			{Name: "some_daily_label", Utilization: 45, PeriodType: models.PeriodDaily},
		},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	// Names without parentheses should be normalized
	if !strings.Contains(result, "Weekly") {
		t.Errorf("expected 'Weekly' label for weekly period, got: %q", result)
	}
	if !strings.Contains(result, "Daily") {
		t.Errorf("expected 'Daily' label for daily period, got: %q", result)
	}
}

func TestRenderProviderPanel_WithOverage(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 90, PeriodType: models.PeriodMonthly}},
		Overage: &models.OverageUsage{
			Used:      10.0,
			Limit:     50.0,
			Currency:  "USD",
			IsEnabled: true,
		},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if !strings.Contains(result, "Extra:") {
		t.Errorf("expected compact 'Extra:' format for overage, got: %q", result)
	}
	if !strings.Contains(result, "$10.00") {
		t.Errorf("expected '$10.00' in overage, got: %q", result)
	}
}

func TestRenderProviderPanel_AgeIndicator(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now().Add(-3 * time.Hour),
		Periods:   []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderProviderPanel(snap, true, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if !strings.Contains(result, "3h ago") {
		t.Errorf("expected '3h ago' in panel title, got: %q", result)
	}
}

func TestRenderProviderPanel_NoAgeIndicatorWhenFresh(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Periods:   []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if strings.Contains(result, "ago") {
		t.Errorf("should not show age indicator for fresh data, got: %q", result)
	}
}

// groupPeriods tests (internal, tested via package-level access)

func TestGroupPeriods(t *testing.T) {
	periods := []models.UsagePeriod{
		{Name: "s1", PeriodType: models.PeriodSession},
		{Name: "d1", PeriodType: models.PeriodDaily},
		{Name: "w1", PeriodType: models.PeriodWeekly},
		{Name: "m1", PeriodType: models.PeriodMonthly},
		{Name: "s2", PeriodType: models.PeriodSession},
		{Name: "d2", PeriodType: models.PeriodDaily},
	}

	session, weekly, daily, monthly := groupPeriods(periods)

	if len(session) != 2 {
		t.Errorf("session count = %d, want 2", len(session))
	}
	if len(weekly) != 1 {
		t.Errorf("weekly count = %d, want 1", len(weekly))
	}
	if len(daily) != 2 {
		t.Errorf("daily count = %d, want 2", len(daily))
	}
	if len(monthly) != 1 {
		t.Errorf("monthly count = %d, want 1", len(monthly))
	}
}

func TestGroupPeriods_Empty(t *testing.T) {
	session, weekly, daily, monthly := groupPeriods(nil)
	if session != nil || weekly != nil || daily != nil || monthly != nil {
		t.Error("expected all nil slices for nil input")
	}
}

// pickLonger tests (internal)

func TestPickLonger_Priority(t *testing.T) {
	weekly := []models.UsagePeriod{{Name: "w"}}
	daily := []models.UsagePeriod{{Name: "d"}}
	monthly := []models.UsagePeriod{{Name: "m"}}

	tests := []struct {
		name    string
		weekly  []models.UsagePeriod
		daily   []models.UsagePeriod
		monthly []models.UsagePeriod
		want    string
	}{
		{"weekly first", weekly, daily, monthly, "Weekly"},
		{"daily when no weekly", nil, daily, monthly, "Daily"},
		{"monthly when no weekly or daily", nil, nil, monthly, "Monthly"},
		{"empty when all nil", nil, nil, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickLonger(tt.weekly, tt.daily, tt.monthly)
			if got.header != tt.want {
				t.Errorf("pickLonger() header = %q, want %q", got.header, tt.want)
			}
		})
	}
}

// colorStyle tests (internal)

func TestColorStyle_ValidColors(t *testing.T) {
	for _, color := range []string{"green", "yellow", "red"} {
		style := colorStyle(color)
		rendered := style.Render("test")
		if rendered == "" {
			t.Errorf("colorStyle(%q).Render should produce non-empty output", color)
		}
	}
}

func TestColorStyle_UnknownColor(t *testing.T) {
	style := colorStyle("purple")
	rendered := style.Render("test")
	// Unknown color returns unstyled, should still contain the text
	if !strings.Contains(rendered, "test") {
		t.Errorf("colorStyle(unknown) should still render text, got: %q", rendered)
	}
}

func TestColorStyle_EmptyColor(t *testing.T) {
	style := colorStyle("")
	rendered := style.Render("test")
	runeCount := utf8.RuneCountInString(rendered)
	if runeCount < 4 {
		t.Errorf("colorStyle('').Render('test') should have at least 4 runes, got %d", runeCount)
	}
}

// Overage formatting tests

func TestFormatOverageLine_WithLimit(t *testing.T) {
	o := &models.OverageUsage{Used: 5.50, Limit: 100.00, Currency: "USD", IsEnabled: true}
	got := formatOverageLine(o, "Extra Usage")
	if got != "Extra Usage: $5.50 / $100.00 USD" {
		t.Errorf("formatOverageLine with limit = %q, want %q", got, "Extra Usage: $5.50 / $100.00 USD")
	}
}

func TestFormatOverageLine_ZeroLimit(t *testing.T) {
	o := &models.OverageUsage{Used: 73.72, Limit: 0.00, Currency: "USD", IsEnabled: true}
	got := formatOverageLine(o, "Extra Usage")
	want := "Extra Usage: $73.72 USD (Unlimited)"
	if got != want {
		t.Errorf("formatOverageLine with zero limit = %q, want %q", got, want)
	}
	if strings.Contains(got, "/ $0.00") {
		t.Error("zero limit should not show '/ $0.00'")
	}
}

func TestFormatOverageLine_NonUSDCurrency(t *testing.T) {
	o := &models.OverageUsage{Used: 10.00, Limit: 50.00, Currency: "EUR", IsEnabled: true}
	got := formatOverageLine(o, "Extra")
	if got != "Extra: 10.00 / 50.00 EUR" {
		t.Errorf("formatOverageLine non-USD = %q, want %q", got, "Extra: 10.00 / 50.00 EUR")
	}
}

func TestFormatOverageLine_ZeroUsed(t *testing.T) {
	o := &models.OverageUsage{Used: 0.00, Limit: 100.00, Currency: "USD", IsEnabled: true}
	got := formatOverageLine(o, "Extra")
	if got != "Extra: $0.00 / $100.00 USD" {
		t.Errorf("formatOverageLine zero used = %q, want %q", got, "Extra: $0.00 / $100.00 USD")
	}
}

// Overage edge cases in rendered output

func TestRenderSingleProvider_OverageZeroLimit(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 90, PeriodType: models.PeriodMonthly}},
		Overage:  &models.OverageUsage{Used: 73.72, Limit: 0.00, Currency: "USD", IsEnabled: true},
	}
	result := RenderSingleProvider(snap, false, DetailOptions{})
	if strings.Contains(result, "/ $0.00") {
		t.Errorf("should not show '/ $0.00' for zero limit overage, got: %q", result)
	}
	if !strings.Contains(result, "$73.72") {
		t.Errorf("should show used amount '$73.72', got: %q", result)
	}
	if !strings.Contains(result, "Unlimited") {
		t.Errorf("should show 'Unlimited' for zero limit, got: %q", result)
	}
}

func TestRenderProviderPanel_OverageZeroLimit(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 90, PeriodType: models.PeriodMonthly}},
		Overage:  &models.OverageUsage{Used: 73.72, Limit: 0.00, Currency: "USD", IsEnabled: true},
	}
	result := RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap}))
	if strings.Contains(result, "/ $0.00") {
		t.Errorf("should not show '/ $0.00' for zero limit overage, got: %q", result)
	}
	if !strings.Contains(result, "$73.72") {
		t.Errorf("should show used amount '$73.72', got: %q", result)
	}
	if !strings.Contains(result, "Unlimited") {
		t.Errorf("should show 'Unlimited' for zero limit, got: %q", result)
	}
}

// formatBalance tests

func TestFormatBalance_NilBilling(t *testing.T) {
	got := formatBalance(nil)
	if got != "" {
		t.Errorf("formatBalance(nil) = %q, want empty", got)
	}
}

func TestFormatBalance_NilBalance(t *testing.T) {
	got := formatBalance(&models.BillingDetail{})
	if got != "" {
		t.Errorf("formatBalance(nil balance) = %q, want empty", got)
	}
}

func TestFormatBalance_Positive(t *testing.T) {
	bal := 12.34
	got := formatBalance(&models.BillingDetail{Balance: &bal})
	if got != "Balance: $12.34" {
		t.Errorf("formatBalance = %q, want %q", got, "Balance: $12.34")
	}
}

func TestFormatBalance_Zero(t *testing.T) {
	bal := 0.0
	got := formatBalance(&models.BillingDetail{Balance: &bal})
	if got != "Balance: $0.00" {
		t.Errorf("formatBalance = %q, want %q", got, "Balance: $0.00")
	}
}

func TestFormatBalance_Negative(t *testing.T) {
	bal := -5.50
	got := formatBalance(&models.BillingDetail{Balance: &bal})
	if got != "Balance: -$5.50" {
		t.Errorf("formatBalance = %q, want %q", got, "Balance: -$5.50")
	}
}

// Standalone billing balance in detail view

func TestRenderSingleProvider_BillingBalanceNoOverage(t *testing.T) {
	bal := 12.34
	snap := models.UsageSnapshot{
		Provider: "amp",
		Periods:  []models.UsagePeriod{{Name: "Daily Free Quota", Utilization: 40, PeriodType: models.PeriodDaily}},
		Billing:  &models.BillingDetail{Balance: &bal},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if !strings.Contains(result, "Balance: $12.34") {
		t.Errorf("expected standalone balance line, got: %q", result)
	}
	if strings.Contains(result, "Extra Usage") {
		t.Errorf("should not show Extra Usage when no overage, got: %q", result)
	}
}

func TestRenderSingleProvider_BillingBalanceOnlyNoPeriods(t *testing.T) {
	bal := 5.00
	snap := models.UsageSnapshot{
		Provider: "amp",
		Billing:  &models.BillingDetail{Balance: &bal},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if !strings.Contains(result, "Balance: $5.00") {
		t.Errorf("expected balance line for credits-only snapshot, got: %q", result)
	}
}

func TestRenderSingleProvider_OverageSuppressesBillingBalance(t *testing.T) {
	bal := 42.00
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 90, PeriodType: models.PeriodMonthly}},
		Overage:  &models.OverageUsage{Used: 5.50, Limit: 100.00, Currency: "USD", IsEnabled: true},
		Billing:  &models.BillingDetail{Balance: &bal},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if !strings.Contains(result, "Extra Usage") {
		t.Errorf("expected Extra Usage for overage, got: %q", result)
	}
	// Balance should appear in the billing sub-line under overage, not as standalone
	if !strings.Contains(result, "Balance: $42.00") {
		t.Errorf("expected balance in billing detail sub-line, got: %q", result)
	}
}

// Standalone billing balance in panel view

func TestRenderProviderPanel_BillingBalanceNoOverage(t *testing.T) {
	bal := 8.00
	snap := models.UsageSnapshot{
		Provider: "amp",
		Periods:  []models.UsagePeriod{{Name: "Daily Free Quota", Utilization: 40, PeriodType: models.PeriodDaily}},
		Billing:  &models.BillingDetail{Balance: &bal},
	}

	result := stripANSI(RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap})))
	if !strings.Contains(result, "Balance: $8.00") {
		t.Errorf("expected balance line in panel, got: %q", result)
	}
	if strings.Contains(result, "Extra") {
		t.Errorf("should not show Extra when no overage, got: %q", result)
	}
}

// formatSubPeriodName tests

func TestFormatSubPeriodName_ModelPeriod(t *testing.T) {
	p := &models.UsagePeriod{Name: "Sonnet", Model: "sonnet"}
	got := formatSubPeriodName(p, "Weekly")
	if got != "  Sonnet" {
		t.Errorf("formatSubPeriodName model = %q, want %q", got, "  Sonnet")
	}
}

func TestFormatSubPeriodName_Aggregate(t *testing.T) {
	p := &models.UsagePeriod{Name: "All Models", Model: ""}
	got := formatSubPeriodName(p, "Weekly")
	if got != "  All Models" {
		t.Errorf("formatSubPeriodName aggregate = %q, want %q", got, "  All Models")
	}
}

func TestFormatSubPeriodName_MatchesHeader(t *testing.T) {
	p := &models.UsagePeriod{Name: "Weekly", Model: ""}
	got := formatSubPeriodName(p, "Weekly")
	if got != "  All Models" {
		t.Errorf("formatSubPeriodName matching header = %q, want %q", got, "  All Models")
	}
}

func TestFormatSubPeriodName_Parenthesized(t *testing.T) {
	p := &models.UsagePeriod{Name: "Weekly (Premium)", Model: ""}
	got := formatSubPeriodName(p, "Weekly")
	if got != "  Premium" {
		t.Errorf("formatSubPeriodName parenthesized = %q, want %q", got, "  Premium")
	}
}

// Snapshot-style layout tests
// These strip ANSI codes and verify the structural layout of rendered output.

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestRenderSingleProvider_DetailLayout(t *testing.T) {
	reset := time.Now().Add(3 * time.Hour)
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods: []models.UsagePeriod{
			{Name: "Session (5h)", Utilization: 25, PeriodType: models.PeriodSession, ResetsAt: &reset},
			{Name: "All Models", Utilization: 60, PeriodType: models.PeriodWeekly, ResetsAt: &reset},
			{Name: "Sonnet", Utilization: 80, PeriodType: models.PeriodWeekly, ResetsAt: &reset, Model: "sonnet"},
			{Name: "Opus", Utilization: 10, PeriodType: models.PeriodWeekly, ResetsAt: &reset, Model: "opus"},
		},
		Overage: &models.OverageUsage{Used: 5.50, Limit: 100.00, Currency: "USD", IsEnabled: true},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	lines := strings.Split(result, "\n")

	// First line is the provider title above the panel
	if !strings.Contains(lines[0], "Claude") {
		t.Errorf("first line should be provider title containing 'Claude', got: %q", lines[0])
	}

	// Find the Usage panel borders
	panelStart := -1
	panelEnd := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "╭─") {
			panelStart = i
		}
		if strings.HasPrefix(line, "╰") {
			panelEnd = i
		}
	}
	if panelStart == -1 || panelEnd == -1 {
		t.Fatalf("expected panel borders (╭/╰), got:\n%s", result)
	}

	// Panel title should be "Usage"
	if !strings.Contains(lines[panelStart], "Usage") {
		t.Errorf("panel border should contain 'Usage' title, got: %q", lines[panelStart])
	}

	// Content lines inside the panel should have │ borders
	for _, line := range lines[panelStart+1 : panelEnd] {
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Errorf("content line should be bordered with │, got: %q", line)
		}
	}

	// Verify content structure
	if !strings.Contains(result, "Session (5h)") {
		t.Error("expected session period")
	}
	if !strings.Contains(result, "Weekly") {
		t.Error("expected Weekly section header")
	}
	if !strings.Contains(result, "All Models") {
		t.Error("expected All Models sub-period")
	}
	if !strings.Contains(result, "Sonnet") {
		t.Error("expected Sonnet sub-period")
	}
	if !strings.Contains(result, "Opus") {
		t.Error("expected Opus sub-period")
	}
	if !strings.Contains(result, "Extra Usage: $5.50 / $100.00 USD") {
		t.Error("expected overage line")
	}
	if !strings.Contains(result, "25%") {
		t.Error("expected session utilization")
	}
	if !strings.Contains(result, "resets in") {
		t.Error("expected reset countdown")
	}
}

func TestRenderSingleProvider_DetailLayout_NoPeriods(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  nil,
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	lines := strings.Split(result, "\n")

	// First line is provider title
	if !strings.Contains(lines[0], "Claude") {
		t.Errorf("first line should contain provider name, got: %q", lines[0])
	}
	// Should still have a Usage panel
	if !strings.Contains(result, "Usage") {
		t.Error("expected Usage panel title")
	}
}

func TestRenderProviderPanel_PanelLayout(t *testing.T) {
	reset := time.Now().Add(5*24*time.Hour + 3*time.Hour)
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods: []models.UsagePeriod{
			{Name: "Session (5h)", Utilization: 25, PeriodType: models.PeriodSession, ResetsAt: &reset},
			{Name: "All Models", Utilization: 60, PeriodType: models.PeriodWeekly, ResetsAt: &reset},
			{Name: "Sonnet", Utilization: 80, PeriodType: models.PeriodWeekly, ResetsAt: &reset, Model: "sonnet"},
		},
		Overage: &models.OverageUsage{Used: 10.00, Limit: 50.00, Currency: "USD", IsEnabled: true},
	}

	result := stripANSI(RenderProviderPanel(snap, false, GlobalPeriodColWidths([]models.UsageSnapshot{snap})))

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// All content lines should be the same visual width
	widths := make(map[int]bool)
	for _, line := range lines {
		widths[lipgloss.Width(line)] = true
	}
	if len(widths) > 1 {
		t.Errorf("panel lines should all be the same width, got widths: %v", widths)
	}

	// Should contain aggregate periods but not model-specific
	if !strings.Contains(result, "25%") {
		t.Error("expected session utilization in panel")
	}
	if strings.Contains(result, "Sonnet") {
		t.Error("panel should not contain model-specific period")
	}
	if !strings.Contains(result, "Extra:") {
		t.Error("expected overage line in panel")
	}
}

func TestRenderSingleProvider_WithStatus(t *testing.T) {
	now := time.Now()
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}
	opts := DetailOptions{
		Status: &models.ProviderStatus{
			Level:       models.StatusOperational,
			Description: "All Systems Operational",
			UpdatedAt:   &now,
		},
	}

	result := stripANSI(RenderSingleProvider(snap, false, opts))
	if !strings.Contains(result, "All Systems Operational") {
		t.Errorf("expected status description, got: %q", result)
	}
	if !strings.Contains(result, "●") {
		t.Errorf("expected status symbol, got: %q", result)
	}
}

func TestRenderSingleProvider_WithStatusDegraded(t *testing.T) {
	now := time.Now()
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}
	opts := DetailOptions{
		Status: &models.ProviderStatus{
			Level:       models.StatusDegraded,
			Description: "Elevated error rates",
			UpdatedAt:   &now,
		},
	}

	result := stripANSI(RenderSingleProvider(snap, false, opts))
	if !strings.Contains(result, "Elevated error rates") {
		t.Errorf("expected degraded status description, got: %q", result)
	}
}

func TestRenderSingleProvider_WithIdentity(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
		Identity: &models.ProviderIdentity{Plan: "pro", Email: "user@example.com"},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if !strings.Contains(result, "Plan") || !strings.Contains(result, "pro") {
		t.Errorf("expected labeled plan, got: %q", result)
	}
	if !strings.Contains(result, "Account") || !strings.Contains(result, "user@example.com") {
		t.Errorf("expected labeled email, got: %q", result)
	}
}

func TestRenderSingleProvider_WithSource(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
		Source:   "oauth",
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if !strings.Contains(result, "Auth") || !strings.Contains(result, "OAuth") {
		t.Errorf("expected labeled source 'Auth OAuth', got: %q", result)
	}
}

func TestRenderSingleProvider_NoMetaWhenEmpty(t *testing.T) {
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}

	result := stripANSI(RenderSingleProvider(snap, false, DetailOptions{}))
	if strings.Contains(result, "Auth") || strings.Contains(result, "Plan") {
		t.Errorf("should not show metadata when empty, got: %q", result)
	}
}

func TestRenderSingleProvider_StatusBetweenTitleAndPanel(t *testing.T) {
	now := time.Now()
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods:  []models.UsagePeriod{{Name: "Monthly", Utilization: 50, PeriodType: models.PeriodMonthly}},
	}
	opts := DetailOptions{
		Status: &models.ProviderStatus{
			Level:       models.StatusOperational,
			Description: "All Systems Operational",
			UpdatedAt:   &now,
		},
	}

	result := stripANSI(RenderSingleProvider(snap, false, opts))

	// Verify ordering: title before status before panel
	titleIdx := strings.Index(result, "Claude")
	statusIdx := strings.Index(result, "Operational")
	panelIdx := strings.Index(result, "╭")

	if titleIdx == -1 || statusIdx == -1 || panelIdx == -1 {
		t.Fatalf("missing expected sections in output:\n%s", result)
	}
	if titleIdx >= statusIdx {
		t.Error("title should appear before status")
	}
	if statusIdx >= panelIdx {
		t.Error("status should appear before panel")
	}
}

// identitySummary tests

func TestRenderMetaLine(t *testing.T) {
	tests := []struct {
		name     string
		snapshot models.UsageSnapshot
		contains []string
		empty    bool
	}{
		{
			"plan and source",
			models.UsageSnapshot{
				Identity: &models.ProviderIdentity{Plan: "Pro"},
				Source:   "oauth",
			},
			[]string{"Plan", "Pro", "Auth", "OAuth"},
			false,
		},
		{
			"all identity fields on separate lines",
			models.UsageSnapshot{
				Identity: &models.ProviderIdentity{Plan: "Pro", Organization: "Acme", Email: "user@example.com"},
			},
			[]string{"Plan", "Pro", "Org", "Acme", "Account", "user@example.com"},
			false,
		},
		{
			"source only",
			models.UsageSnapshot{Source: "api_key"},
			[]string{"Auth", "API Key"},
			false,
		},
		{
			"empty",
			models.UsageSnapshot{},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(renderMetaLine(tt.snapshot))
			if tt.empty {
				if got != "" {
					t.Errorf("renderMetaLine() = %q, want empty", got)
				}
				return
			}
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("renderMetaLine() = %q, missing %q", got, s)
				}
			}
		})
	}
}

func TestRenderMetaLine_ColumnAlignment(t *testing.T) {
	snap := models.UsageSnapshot{
		Identity: &models.ProviderIdentity{Plan: "Pro", Organization: "Acme", Email: "user@example.com"},
		Source:   "oauth",
	}
	got := stripANSI(renderMetaLine(snap))
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), got)
	}
	// Longest label is "Account" (7) + 2 gap spaces = values start at column 9
	values := []string{"Pro", "Acme", "user@example.com", "OAuth"}
	for i, line := range lines {
		idx := strings.Index(line, values[i])
		if idx < 0 {
			t.Errorf("line %d missing value %q: %q", i, values[i], line)
			continue
		}
		if idx != 9 {
			t.Errorf("line %d value %q at column %d, want 9:\n%s", i, values[i], idx, got)
		}
	}
}

func TestFormatSourceName(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"oauth", "OAuth"},
		{"web", "Web Session"},
		{"api_key", "API Key"},
		{"device_flow", "Device Flow"},
		{"provider_cli", "CLI"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := formatSourceName(tt.source)
			if got != tt.want {
				t.Errorf("formatSourceName(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

// renderStatusLine tests

func TestRenderStatusLine_Operational(t *testing.T) {
	now := time.Now()
	status := models.ProviderStatus{
		Level:       models.StatusOperational,
		Description: "All Systems Operational",
		UpdatedAt:   &now,
	}
	result := stripANSI(renderStatusLine(status))
	if !strings.Contains(result, "●") {
		t.Error("expected operational symbol ●")
	}
	if !strings.Contains(result, "All Systems Operational") {
		t.Error("expected status description")
	}
	if !strings.Contains(result, "just now") {
		t.Error("expected time indicator")
	}
}

func TestRenderStatusLine_NoDescription(t *testing.T) {
	status := models.ProviderStatus{
		Level: models.StatusDegraded,
	}
	result := stripANSI(renderStatusLine(status))
	// Should fall back to level name
	if !strings.Contains(result, "degraded") {
		t.Errorf("expected level name as fallback, got: %q", result)
	}
}

func TestRenderSingleProvider_ConsistentPanelLineWidths(t *testing.T) {
	reset := time.Now().Add(3 * time.Hour)
	snap := models.UsageSnapshot{
		Provider: "claude",
		Periods: []models.UsagePeriod{
			{Name: "Session (5h)", Utilization: 25, PeriodType: models.PeriodSession, ResetsAt: &reset},
			{Name: "All Models", Utilization: 60, PeriodType: models.PeriodWeekly, ResetsAt: &reset},
			{Name: "Sonnet", Utilization: 80, PeriodType: models.PeriodWeekly, ResetsAt: &reset, Model: "sonnet"},
		},
	}

	result := RenderSingleProvider(snap, false, DetailOptions{})
	lines := strings.Split(result, "\n")

	// Find panel lines (between ╭ and ╰ inclusive)
	panelStart := -1
	panelEnd := -1
	for i, line := range lines {
		stripped := stripANSI(line)
		if strings.HasPrefix(stripped, "╭") {
			panelStart = i
		}
		if strings.HasPrefix(stripped, "╰") {
			panelEnd = i
		}
	}
	if panelStart == -1 || panelEnd == -1 {
		t.Fatal("expected panel borders")
	}

	// All panel lines should be the same visual width
	widths := make(map[int]bool)
	for _, line := range lines[panelStart : panelEnd+1] {
		widths[lipgloss.Width(line)] = true
	}
	if len(widths) > 1 {
		t.Errorf("all panel lines should be the same visual width, got widths: %v", widths)
	}
}
