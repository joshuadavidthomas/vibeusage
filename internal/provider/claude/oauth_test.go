package claude

import (
	"errors"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func floatPtr(f float64) *float64 { return &f }

func TestParseOAuthUsageResponse_FullResponse(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{
			Utilization: 42.0,
			ResetsAt:    "2025-02-19T22:00:00Z",
		},
		SevenDay: &UsagePeriodResponse{
			Utilization: 75.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		Monthly: &UsagePeriodResponse{
			Utilization: 30.0,
			ResetsAt:    "2025-03-01T00:00:00Z",
		},
		SevenDaySonnet: &UsagePeriodResponse{
			Utilization: 60.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		SevenDayOpus: &UsagePeriodResponse{
			Utilization: 10.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		SevenDayHaiku: &UsagePeriodResponse{
			Utilization: 90.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		ExtraUsage: &ExtraUsageResponse{
			IsEnabled:    true,
			UsedCredits:  550,
			MonthlyLimit: floatPtr(10000),
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "claude" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "claude")
	}
	if snapshot.Source != "oauth" {
		t.Errorf("source = %q, want %q", snapshot.Source, "oauth")
	}

	// Should have 6 periods: 3 standard + 3 model-specific
	if len(snapshot.Periods) != 6 {
		t.Fatalf("len(periods) = %d, want 6", len(snapshot.Periods))
	}

	// Find periods by name
	periodByName := make(map[string]models.UsagePeriod)
	for _, p := range snapshot.Periods {
		periodByName[p.Name] = p
	}

	// Standard periods
	fiveHour, ok := periodByName["Session (5h)"]
	if !ok {
		t.Fatal("missing Session (5h) period")
	}
	if fiveHour.Utilization != 42 {
		t.Errorf("Session (5h) utilization = %d, want 42", fiveHour.Utilization)
	}
	if fiveHour.PeriodType != models.PeriodSession {
		t.Errorf("Session (5h) period_type = %q, want %q", fiveHour.PeriodType, models.PeriodSession)
	}
	if fiveHour.ResetsAt == nil {
		t.Fatal("Session (5h) resets_at should not be nil")
	}

	sevenDay, ok := periodByName["All Models"]
	if !ok {
		t.Fatal("missing All Models period")
	}
	if sevenDay.Utilization != 75 {
		t.Errorf("All Models utilization = %d, want 75", sevenDay.Utilization)
	}
	if sevenDay.PeriodType != models.PeriodWeekly {
		t.Errorf("All Models period_type = %q, want %q", sevenDay.PeriodType, models.PeriodWeekly)
	}

	monthly, ok := periodByName["Monthly"]
	if !ok {
		t.Fatal("missing Monthly period")
	}
	if monthly.Utilization != 30 {
		t.Errorf("Monthly utilization = %d, want 30", monthly.Utilization)
	}
	if monthly.PeriodType != models.PeriodMonthly {
		t.Errorf("Monthly period_type = %q, want %q", monthly.PeriodType, models.PeriodMonthly)
	}

	// Model-specific periods
	sonnet, ok := periodByName["Sonnet"]
	if !ok {
		t.Fatal("missing Sonnet period")
	}
	if sonnet.Utilization != 60 {
		t.Errorf("Sonnet utilization = %d, want 60", sonnet.Utilization)
	}
	if sonnet.Model != "sonnet" {
		t.Errorf("Sonnet model = %q, want %q", sonnet.Model, "sonnet")
	}
	if sonnet.PeriodType != models.PeriodWeekly {
		t.Errorf("Sonnet period_type = %q, want %q", sonnet.PeriodType, models.PeriodWeekly)
	}

	opus, ok := periodByName["Opus"]
	if !ok {
		t.Fatal("missing Opus period")
	}
	if opus.Utilization != 10 {
		t.Errorf("Opus utilization = %d, want 10", opus.Utilization)
	}

	haiku, ok := periodByName["Haiku"]
	if !ok {
		t.Fatal("missing Haiku period")
	}
	if haiku.Utilization != 90 {
		t.Errorf("Haiku utilization = %d, want 90", haiku.Utilization)
	}

	// Overage
	if snapshot.Overage == nil {
		t.Fatal("expected overage to be present")
	}
	if snapshot.Overage.Used != 5.50 {
		t.Errorf("overage used = %v, want 5.50", snapshot.Overage.Used)
	}
	if snapshot.Overage.Limit != 100.0 {
		t.Errorf("overage limit = %v, want 100.0", snapshot.Overage.Limit)
	}
	if snapshot.Overage.Currency != "USD" {
		t.Errorf("overage currency = %q, want %q", snapshot.Overage.Currency, "USD")
	}
	if !snapshot.Overage.IsEnabled {
		t.Error("expected overage.is_enabled to be true")
	}
}

func TestParseOAuthUsageResponse_MinimalResponse(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{
			Utilization: 10.0,
			ResetsAt:    "2025-02-19T22:00:00Z",
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("len(periods) = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Name != "Session (5h)" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Session (5h)")
	}
	if snapshot.Periods[0].Utilization != 10 {
		t.Errorf("utilization = %d, want 10", snapshot.Periods[0].Utilization)
	}
	if snapshot.Overage != nil {
		t.Error("expected overage to be nil")
	}
}

func TestParseOAuthUsageResponse_EmptyResponse(t *testing.T) {
	resp := OAuthUsageResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Periods) != 0 {
		t.Errorf("len(periods) = %d, want 0", len(snapshot.Periods))
	}
}

func TestParseOAuthUsageResponse_OverageDisabled(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{Utilization: 20.0},
		ExtraUsage: &ExtraUsageResponse{
			IsEnabled:    false,
			UsedCredits:  0,
			MonthlyLimit: floatPtr(0),
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Overage != nil {
		t.Error("expected overage to be nil when disabled")
	}
}

func TestParseOAuthUsageResponse_ResetsAtParsing(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{
			Utilization: 50.0,
			ResetsAt:    "2025-02-19T22:00:00Z",
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	p := snapshot.Periods[0]
	if p.ResetsAt == nil {
		t.Fatal("expected resets_at to be set")
	}

	expected := time.Date(2025, 2, 19, 22, 0, 0, 0, time.UTC)
	if !p.ResetsAt.Equal(expected) {
		t.Errorf("resets_at = %v, want %v", p.ResetsAt, expected)
	}
}

func TestParseOAuthUsageResponse_InvalidResetsAt(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{
			Utilization: 50.0,
			ResetsAt:    "not-a-date",
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	p := snapshot.Periods[0]
	if p.ResetsAt != nil {
		t.Error("expected resets_at to be nil for invalid date")
	}
	// Utilization should still be parsed
	if p.Utilization != 50 {
		t.Errorf("utilization = %d, want 50", p.Utilization)
	}
}

func TestParseOAuthUsageResponse_NewPeriodFields(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour:          &UsagePeriodResponse{Utilization: 9.0},
		SevenDay:          &UsagePeriodResponse{Utilization: 2.0},
		SevenDayOAuthApps: &UsagePeriodResponse{Utilization: 15.0},
		SevenDayCowork:    &UsagePeriodResponse{Utilization: 30.0},
		IguanaNecktie:     &UsagePeriodResponse{Utilization: 5.0},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	periodByName := make(map[string]models.UsagePeriod)
	for _, p := range snapshot.Periods {
		periodByName[p.Name] = p
	}

	oauthApps, ok := periodByName["OAuth Apps"]
	if !ok {
		t.Fatal("missing OAuth Apps period")
	}
	if oauthApps.Utilization != 15 {
		t.Errorf("OAuth Apps utilization = %d, want 15", oauthApps.Utilization)
	}
	if oauthApps.Model != "oauth_apps" {
		t.Errorf("OAuth Apps model = %q, want %q", oauthApps.Model, "oauth_apps")
	}

	cowork, ok := periodByName["Cowork"]
	if !ok {
		t.Fatal("missing Cowork period")
	}
	if cowork.Utilization != 30 {
		t.Errorf("Cowork utilization = %d, want 30", cowork.Utilization)
	}
	if cowork.Model != "cowork" {
		t.Errorf("Cowork model = %q, want %q", cowork.Model, "cowork")
	}

	iguana, ok := periodByName["Iguana Necktie"]
	if !ok {
		t.Fatal("missing Iguana Necktie period")
	}
	if iguana.Utilization != 5 {
		t.Errorf("Iguana Necktie utilization = %d, want 5", iguana.Utilization)
	}
	if iguana.Model != "iguana_necktie" {
		t.Errorf("Iguana Necktie model = %q, want %q", iguana.Model, "iguana_necktie")
	}
}

func TestParseOAuthUsageResponse_NullMonthlyLimit(t *testing.T) {
	resp := OAuthUsageResponse{
		FiveHour: &UsagePeriodResponse{Utilization: 9.0},
		ExtraUsage: &ExtraUsageResponse{
			IsEnabled:    true,
			UsedCredits:  7372,
			MonthlyLimit: nil,
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseOAuthUsageResponse(resp)

	if snapshot.Overage == nil {
		t.Fatal("expected overage to be present")
	}
	if snapshot.Overage.Used != 73.72 {
		t.Errorf("overage used = %v, want 73.72", snapshot.Overage.Used)
	}
	if snapshot.Overage.Limit != 0 {
		t.Errorf("overage limit = %v, want 0 (null means no limit)", snapshot.Overage.Limit)
	}
}

func TestOAuthCredentials_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name  string
		creds OAuthCredentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: OAuthCredentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name: "not expired and outside buffer",
			creds: OAuthCredentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "expired",
			creds: OAuthCredentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "within refresh buffer",
			creds: OAuthCredentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "invalid date",
			creds: OAuthCredentials{
				AccessToken: "tok",
				ExpiresAt:   "garbage",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.NeedsRefresh()
			if got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadKeychainCredentials(t *testing.T) {
	old := readKeychainSecret
	defer func() { readKeychainSecret = old }()

	readKeychainSecret = func(service, account string) (string, error) {
		if service != claudeKeychainSecret {
			t.Fatalf("service = %q, want %q", service, claudeKeychainSecret)
		}
		if account != "" {
			t.Fatalf("account = %q, want empty", account)
		}
		return `{"claudeAiOauth":{"accessToken":"tok","refreshToken":"ref","expiresAt":4102444800000}}`, nil
	}

	s := OAuthStrategy{}
	creds := s.loadKeychainCredentials()
	if creds == nil {
		t.Fatal("expected credentials")
	}
	if creds.AccessToken != "tok" {
		t.Errorf("access_token = %q, want tok", creds.AccessToken)
	}
	if creds.RefreshToken != "ref" {
		t.Errorf("refresh_token = %q, want ref", creds.RefreshToken)
	}
}

func TestLoadKeychainCredentials_Error(t *testing.T) {
	old := readKeychainSecret
	defer func() { readKeychainSecret = old }()

	readKeychainSecret = func(service, account string) (string, error) {
		return "", errors.New("not found")
	}

	s := OAuthStrategy{}
	if creds := s.loadKeychainCredentials(); creds != nil {
		t.Fatalf("expected nil credentials, got %+v", creds)
	}
}
