package antigravity

import (
	"os"
	"path/filepath"
	"testing"

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
	if strategies[0].Name() != "oauth" {
		t.Errorf("strategy name = %q, want %q", strategies[0].Name(), "oauth")
	}
}

func TestOAuthStrategy_CredentialPaths_RespectsReuseProviderCredentials(t *testing.T) {
	dir := t.TempDir()
	testenv.ApplyVibeusage(t.Setenv, dir)
	t.Setenv("HOME", filepath.Join(dir, "home"))
	// os.UserConfigDir on linux uses XDG_CONFIG_HOME before HOME/.config.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "home", ".config"))

	cfg := config.DefaultConfig()
	cfg.Credentials.ReuseProviderCredentials = false
	config.Override(t, cfg)

	externalPath := filepath.Join(dir, "home", ".config", "Antigravity", "credentials.json")
	if err := os.MkdirAll(filepath.Dir(externalPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(externalPath, []byte(`{"apiKey":"tok"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := &OAuthStrategy{}
	paths := s.credentialPaths()
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	if paths[0] != config.CredentialPath("antigravity", "oauth") {
		t.Errorf("paths[0] = %q, want %q", paths[0], config.CredentialPath("antigravity", "oauth"))
	}
	if s.IsAvailable() {
		t.Fatal("IsAvailable() = true, want false when only provider CLI credentials exist")
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
