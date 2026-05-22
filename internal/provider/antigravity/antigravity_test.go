package antigravity

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func TestMeta(t *testing.T) {
	a := Antigravity{}
	meta := a.Meta()

	if meta.ID != "antigravity" {
		t.Errorf("ID = %q, want %q", meta.ID, "antigravity")
	}
	if meta.Name != "Antigravity" {
		t.Errorf("Name = %q, want %q", meta.Name, "Antigravity")
	}
}

func TestFetchStrategies(t *testing.T) {
	a := Antigravity{}
	strategies := a.FetchStrategies()

	if len(strategies) != 1 {
		t.Fatalf("len(strategies) = %d, want 1", len(strategies))
	}
}

func TestParseModelsResponse_FullResponse(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-2.5-pro": {
				DisplayName: "Gemini 2.5 Pro",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.75),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.5),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &TierInfo{ID: "pro-tier", Name: "Google AI Pro"},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Provider != "antigravity" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "antigravity")
	}
	if snapshot.Source != "oauth" {
		t.Errorf("source = %q, want %q", snapshot.Source, "oauth")
	}

	// 1 summary + 2 model periods
	if len(snapshot.Periods) != 3 {
		t.Fatalf("len(periods) = %d, want 3", len(snapshot.Periods))
	}

	// First period is the summary (no model)
	summary := snapshot.Periods[0]
	if summary.Model != "" {
		t.Errorf("summary model = %q, want empty", summary.Model)
	}
	if summary.Utilization != 50 {
		t.Errorf("summary utilization = %d, want 50 (highest of 25 and 50)", summary.Utilization)
	}
	if summary.PeriodType != models.PeriodSession {
		t.Errorf("summary period_type = %q, want %q", summary.PeriodType, models.PeriodSession)
	}
	if summary.Name != "Session (5h)" {
		t.Errorf("summary name = %q, want %q", summary.Name, "Session (5h)")
	}

	// Model periods are sorted by model name
	p1 := snapshot.Periods[1]
	if p1.Model != "gemini-2.5-pro" {
		t.Errorf("period[1] model = %q, want %q", p1.Model, "gemini-2.5-pro")
	}
	if p1.Utilization != 25 {
		t.Errorf("period[1] utilization = %d, want 25", p1.Utilization)
	}
	if p1.ResetsAt == nil {
		t.Fatal("expected resets_at")
	}

	p2 := snapshot.Periods[2]
	if p2.Utilization != 50 {
		t.Errorf("period[2] utilization = %d, want 50", p2.Utilization)
	}

	if snapshot.Identity == nil {
		t.Fatal("expected identity")
	}
	if snapshot.Identity.Plan != "Google AI Pro" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Google AI Pro")
	}
}

func TestParseModelsResponse_FreeTier(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.9),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}
	codeAssist := &CodeAssistResponse{
		CurrentTier: &TierInfo{ID: "free-tier", Name: "Antigravity"},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, codeAssist)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (free tier)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseModelsResponse_SkipsModelsWithoutResetTime(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(0.5),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
			"tab_flash_lite_preview": {
				// No display name, no reset time — tab completion model
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(1.0),
				},
			},
			"chat_20706": {
				// No quota info at all
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	// 1 summary + 1 model period (skipped 2 without reset time)
	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2 (summary + 1 model)", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Model != "" {
		t.Errorf("period[0] should be summary (no model), got %q", snapshot.Periods[0].Model)
	}
	if snapshot.Periods[1].Model != "gemini-3-flash" {
		t.Errorf("period[1] model = %q, want %q", snapshot.Periods[1].Model, "gemini-3-flash")
	}
}

func TestParseModelsResponse_NoTier(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo: &QuotaInfo{
					RemainingFraction: ptrFloat64(1.0),
					ResetTime:         "2026-02-20T05:00:00Z",
				},
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Identity != nil {
		t.Error("expected nil identity when no code assist response")
	}

	p := snapshot.Periods[0]
	if p.PeriodType != models.PeriodWeekly {
		t.Errorf("period_type = %q, want %q (no tier defaults to weekly)", p.PeriodType, models.PeriodWeekly)
	}
}

func TestParseModelsResponse_EmptyModels(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot (fallback period)")
	}
	// 1 fallback "Usage" period + 1 summary prepended
	if len(snapshot.Periods) != 2 {
		t.Fatalf("len(periods) = %d, want 2", len(snapshot.Periods))
	}
	// Summary comes first
	if snapshot.Periods[0].Name != "Weekly" {
		t.Errorf("period[0] name = %q, want %q", snapshot.Periods[0].Name, "Weekly")
	}
	if snapshot.Periods[1].Name != "Usage" {
		t.Errorf("period[1] name = %q, want %q", snapshot.Periods[1].Name, "Usage")
	}
}

