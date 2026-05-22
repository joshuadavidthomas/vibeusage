package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/google"
	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Antigravity struct{}

func (a Antigravity) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "antigravity",
		Name:         "Antigravity",
		Description:  "Google Antigravity AI IDE",
		Homepage:     "https://antigravity.google",
		DashboardURL: "https://one.google.com/ai",
	}
}

func (a Antigravity) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.config/Antigravity/credentials.json"},
		EnvVars:  []string{"ANTIGRAVITY_API_KEY"},
	}
}

func (a Antigravity) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
	}
}

func (a Antigravity) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchGoogleAppsStatus(ctx, []string{
		"antigravity", "gemini", "cloud code", "generative ai", "ai studio",
	})
}

// Auth returns the OAuth browser flow for Antigravity.
func (a Antigravity) Auth() provider.AuthFlow {
	return provider.CustomAuthFlow{RunFlow: RunAuthFlow}
}

func init() {
	provider.Register(Antigravity{})
}

const (
	// OAuth client credentials from the Antigravity IDE.
	antigravityClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret = "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"

	fetchModelsURL       = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	loadCodeAssistURL    = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	antigravityUserAgent = "antigravity"
)

// OAuthStrategy fetches Antigravity quota using Google OAuth credentials.
type OAuthStrategy struct {
	HTTPTimeout float64
	vscdb       *vscdbResult // cached vscdb data (credentials + subscription)
}

func (s *OAuthStrategy) IsAvailable() bool {
	if config.HasCredential("antigravity", "oauth") {
		return true
	}
	for _, p := range s.externalPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	// Check if Antigravity's VS Code state database exists
	if _, err := os.Stat(vscdbPath()); err == nil {
		return true
	}
	return false
}

func (s *OAuthStrategy) externalPaths() []string {
	var paths []string
	if configDir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(configDir, "Antigravity", "credentials.json"))
	}
	return paths
}

// vscdbPath returns the path to Antigravity's VS Code state database.
func vscdbPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "Antigravity", "User", "globalStorage", "state.vscdb")
}

