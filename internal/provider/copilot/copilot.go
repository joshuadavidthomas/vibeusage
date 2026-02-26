package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/deviceflow"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Copilot struct{}

func (c Copilot) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "copilot",
		Name:         "Copilot",
		Description:  "GitHub's AI pair programmer",
		Homepage:     "https://github.com/features/copilot",
		StatusURL:    "https://www.githubstatus.com",
		DashboardURL: "https://github.com/settings/copilot",
	}
}

func (c Copilot) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.config/github-copilot/hosts.json"},
		EnvVars:  []string{"GITHUB_TOKEN"},
	}
}

func (c Copilot) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&DeviceFlowStrategy{HTTPTimeout: timeout}}
}

func (c Copilot) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://www.githubstatus.com/api/v2/status.json")
}

// Auth returns the GitHub device flow for Copilot.
func (c Copilot) Auth() provider.AuthFlow {
	return provider.DeviceAuthFlow{
		Config: deviceflow.Config{
			DeviceCodeURL: deviceCodeURL,
			DeviceCodeParams: map[string]string{
				"client_id": clientID,
				"scope":     "read:user",
			},
			TokenURL: tokenURL,
			TokenParams: map[string]string{
				"client_id":  clientID,
				"grant_type": "urn:ietf:params:oauth:grant-type:device_code",
			},
			HTTPOptions:     []httpclient.RequestOption{httpclient.WithHeader("Accept", "application/json")},
			HTTPTimeout:     config.Get().Fetch.Timeout,
			ManualCodeEntry: true,
			FormatCode: func(code string) string {
				if len(code) == 8 {
					return code[:4] + "-" + code[4:]
				}
				return code
			},
			CredentialPath:  config.CredentialPath("copilot", "oauth"),
			ShowRefreshHint: true,
		},
	}
}

func init() {
	provider.Register(Copilot{})
}

const (
	deviceCodeURL = "https://github.com/login/device/code"
	tokenURL      = "https://github.com/login/oauth/access_token"
	usageURL      = "https://api.github.com/copilot_internal/user"
	clientID      = "Iv1.b507a08c87ecfe98" // VS Code Copilot OAuth app client ID
)

type DeviceFlowStrategy struct {
	HTTPTimeout float64
}

func (s *DeviceFlowStrategy) IsAvailable() bool {
	path := config.CredentialPath("copilot", "oauth")
	_, err := os.Stat(path)
	return err == nil
}

func (s *DeviceFlowStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found. Run `vibeusage auth copilot` to authenticate."), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(ctx, creds)
		if refreshed != nil {
			creds = refreshed
		}
		// If refresh fails, try the existing token anyway — it might still
		// work (e.g. non-expiring token with empty ExpiresAt from a parse error).
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var userResp UserResponse
	resp, err := client.GetJSONCtx(ctx, usageURL, &userResp,
		httpclient.WithBearer(creds.AccessToken),
		httpclient.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		// If we have a refresh token, try once more before giving up
		if creds.RefreshToken != "" {
			refreshed := s.refreshToken(ctx, creds)
			if refreshed != nil {
				resp, err = client.GetJSONCtx(ctx, usageURL, &userResp,
					httpclient.WithBearer(refreshed.AccessToken),
					httpclient.WithHeader("Accept", "application/json"),
				)
				if err == nil && resp.StatusCode == 200 && resp.JSONErr == nil {
					snapshot := s.parseTypedUsageResponse(userResp)
					if snapshot != nil {
						return fetch.ResultOK(*snapshot), nil
					}
				}
			}
		}
		return fetch.ResultFatal("OAuth token expired. Run `vibeusage auth copilot` to re-authenticate."), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFail("Not authorized. Account may not have Copilot subscription."), nil
	}
	if resp.StatusCode == 404 {
		return fetch.ResultFail("Copilot API not found. Account may not have Copilot access."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Usage request failed: %d", resp.StatusCode)), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Copilot API: %v", resp.JSONErr)), nil
	}

	snapshot := s.parseTypedUsageResponse(userResp)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse Copilot usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *DeviceFlowStrategy) loadCredentials() *OAuthCredentials {
	data, err := config.ReadCredential(config.CredentialPath("copilot", "oauth"))
	if err != nil || data == nil {
		return nil
	}
	var creds OAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil
	}
	if creds.AccessToken == "" {
		return nil
	}
	return &creds
}

func (s *DeviceFlowStrategy) refreshToken(ctx context.Context, creds *OAuthCredentials) *OAuthCredentials {
	return oauth.Refresh(ctx, creds.RefreshToken, oauth.RefreshConfig{
		TokenURL:    tokenURL,
		FormFields:  map[string]string{"client_id": clientID},
		Headers:     []httpclient.RequestOption{httpclient.WithHeader("Accept", "application/json")},
		ProviderID:  "copilot",
		HTTPTimeout: s.HTTPTimeout,
	})
}

func (s *DeviceFlowStrategy) parseTypedUsageResponse(resp UserResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	resetsAt := models.ParseRFC3339Ptr(resp.QuotaResetDateUTC)

	if resp.QuotaSnapshots != nil {
		type quotaEntry struct {
			quota *Quota
			name  string
		}
		entries := []quotaEntry{
			{resp.QuotaSnapshots.PremiumInteractions, "Monthly (Premium)"},
			{resp.QuotaSnapshots.Chat, "Monthly (Chat)"},
			{resp.QuotaSnapshots.Completions, "Monthly (Completions)"},
		}
		for _, e := range entries {
			if e.quota != nil && e.quota.HasUsage() {
				periods = append(periods, models.UsagePeriod{
					Name:        e.name,
					Utilization: e.quota.Utilization(),
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    resetsAt,
				})
			}
		}
	}

	if len(periods) == 0 {
		return nil
	}

	var identity *models.ProviderIdentity
	if resp.CopilotPlan != "" {
		identity = &models.ProviderIdentity{Plan: resp.CopilotPlan}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "copilot",
		FetchedAt: now,
		Periods:   periods,
		Identity:  identity,
		Source:    "device_flow",
	}
}
