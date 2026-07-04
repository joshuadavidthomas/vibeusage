package claude

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
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
		SevenDaySonnet: &UsagePeriodResponse{
			Utilization: 60.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		SevenDayOpus: &UsagePeriodResponse{
			Utilization: 10.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		SevenDayOmelette: &UsagePeriodResponse{
			Utilization: 3.0,
			ResetsAt:    "2025-02-26T00:00:00Z",
		},
		ExtraUsage: &ExtraUsageResponse{
			IsEnabled:    true,
			UsedCredits:  550,
			MonthlyLimit: floatPtr(10000),
			Currency:     "USD",
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

	// Should have 5 periods: 2 standard (5h + 7d) + 3 model-specific (sonnet, opus, omelette)
	if len(snapshot.Periods) != 5 {
		t.Fatalf("len(periods) = %d, want 5", len(snapshot.Periods))
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

	design, ok := periodByName["Claude Design"]
	if !ok {
		t.Fatal("missing Claude Design period")
	}
	if design.Utilization != 3 {
		t.Errorf("Claude Design utilization = %d, want 3", design.Utilization)
	}
	if design.Model != "omelette" {
		t.Errorf("Claude Design model = %q, want %q", design.Model, "omelette")
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
		creds oauth.Credentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: oauth.Credentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name: "not expired and outside buffer",
			creds: oauth.Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "expired",
			creds: oauth.Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "within refresh buffer",
			creds: oauth.Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "invalid date",
			creds: oauth.Credentials{
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

func TestLoadCachedIdentity(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantNil bool
	}{
		{
			name:    "no cache",
			setup:   func(t *testing.T) {},
			wantNil: true,
		},
		{
			name: "cache present but identity nil",
			setup: func(t *testing.T) {
				snap := models.UsageSnapshot{
					Provider:  "claude",
					FetchedAt: time.Now().UTC(),
				}
				if err := config.CacheSnapshot(snap); err != nil {
					t.Fatalf("CacheSnapshot: %v", err)
				}
			},
			wantNil: true,
		},
		{
			name: "cache present but identity empty",
			setup: func(t *testing.T) {
				snap := models.UsageSnapshot{
					Provider:  "claude",
					FetchedAt: time.Now().UTC(),
					Identity:  &models.ProviderIdentity{},
				}
				if err := config.CacheSnapshot(snap); err != nil {
					t.Fatalf("CacheSnapshot: %v", err)
				}
			},
			wantNil: true,
		},
		{
			name: "cache stale",
			setup: func(t *testing.T) {
				snap := models.UsageSnapshot{
					Provider:  "claude",
					FetchedAt: time.Now().UTC().Add(-25 * time.Hour),
					Identity:  &models.ProviderIdentity{Email: "u@example.com", Plan: "Max 20x"},
				}
				if err := config.CacheSnapshot(snap); err != nil {
					t.Fatalf("CacheSnapshot: %v", err)
				}
			},
			wantNil: true,
		},
		{
			name: "cache fresh with identity",
			setup: func(t *testing.T) {
				snap := models.UsageSnapshot{
					Provider:  "claude",
					FetchedAt: time.Now().UTC().Add(-1 * time.Hour),
					Identity:  &models.ProviderIdentity{Email: "u@example.com", Plan: "Max 20x"},
				}
				if err := config.CacheSnapshot(snap); err != nil {
					t.Fatalf("CacheSnapshot: %v", err)
				}
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testenv.ApplyVibeusage(t.Setenv, t.TempDir())
			tt.setup(t)

			got := loadCachedIdentity()
			if tt.wantNil && got != nil {
				t.Errorf("loadCachedIdentity() = %+v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("loadCachedIdentity() = nil, want non-nil")
			}
		})
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

// stubKeychainEmpty stubs readKeychainSecret to behave as if the keychain
// has no entry. It restores the previous stub when the test ends.
func stubKeychainEmpty(t *testing.T) {
	t.Helper()
	old := readKeychainSecret
	t.Cleanup(func() { readKeychainSecret = old })
	readKeychainSecret = func(string, string) (string, error) {
		return "", errors.New("no entry")
	}
}

func writeClaudeAuth(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// writeClaudeCLICreds writes a Claude CLI credentials file to ~/.claude.
func writeClaudeCLICreds(t *testing.T, home string) {
	t.Helper()
	writeClaudeAuth(t, home, `{"claudeAiOauth":{"accessToken":"cli-tok","refreshToken":"cli-ref","expiresAt":4102444800000}}`)
}

func prependFakeClaude(t *testing.T, script string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	bin := filepath.Join(binDir, "claude")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func setClaudeOAuthEndpoints(t *testing.T, usageURL, accountURL string) {
	t.Helper()
	oldUsage := oauthUsageEndpoint
	oldAccount := oauthAccountEndpoint
	oauthUsageEndpoint = usageURL
	oauthAccountEndpoint = accountURL
	t.Cleanup(func() {
		oauthUsageEndpoint = oldUsage
		oauthAccountEndpoint = oldAccount
	})
}

func cacheClaudeIdentity(t *testing.T) {
	t.Helper()
	snap := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now().UTC(),
		Identity:  &models.ProviderIdentity{Email: "u@example.com", Plan: "Max"},
	}
	if err := config.CacheSnapshot(snap); err != nil {
		t.Fatalf("CacheSnapshot: %v", err)
	}
}

func TestFetch_UsesExpiredMetadataTokenBeforeRefreshing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	stubKeychainEmpty(t)
	cacheClaudeIdentity(t)
	writeClaudeAuth(t, home, `{"claudeAiOauth":{"accessToken":"still-valid","refreshToken":"ref","expiresAt":1}}`)
	prependFakeClaude(t, "#!/usr/bin/env sh\nexit 42\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer still-valid" {
			t.Errorf("Authorization = %q, want Bearer still-valid", got)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10}}`))
	}))
	defer server.Close()
	setClaudeOAuthEndpoints(t, server.URL+"/usage", server.URL+"/account")

	result, err := (&OAuthStrategy{HTTPTimeout: 2}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() err = %v", err)
	}
	if !result.Success {
		t.Fatalf("Fetch() success = false, error = %q", result.Error)
	}
}

func TestFetch_RefreshesAfterUnauthorized(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	stubKeychainEmpty(t)
	cacheClaudeIdentity(t)
	writeClaudeAuth(t, home, `{"claudeAiOauth":{"accessToken":"stale","refreshToken":"ref","expiresAt":1}}`)
	prependFakeClaude(t, "#!/usr/bin/env sh\ncat > "+strconv.Quote(filepath.Join(home, ".claude", ".credentials.json"))+" <<'JSON'\n{\"claudeAiOauth\":{\"accessToken\":\"fresh\",\"refreshToken\":\"ref\",\"expiresAt\":4102444800000}}\nJSON\n")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.Header.Get("Authorization") {
		case "Bearer stale":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case "Bearer fresh":
			_, _ = w.Write([]byte(`{"five_hour":{"utilization":10}}`))
		default:
			t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	setClaudeOAuthEndpoints(t, server.URL+"/usage", server.URL+"/account")

	result, err := (&OAuthStrategy{HTTPTimeout: 2}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() err = %v", err)
	}
	if !result.Success {
		t.Fatalf("Fetch() success = false, error = %q", result.Error)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("usage requests = %d, want 2", got)
	}
}

func TestLoadCredentials_DeletesOrphanSlotWhenCanonicalFilePresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	stubKeychainEmpty(t)

	writeClaudeCLICreds(t, home)
	if err := config.WriteCredential("claude", "oauth", []byte(`{"access_token":"stale"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	s := OAuthStrategy{}
	creds := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() = nil, want canonical creds")
	}
	if creds.AccessToken != "cli-tok" {
		t.Errorf("access_token = %q, want cli-tok (canonical CLI source)", creds.AccessToken)
	}
	if config.HasCredential("claude", "oauth") {
		t.Error("orphan claude/oauth slot should have been deleted")
	}
}

func TestLoadCredentials_DeletesOrphanSlotWhenKeychainPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())

	old := readKeychainSecret
	t.Cleanup(func() { readKeychainSecret = old })
	readKeychainSecret = func(string, string) (string, error) {
		return `{"claudeAiOauth":{"accessToken":"kc-tok","refreshToken":"kc-ref","expiresAt":4102444800000}}`, nil
	}

	if err := config.WriteCredential("claude", "oauth", []byte(`{"access_token":"stale"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	s := OAuthStrategy{}
	creds := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() = nil, want keychain creds")
	}
	if creds.AccessToken != "kc-tok" {
		t.Errorf("access_token = %q, want kc-tok", creds.AccessToken)
	}
	if config.HasCredential("claude", "oauth") {
		t.Error("orphan claude/oauth slot should have been deleted")
	}
}

func TestLoadCredentials_NoCanonicalSource_PreservesOrphan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	stubKeychainEmpty(t)

	if err := config.WriteCredential("claude", "oauth", []byte(`{"access_token":"stale"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	s := OAuthStrategy{}
	if creds := s.loadCredentials(); creds != nil {
		t.Errorf("loadCredentials() = %+v, want nil (no canonical source)", creds)
	}
	if !config.HasCredential("claude", "oauth") {
		t.Error("orphan should not be deleted when no canonical source is found")
	}
}
