package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
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

func (c Copilot) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{&DeviceFlowStrategy{}}
}

func (c Copilot) FetchStatus() models.ProviderStatus {
	return provider.FetchStatuspageStatus("https://www.githubstatus.com/api/v2/status.json")
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

type DeviceFlowStrategy struct{}

func (s *DeviceFlowStrategy) Name() string { return "device_flow" }

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

	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	var userResp UserResponse
	resp, err := client.GetJSONCtx(ctx, usageURL, &userResp,
		httpclient.WithBearer(creds.AccessToken),
		httpclient.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
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
		return fetch.ResultFail("Invalid response from Copilot API"), nil
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

func (s *DeviceFlowStrategy) parseTypedUsageResponse(resp UserResponse) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	var resetsAt *time.Time
	if resp.QuotaResetDateUTC != "" {
		if t, err := time.Parse(time.RFC3339, resp.QuotaResetDateUTC); err == nil {
			resetsAt = &t
		}
	}

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

// RunDeviceFlow runs the interactive GitHub device flow for auth.
// Output is written to w, allowing callers to control where messages go.
func RunDeviceFlow(w io.Writer, quiet bool) (bool, error) {
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)

	// Request device code
	var dcResp DeviceCodeResponse
	resp, err := client.PostForm(deviceCodeURL,
		map[string]string{
			"client_id": clientID,
			"scope":     "read:user",
		},
		&dcResp,
		httpclient.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return false, fmt.Errorf("failed to request device code: %w", err)
	}
	if resp.JSONErr != nil {
		return false, fmt.Errorf("invalid device code response: %w", resp.JSONErr)
	}

	deviceCode := dcResp.DeviceCode
	userCode := dcResp.UserCode
	verificationURI := dcResp.VerificationURI
	interval := dcResp.Interval
	if interval == 0 {
		interval = 5
	}

	// Display instructions
	if !quiet {
		_, _ = fmt.Fprintln(w, "\n🔐 GitHub Device Flow Authentication")
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "  1. Open %s\n", verificationURI)
		if len(userCode) == 8 {
			_, _ = fmt.Fprintf(w, "  2. Enter code: %s-%s\n", userCode[:4], userCode[4:])
		} else {
			_, _ = fmt.Fprintf(w, "  2. Enter code: %s\n", userCode)
		}
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "  Waiting for authorization...")
	} else {
		_, _ = fmt.Fprintln(w, verificationURI)
		_, _ = fmt.Fprintf(w, "Code: %s\n", userCode)
	}

	// Poll for token
	for attempt := 0; attempt < 60; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}

		var tokenResp TokenResponse
		pollResp, err := client.PostForm(tokenURL,
			map[string]string{
				"client_id":   clientID,
				"device_code": deviceCode,
				"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
			},
			&tokenResp,
			httpclient.WithHeader("Accept", "application/json"),
		)
		if err != nil {
			if attempt < 3 {
				continue
			}
			return false, fmt.Errorf("network error: %w", err)
		}
		if pollResp.JSONErr != nil {
			continue
		}

		if tokenResp.AccessToken != "" {
			creds := OAuthCredentials{AccessToken: tokenResp.AccessToken}
			content, _ := json.Marshal(creds)
			_ = config.WriteCredential(config.CredentialPath("copilot", "oauth"), content)
			if !quiet {
				_, _ = fmt.Fprintln(w, "\n  ✓ Authentication successful!")
			}
			return true, nil
		}

		switch tokenResp.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			if !quiet {
				_, _ = fmt.Fprintln(w, "\n  ✗ Device code expired.")
			}
			return false, nil
		case "access_denied":
			if !quiet {
				_, _ = fmt.Fprintln(w, "\n  ✗ Authorization denied by user.")
			}
			return false, nil
		default:
			desc := tokenResp.ErrorDescription
			if desc == "" {
				desc = tokenResp.Error
			}
			return false, fmt.Errorf("authentication error: %s", desc)
		}
	}

	if !quiet {
		_, _ = fmt.Fprintln(w, "\n  ⏱ Timeout waiting for authorization.")
	}
	return false, nil
}