func TestParseModelsResponse_SortedByModelName(t *testing.T) {
	modelsResp := FetchAvailableModelsResponse{
		Models: map[string]ModelInfo{
			"gemini-3-flash": {
				DisplayName: "Gemini 3 Flash",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
			"claude-sonnet-4-6": {
				DisplayName: "Claude Sonnet 4.6",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
			"gemini-2.5-pro": {
				DisplayName: "Gemini 2.5 Pro",
				QuotaInfo:   &QuotaInfo{RemainingFraction: ptrFloat64(1.0), ResetTime: "2026-02-20T05:00:00Z"},
			},
		},
	}

	s := OAuthStrategy{}
	snapshot := s.parseModelsResponse(modelsResp, nil)

	// 1 summary + 3 model periods
	if len(snapshot.Periods) != 4 {
		t.Fatalf("len(periods) = %d, want 4", len(snapshot.Periods))
	}
	// First is summary
	if snapshot.Periods[0].Model != "" {
		t.Errorf("period[0] should be summary, got model %q", snapshot.Periods[0].Model)
	}
	// Model periods sorted
	if snapshot.Periods[1].Model != "claude-sonnet-4-6" {
		t.Errorf("period[1] model = %q, want %q", snapshot.Periods[1].Model, "claude-sonnet-4-6")
	}
	if snapshot.Periods[2].Model != "gemini-2.5-pro" {
		t.Errorf("period[2] model = %q, want %q", snapshot.Periods[2].Model, "gemini-2.5-pro")
	}
	if snapshot.Periods[3].Model != "gemini-3-flash" {
		t.Errorf("period[3] model = %q, want %q", snapshot.Periods[3].Model, "gemini-3-flash")
	}
}

func TestPeriodTypeForTier(t *testing.T) {
	tests := []struct {
		tier string
		want models.PeriodType
	}{
		{"", models.PeriodWeekly},
		{"free", models.PeriodWeekly},
		{"free-tier", models.PeriodWeekly},
		{"Antigravity", models.PeriodWeekly},
		{"Google AI Pro", models.PeriodSession},
		{"Google AI Ultra", models.PeriodSession},
		{"g1-pro-tier", models.PeriodSession},
		{"premium", models.PeriodSession},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := periodTypeForTier(tt.tier)
			if got != tt.want {
				t.Errorf("periodTypeForTier(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

// applyAntigravityTestEnv isolates os.UserConfigDir(), os.UserHomeDir() and
// vibeusage's config dir to fresh temp dirs so external paths and the vscdb
// lookup never resolve to real user data.
func applyAntigravityTestEnv(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	return cfg
}

func writeAntigravityFileCreds(t *testing.T, configDir string) {
	t.Helper()
	dir := filepath.Join(configDir, "Antigravity")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"access_token":"file-tok","refresh_token":"file-ref","expires_at":"2099-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// writeAntigravityOwnedSlot writes a slot credential with the
// vibeusage_owned marker, simulating creds minted via `vibeusage auth
// antigravity`.
func writeAntigravityOwnedSlot(t *testing.T) {
	t.Helper()
	content := `{"access_token":"slot-tok","refresh_token":"slot-ref","expires_at":"2099-01-01T00:00:00Z","vibeusage_owned":true}`
	if err := config.WriteCredential("antigravity", "oauth", []byte(content)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}
}

// writeAntigravityUnmarkedSlot writes a slot credential without the
// vibeusage_owned marker, simulating buggy-era residue from the pre-fix
// refresh path.
func writeAntigravityUnmarkedSlot(t *testing.T) {
	t.Helper()
	content := `{"access_token":"slot-tok","refresh_token":"slot-ref","expires_at":"2099-01-01T00:00:00Z"}`
	if err := config.WriteCredential("antigravity", "oauth", []byte(content)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}
}

func TestLoadCredentials_OwnedSlotWinsOverFile(t *testing.T) {
	cfg := applyAntigravityTestEnv(t)
	writeAntigravityFileCreds(t, cfg)
	writeAntigravityOwnedSlot(t)

	s := &OAuthStrategy{}
	creds, source := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() returned nil creds")
	}
	if source != sourceSlot {
		t.Errorf("source = %v, want sourceSlot", source)
	}
	if creds.AccessToken != "slot-tok" {
		t.Errorf("access_token = %q, want slot-tok", creds.AccessToken)
	}
}

func TestLoadCredentials_UnmarkedSlotMigrates_FilePresent(t *testing.T) {
	cfg := applyAntigravityTestEnv(t)
	writeAntigravityFileCreds(t, cfg)
	writeAntigravityUnmarkedSlot(t)

	s := &OAuthStrategy{}
	creds, source := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() returned nil creds")
	}
	if source != sourceFile {
		t.Errorf("source = %v, want sourceFile (unmarked slot is buggy-era residue)", source)
	}
	if creds.AccessToken != "file-tok" {
		t.Errorf("access_token = %q, want file-tok (canonical source)", creds.AccessToken)
	}
	if config.HasCredential("antigravity", "oauth") {
		t.Error("unmarked slot should be deleted when canonical source is found")
	}
}

func TestLoadCredentials_UnmarkedSlotSurfacesAsBuggyResidue(t *testing.T) {
	applyAntigravityTestEnv(t)
	writeAntigravityUnmarkedSlot(t)

	s := &OAuthStrategy{}
	creds, source := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() returned nil creds")
	}
	if source != sourceUnmarkedSlot {
		t.Errorf("source = %v, want sourceUnmarkedSlot (buggy-era residue must be distinguishable from legitimate piggyback)", source)
	}
	if creds.AccessToken != "slot-tok" {
		t.Errorf("access_token = %q, want slot-tok", creds.AccessToken)
	}
	if !config.HasCredential("antigravity", "oauth") {
		t.Error("unmarked slot should be preserved when there is no canonical replacement (Fetch surfaces a re-auth message instead of deleting blindly)")
	}
}

func TestFetch_FailsClosedForUnmarkedSlot(t *testing.T) {
	applyAntigravityTestEnv(t)
	writeAntigravityUnmarkedSlot(t)

	s := &OAuthStrategy{}
	res, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Success {
		t.Fatal("Fetch should fail for an unmarked slot — the chain forked when the buggy refresh wrote the rotated RT")
	}
	if res.ShouldFallback {
		t.Error("Fetch should be fatal (no fallback) so verification during `vibeusage auth` cannot accept stale piggyback creds")
	}
	if !strings.Contains(res.Error, "vibeusage auth antigravity") {
		t.Errorf("Fetch error = %q, want guidance to re-auth", res.Error)
	}
}

func TestLoadCredentials_FileSourceWhenNoSlot(t *testing.T) {
	cfg := applyAntigravityTestEnv(t)
	writeAntigravityFileCreds(t, cfg)

	s := &OAuthStrategy{}
	creds, source := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() returned nil creds")
	}
	if source != sourceFile {
		t.Errorf("source = %v, want sourceFile", source)
	}
	if creds.AccessToken != "file-tok" {
		t.Errorf("access_token = %q, want file-tok", creds.AccessToken)
	}
}

func TestLoadCredentials_NoneWhenNothingPresent(t *testing.T) {
	applyAntigravityTestEnv(t)

	s := &OAuthStrategy{}
	creds, source := s.loadCredentials()
	if creds != nil {
		t.Errorf("creds = %+v, want nil", creds)
	}
	if source != sourceNone {
		t.Errorf("source = %v, want sourceNone", source)
	}
}

func TestSaveAntigravityCredentials_RoundTripsAsOwnedSlot(t *testing.T) {
	applyAntigravityTestEnv(t)

	in := &oauth.Credentials{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    "2099-01-01T00:00:00Z",
	}
	if err := saveAntigravityCredentials(in); err != nil {
		t.Fatalf("saveAntigravityCredentials: %v", err)
	}

	s := &OAuthStrategy{}
	got, source := s.loadCredentials()
	if got == nil {
		t.Fatal("loadCredentials() = nil after save")
	}
	if source != sourceSlot {
		t.Errorf("source = %v, want sourceSlot (saveAntigravityCredentials must mark as vibeusage-owned)", source)
	}
	if got.AccessToken != in.AccessToken {
		t.Errorf("access_token = %q, want %q", got.AccessToken, in.AccessToken)
	}
	if got.RefreshToken != in.RefreshToken {
		t.Errorf("refresh_token = %q, want %q", got.RefreshToken, in.RefreshToken)
	}

	// Verify the marker landed on disk.
	data, err := config.ReadCredential("antigravity", "oauth")
	if err != nil || data == nil {
		t.Fatalf("ReadCredential after save: data=%q err=%v", data, err)
	}
	if !strings.Contains(string(data), `"vibeusage_owned":true`) {
		t.Errorf("persisted slot missing vibeusage_owned marker: %s", data)
	}
}

func TestFetch_FailsClosedForSourceFileWhenRefreshNeeded(t *testing.T) {
	cfg := applyAntigravityTestEnv(t)

	// Write a sourceFile credential with an expired access token so
	// NeedsRefresh() triggers the refresh branch.
	expired := "2020-01-01T00:00:00Z"
	dir := filepath.Join(cfg, "Antigravity")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"access_token":"file-tok","refresh_token":"file-ref","expires_at":"` + expired + `"}`
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := &OAuthStrategy{}
	res, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Success {
		t.Fatal("Fetch should fail when sourceFile creds need refresh")
	}
	if !strings.Contains(res.Error, "Antigravity IDE") {
		t.Errorf("Fetch error = %q, want guidance to use the IDE", res.Error)
	}
}
