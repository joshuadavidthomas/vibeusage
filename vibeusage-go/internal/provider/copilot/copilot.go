package copilot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
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
	clientID      = "Iv1.b507a08c87ecfe98"
)

type DeviceFlowStrategy struct{}

func (s *DeviceFlowStrategy) Name() string { return "device_flow" }

func (s *DeviceFlowStrategy) IsAvailable() bool {
	path := config.CredentialPath("copilot", "oauth")
	_, err := os.Stat(path)
	return err == nil
}

func (s *DeviceFlowStrategy) Fetch() (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found. Run `vibeusage auth copilot` to authenticate."), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	req, _ := http.NewRequest("GET", usageURL, nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

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
		return fetch.ResultFail("Usage request failed: " + resp.Status), nil
	}

	body, _ := io.ReadAll(resp.Body)
	var userResp UserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
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
		resetStr := strings.Replace(resp.QuotaResetDateUTC, "Z", "+00:00", 1)
		if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
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
	client := &http.Client{Timeout: 30 * time.Second}

	// Request device code
	form := url.Values{
		"client_id": {clientID},
		"scope":     {"read:user"},
	}

	req, _ := http.NewRequest("POST", deviceCodeURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var dcResp DeviceCodeResponse
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return false, fmt.Errorf("invalid device code response: %w", err)
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
		fmt.Fprintln(w, "\n🔐 GitHub Device Flow Authentication")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  1. Open %s\n", verificationURI)
		if len(userCode) == 8 {
			fmt.Fprintf(w, "  2. Enter code: %s-%s\n", userCode[:4], userCode[4:])
		} else {
			fmt.Fprintf(w, "  2. Enter code: %s\n", userCode)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Waiting for authorization...")
	} else {
		fmt.Fprintln(w, verificationURI)
		fmt.Fprintf(w, "Code: %s\n", userCode)
	}

	// Poll for token
	for attempt := 0; attempt < 60; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}

		pollForm := url.Values{
			"client_id":   {clientID},
			"device_code": {deviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		pollReq, _ := http.NewRequest("POST", tokenURL, strings.NewReader(pollForm.Encode()))
		pollReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		pollReq.Header.Set("Accept", "application/json")

		pollResp, err := client.Do(pollReq)
		if err != nil {
			if attempt < 3 {
				continue
			}
			return false, fmt.Errorf("network error: %w", err)
		}

		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var tokenResp TokenResponse
		if err := json.Unmarshal(pollBody, &tokenResp); err != nil {
			continue
		}

		if tokenResp.AccessToken != "" {
			creds := OAuthCredentials{AccessToken: tokenResp.AccessToken}
			content, _ := json.Marshal(creds)
			_ = config.WriteCredential(config.CredentialPath("copilot", "oauth"), content)
			if !quiet {
				fmt.Fprintln(w, "\n  ✓ Authentication successful!")
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
				fmt.Fprintln(w, "\n  ✗ Device code expired.")
			}
			return false, nil
		case "access_denied":
			if !quiet {
				fmt.Fprintln(w, "\n  ✗ Authorization denied by user.")
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
		fmt.Fprintln(w, "\n  ⏱ Timeout waiting for authorization.")
	}
	return false, nil
}