func (s *OAuthStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	creds, source := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found"), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	// Buggy-era residue: vibeusage's slot was populated by the pre-fix
	// refresh path (no vibeusage_owned marker) and there is no canonical
	// IDE source to migrate to. The chain forked at the moment of that
	// rotated-RT write, so we can't refresh and can't trust the access
	// token long-term. Fail closed so the user re-auths cleanly.
	if source == sourceUnmarkedSlot {
		return fetch.ResultFatal("Stale Antigravity credentials detected. Run `vibeusage auth antigravity` to re-authenticate."), nil
	}

	if creds.NeedsRefresh() {
		// Only refresh against Google's token endpoint when vibeusage owns the
		// chain (i.e. the user explicitly minted credentials via `vibeusage
		// auth antigravity`). For creds piggy-backed off the IDE's file or
		// vscdb, the refresh request itself can rotate the refresh token
		// server-side — invalidating the IDE's stored copy and breaking its
		// next refresh. Fail closed and let the user re-authenticate via
		// the IDE, which will refresh on its own terms.
		if source != sourceSlot {
			return fetch.ResultFail("Token expired. Sign back in via the Antigravity IDE to refresh credentials, or run `vibeusage auth antigravity` for a vibeusage-owned chain."), nil
		}
		refreshed := google.RefreshToken(ctx, creds, google.RefreshConfig{
			ClientID:     antigravityClientID,
			ClientSecret: antigravityClientSecret,
			Save:         saveAntigravityCredentials,
			HTTPTimeout:  s.HTTPTimeout,
		})
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
	}

	modelsResp, codeAssistResp, fetchErr := s.fetchQuotaData(ctx, creds.AccessToken)
	if modelsResp == nil {
		if fetchErr.authFailed {
			return fetch.ResultFail("Token expired or invalid. Sign in again in the Antigravity IDE, or run `vibeusage auth antigravity`."), nil
		}
		return fetch.ResultFail(fmt.Sprintf("Failed to fetch quota data: %s", fetchErr.message)), nil
	}

	snapshot := s.parseModelsResponse(*modelsResp, codeAssistResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

// credSource identifies which storage location supplied the loaded credentials.
// Vibeusage owns the rotating chain only when the slot was the source (i.e.
// the user minted credentials via `vibeusage auth antigravity`); for the
// IDE-managed file and vscdb sources, vibeusage piggy-backs and must not
// persist rotated tokens. sourceUnmarkedSlot is a buggy-era residue that
// can no longer be safely refreshed and is treated as a hard failure.
type credSource int

const (
	sourceNone credSource = iota
	sourceSlot
	sourceFile
	sourceVSCDB
	sourceUnmarkedSlot
)

func (s *OAuthStrategy) loadCredentials() (*oauth.Credentials, credSource) {
	// Read the vibeusage slot first, but distinguish legitimately-minted
	// credentials (carrying the vibeusage_owned marker, written by
	// RunAuthFlow / saveAntigravityCredentials) from buggy-era residue (the
	// old refresh path persisted rotated tokens here without a marker).
	slotCreds, slotOwned := s.loadSlotCredentials()
	if slotOwned {
		return slotCreds, sourceSlot
	}

	// Check external CLI paths (IDE's credentials file).
	for _, path := range s.externalPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if creds := parseAntigravityCredentials(data); creds != nil {
			if slotCreds != nil {
				config.DeleteCredential("antigravity", "oauth")
			}
			return creds, sourceFile
		}
	}

	// Try reading from Antigravity's VS Code state database.
	if result := loadFromVSCDB(); result != nil {
		s.vscdb = result
		if slotCreds != nil {
			config.DeleteCredential("antigravity", "oauth")
		}
		return result.creds, sourceVSCDB
	}

	// No canonical source. If we still have an unmarked slot, surface it
	// as sourceUnmarkedSlot so callers can recognise buggy-era residue and
	// force a re-auth instead of refreshing it against Google or treating
	// it as legitimate detected credentials.
	if slotCreds != nil {
		return slotCreds, sourceUnmarkedSlot
	}

	return nil, sourceNone
}

// loadSlotCredentials reads vibeusage's antigravity/oauth slot and reports
// whether it carries the vibeusage_owned marker. Unmarked slots predate the
// owned-chain marker (or were written by the old buggy refresh path) and are
// not safe to refresh against Google's token endpoint.
func (s *OAuthStrategy) loadSlotCredentials() (*oauth.Credentials, bool) {
	data, err := config.ReadCredential("antigravity", "oauth")
	if err != nil || data == nil {
		return nil, false
	}
	var sc slotCredentials
	if err := json.Unmarshal(data, &sc); err != nil || sc.AccessToken == "" {
		return nil, false
	}
	return &oauth.Credentials{
		AccessToken:  sc.AccessToken,
		RefreshToken: sc.RefreshToken,
		ExpiresAt:    sc.ExpiresAt,
	}, sc.VibeusageOwned
}

// slotCredentials is the on-disk shape for vibeusage's antigravity/oauth slot.
// VibeusageOwned distinguishes credentials minted via `vibeusage auth
// antigravity` (refreshable here) from stale credentials written by the
// pre-fix refresh path (must not be refreshed here).
type slotCredentials struct {
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	VibeusageOwned bool   `json:"vibeusage_owned,omitempty"`
}

func saveAntigravityCredentials(c *oauth.Credentials) error {
	sc := slotCredentials{
		AccessToken:    c.AccessToken,
		RefreshToken:   c.RefreshToken,
		ExpiresAt:      c.ExpiresAt,
		VibeusageOwned: true,
	}
	content, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshal antigravity credentials: %w", err)
	}
	return config.WriteCredential("antigravity", "oauth", content)
}

