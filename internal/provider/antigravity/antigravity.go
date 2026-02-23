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

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/googleauth"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
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

func (a Antigravity) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&OAuthStrategy{},
	}
}

func (a Antigravity) FetchStatus() models.ProviderStatus {
	return provider.FetchGoogleAppsStatus([]string{
		"antigravity", "gemini", "cloud code", "generative ai", "ai studio",
	})
}

// Auth returns the OAuth browser flow for Antigravity.
func (a Antigravity) Auth() provider.AuthFlow {
	return provider.DeviceAuthFlow{RunFlow: RunAuthFlow}
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
	vscdb *vscdbResult // cached vscdb data (credentials + subscription)
}

func (s *OAuthStrategy) Name() string { return "oauth" }

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
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

func (s *OAuthStrategy) credentialPaths() []string {
	paths := []string{config.CredentialPath("antigravity", "oauth")}
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
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found"), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := googleauth.RefreshToken(ctx, creds, googleauth.RefreshConfig{
			ClientID:     antigravityClientID,
			ClientSecret: antigravityClientSecret,
			ProviderID:   "antigravity",
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

func (s *OAuthStrategy) loadCredentials() *googleauth.OAuthCredentials {
	// Try JSON credential files first
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}

		// Try the Antigravity credential format
		var agCreds AntigravityCredentials
		if err := json.Unmarshal(data, &agCreds); err == nil {
			if creds := agCreds.ToOAuthCredentials(); creds != nil {
				return creds
			}
		}

		// Try direct OAuth credentials format
		var oauthCreds googleauth.OAuthCredentials
		if err := json.Unmarshal(data, &oauthCreds); err == nil && oauthCreds.AccessToken != "" {
			return &oauthCreds
		}
	}

	// Try reading from Antigravity's VS Code state database
	if result := loadFromVSCDB(); result != nil {
		s.vscdb = result
		return result.creds
	}

	return nil
}

// vscdbResult holds credentials and subscription info read from the vscdb.
type vscdbResult struct {
	creds        *googleauth.OAuthCredentials
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
		creds: &googleauth.OAuthCredentials{
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
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
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
		modelsErr = fetchError{message: fmt.Sprintf("HTTP %d: %s", mResp.StatusCode, googleauth.ExtractAPIError(mResp.Body))}
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
			displayName = strutil.TitleCase(strings.ReplaceAll(strings.ReplaceAll(modelID, "-", " "), "_", " "))
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
