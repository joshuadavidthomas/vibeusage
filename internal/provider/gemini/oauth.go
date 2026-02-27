package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/googleauth"
)

const (
	// OAuth client credentials extracted from the Gemini CLI installation.
	// Required to refresh tokens stored in ~/.gemini/oauth_creds.json.
	geminiClientID     = "77185425430.apps.googleusercontent.com"
	geminiClientSecret = "GOCSPX-1mdrl61JR9D-iFHq4QPq2mJGwZv"

	quotaURL      = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	codeAssistURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
)

// OAuthStrategy fetches Gemini usage using OAuth credentials.
type OAuthStrategy struct {
	HTTPTimeout float64
}

func (s *OAuthStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func (s *OAuthStrategy) credentialPaths() []string {
	home, _ := os.UserHomeDir()
	return provider.CredentialSearchPaths("gemini", "oauth", filepath.Join(home, ".gemini", "oauth_creds.json"))
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
			ClientID:     geminiClientID,
			ClientSecret: geminiClientSecret,
			ProviderID:   "gemini",
			HTTPTimeout:  s.HTTPTimeout,
		})
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token"), nil
		}
		creds = refreshed
	}

	quotaResp, codeAssistResp, fetchErr := s.fetchQuotaData(ctx, creds.AccessToken)
	if quotaResp == nil {
		if fetchErr.authFailed {
			return fetch.ResultFail("Token expired or invalid. Run `vibeusage auth gemini` to re-authenticate."), nil
		}
		return fetch.ResultFail(fmt.Sprintf("Failed to fetch quota data: %s", fetchErr.message)), nil
	}

	snapshot := s.parseTypedQuotaResponse(*quotaResp, codeAssistResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *OAuthStrategy) loadCredentials() *oauth.Credentials {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}
		var cliCreds GeminiCLICredentials
		if err := json.Unmarshal(data, &cliCreds); err != nil {
			continue
		}
		if creds := cliCreds.EffectiveCredentials(); creds != nil {
			return creds
		}
	}
	return nil
}

type fetchError struct {
	message    string
	authFailed bool
}

func (e fetchError) String() string { return e.message }

func (s *OAuthStrategy) fetchQuotaData(ctx context.Context, accessToken string) (*QuotaResponse, *CodeAssistResponse, fetchError) {
	client := httpclient.NewFromConfig(s.HTTPTimeout)
	bearer := httpclient.WithBearer(accessToken)
	var quotaResp *QuotaResponse
	var codeAssistResp *CodeAssistResponse
	var quotaErr fetchError

	// Quota
	var qr QuotaResponse
	qResp, err := client.PostJSONCtx(ctx, quotaURL,
		json.RawMessage("{}"), &qr, bearer,
	)
	if err != nil {
		quotaErr = fetchError{message: fmt.Sprintf("request failed: %v", err)}
	} else if qResp.StatusCode == 401 || qResp.StatusCode == 403 {
		quotaErr = fetchError{message: fmt.Sprintf("HTTP %d", qResp.StatusCode), authFailed: true}
	} else if qResp.StatusCode != 200 {
		quotaErr = fetchError{message: fmt.Sprintf("HTTP %d: %s", qResp.StatusCode, googleauth.ExtractAPIError(qResp.Body))}
	} else if qResp.JSONErr != nil {
		quotaErr = fetchError{message: fmt.Sprintf("invalid response: %v", qResp.JSONErr)}
	} else {
		quotaResp = &qr
	}

	// User tier (non-fatal if it fails)
	var ca CodeAssistResponse
	tResp, err := client.PostJSONCtx(ctx, codeAssistURL,
		json.RawMessage("{}"), &ca, bearer,
	)
	if err == nil && tResp.StatusCode == 200 && tResp.JSONErr == nil {
		codeAssistResp = &ca
	}

	return quotaResp, codeAssistResp, quotaErr
}

func (s *OAuthStrategy) parseTypedQuotaResponse(quotaResp QuotaResponse, codeAssistResp *CodeAssistResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	for _, bucket := range quotaResp.QuotaBuckets {
		modelName := bucket.ModelID
		if idx := strings.LastIndex(bucket.ModelID, "/"); idx >= 0 {
			modelName = bucket.ModelID[idx+1:]
		}

		displayName := titleCase(strings.ReplaceAll(strings.ReplaceAll(modelName, "-", " "), "_", " "))
		periods = append(periods, models.UsagePeriod{
			Name:        displayName,
			Utilization: bucket.Utilization(),
			PeriodType:  models.PeriodDaily,
			ResetsAt:    bucket.ResetTimeUTC(),
			Model:       modelName,
		})
	}

	if len(periods) == 0 {
		tomorrow := nextMidnightUTC()
		periods = append(periods, models.UsagePeriod{
			Name:        "Daily",
			Utilization: 0,
			PeriodType:  models.PeriodDaily,
			ResetsAt:    &tomorrow,
		})
	}

	var identity *models.ProviderIdentity
	if codeAssistResp != nil && codeAssistResp.UserTier != "" {
		identity = &models.ProviderIdentity{Plan: codeAssistResp.UserTier}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "gemini",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    "oauth",
	}
}