func parseAntigravityCredentials(data []byte) *oauth.Credentials {
	// Try the Antigravity credential format
	var agCreds AntigravityCredentials
	if err := json.Unmarshal(data, &agCreds); err == nil {
		if creds := agCreds.ToOAuthCredentials(); creds != nil {
			return creds
		}
	}

	// Try direct OAuth credentials format
	var oauthCreds oauth.Credentials
	if err := json.Unmarshal(data, &oauthCreds); err == nil && oauthCreds.AccessToken != "" {
		return &oauthCreds
	}
	return nil
}

// vscdbResult holds credentials and subscription info read from the vscdb.
type vscdbResult struct {
	creds        *oauth.Credentials
	subscription *SubscriptionInfo
	email        string
}

// loadFromVSCDB reads OAuth credentials and subscription info from
// Antigravity's VS Code state database (state.vscdb). Antigravity stores
// auth status as a JSON blob in a SQLite database under the
// "antigravityAuthStatus" key. The apiKey field contains a Google OAuth
// access token (ya29.*), and the userStatusProtoBinaryBase64 field contains
// a protobuf blob with subscription tier info.
func loadFromVSCDB() *vscdbResult {
	dbPath := vscdbPath()
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	// Use sqlite3 CLI to read the auth status — avoids adding a heavy
	// SQLite library dependency for reading a single key.
	out, err := exec.Command(
		"sqlite3", dbPath,
		"SELECT value FROM ItemTable WHERE key = 'antigravityAuthStatus';",
	).Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	var authStatus VscdbAuthStatus
	if err := json.Unmarshal(out, &authStatus); err != nil {
		return nil
	}

	if authStatus.APIKey == "" {
		return nil
	}

	result := &vscdbResult{
		creds: &oauth.Credentials{
			AccessToken: authStatus.APIKey,
			// No refresh token available from the vscdb — the Antigravity
			// IDE manages token refresh internally.
		},
		email:        authStatus.Email,
		subscription: parseSubscriptionFromProto(authStatus.UserStatusProtoBinaryBase64),
	}

	return result
}

type fetchError struct {
	message    string
	authFailed bool
}

func (e fetchError) String() string { return e.message }

func (s *OAuthStrategy) fetchQuotaData(ctx context.Context, accessToken string) (*FetchAvailableModelsResponse, *CodeAssistResponse, fetchError) {
	client := httpclient.NewFromConfig(s.HTTPTimeout)
	bearer := httpclient.WithBearer(accessToken)
	ua := httpclient.WithHeader("User-Agent", antigravityUserAgent)
	var modelsResp *FetchAvailableModelsResponse
	var codeAssistResp *CodeAssistResponse
	var modelsErr fetchError

	// Fetch available models (includes per-model quota info)
	var mr FetchAvailableModelsResponse
	mResp, err := client.PostJSONCtx(ctx, fetchModelsURL,
		json.RawMessage("{}"), &mr, bearer, ua,
	)
	if err != nil {
		modelsErr = fetchError{message: fmt.Sprintf("request failed: %v", err)}
	} else if mResp.StatusCode == 401 || mResp.StatusCode == 403 {
		modelsErr = fetchError{message: fmt.Sprintf("HTTP %d", mResp.StatusCode), authFailed: true}
	} else if mResp.StatusCode != 200 {
		modelsErr = fetchError{message: fmt.Sprintf("HTTP %d: %s", mResp.StatusCode, google.ExtractAPIError(mResp.Body))}
	} else if mResp.JSONErr != nil {
		modelsErr = fetchError{message: fmt.Sprintf("invalid response: %v", mResp.JSONErr)}
	} else {
		modelsResp = &mr
	}

	// User tier — requires IDE metadata (non-fatal if it fails)
	reqBody := CodeAssistRequest{
		Metadata: &CodeAssistRequestMetadata{
			IDEType:    "ANTIGRAVITY",
			Platform:   "PLATFORM_UNSPECIFIED",
			PluginType: "GEMINI",
		},
	}
	var ca CodeAssistResponse
	tResp, err := client.PostJSONCtx(ctx, loadCodeAssistURL,
		reqBody, &ca, bearer, ua,
	)
	if err == nil && tResp.StatusCode == 200 && tResp.JSONErr == nil {
		codeAssistResp = &ca
	}

	return modelsResp, codeAssistResp, modelsErr
}

// periodTypeForTier returns the appropriate period type based on the user's plan.
// Paid tiers (Pro/Ultra) use 5-hour sessions; free tier uses weekly cycles.
func periodTypeForTier(tier string) models.PeriodType {
	lower := strings.ToLower(tier)
	switch {
	case strings.Contains(lower, "pro"),
		strings.Contains(lower, "ultra"),
		strings.Contains(lower, "premium"):
		return models.PeriodSession
	default:
		return models.PeriodWeekly
	}
}

func (s *OAuthStrategy) parseModelsResponse(modelsResp FetchAvailableModelsResponse, codeAssistResp *CodeAssistResponse) *models.UsageSnapshot {
	// Determine plan name: prefer subscription info from protobuf (accurate),
	// fall back to loadCodeAssist response (may show product name, not subscription).
	tier := ""
	if s.vscdb != nil && s.vscdb.subscription != nil {
		tier = s.vscdb.subscription.TierName
	}
	if tier == "" {
		tier = codeAssistResp.EffectiveTier()
	}
	periodType := periodTypeForTier(tier)

	var periods []models.UsagePeriod

	for modelID, info := range modelsResp.Models {
		if info.QuotaInfo == nil || info.QuotaInfo.ResetTime == "" {
			continue // skip models without quota tracking (tab completion, internal, etc.)
		}

		displayName := info.DisplayName
		if displayName == "" {
			displayName = titleCase(strings.ReplaceAll(strings.ReplaceAll(modelID, "-", " "), "_", " "))
		}

		periods = append(periods, models.UsagePeriod{
			Name:        displayName,
			Utilization: info.QuotaInfo.Utilization(),
			PeriodType:  periodType,
			ResetsAt:    info.QuotaInfo.ResetTimeUTC(),
			Model:       modelID,
		})
	}

	// Sort periods by model name for stable output
	sort.Slice(periods, func(i, j int) bool {
		return periods[i].Model < periods[j].Model
	})

	if len(periods) == 0 {
		periods = append(periods, models.UsagePeriod{
			Name:        "Usage",
			Utilization: 0,
			PeriodType:  periodType,
		})
	}

	// Add a summary period (Model == "") for the compact panel view.
	// Uses the highest utilization across all models — shows worst case.
	summary := summarizePeriods(periods, periodType)
	periods = append([]models.UsagePeriod{summary}, periods...)

	var identity *models.ProviderIdentity
	if tier != "" {
		identity = &models.ProviderIdentity{Plan: tier}
		if s.vscdb != nil && s.vscdb.email != "" {
			identity.Email = s.vscdb.email
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "antigravity",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    "oauth",
	}
}

// summarizePeriods creates a summary period from per-model periods.
// It picks the highest utilization (worst case) and the earliest reset time.
func summarizePeriods(periods []models.UsagePeriod, periodType models.PeriodType) models.UsagePeriod {
	name := "Session (5h)"
	if periodType == models.PeriodWeekly {
		name = "Weekly"
	}

	summary := models.UsagePeriod{
		Name:       name,
		PeriodType: periodType,
	}

	for _, p := range periods {
		if p.Utilization > summary.Utilization {
			summary.Utilization = p.Utilization
		}
		if p.ResetsAt != nil {
			if summary.ResetsAt == nil || p.ResetsAt.Before(*summary.ResetsAt) {
				summary.ResetsAt = p.ResetsAt
			}
		}
	}

	return summary
}

// titleCase capitalizes the first letter of each space-separated word.
// Used for formatting model display names (e.g. "claude-3-5-sonnet" → "Claude 3 5 Sonnet").
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
