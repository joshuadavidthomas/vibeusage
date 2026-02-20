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

	accessToken, _ := creds["access_token"].(string)
	if accessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	req, _ := http.NewRequest("GET", usageURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return fetch.ResultFail("Invalid response from Copilot API"), nil
	}

	snapshot := s.parseUsageResponse(data)
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse Copilot usage response"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *DeviceFlowStrategy) loadCredentials() map[string]any {
	data, err := config.ReadCredential(config.CredentialPath("copilot", "oauth"))
	if err != nil || data == nil {
		return nil
	}
	var creds map[string]any
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil
	}
	return creds
}

func (s *DeviceFlowStrategy) parseUsageResponse(data map[string]any) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	// Parse reset date
	var resetsAt *time.Time
	if resetStr, ok := data["quota_reset_date_utc"].(string); ok {
		resetStr = strings.Replace(resetStr, "Z", "+00:00", 1)
		if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
			resetsAt = &t
		}
	}

	// Parse quota_snapshots
	if quotas, ok := data["quota_snapshots"].(map[string]any); ok {
		// Premium interactions
		if premium, ok := quotas["premium_interactions"].(map[string]any); ok {
			entitlement, _ := premium["entitlement"].(float64)
			remaining, _ := premium["remaining"].(float64)
			unlimited, _ := premium["unlimited"].(bool)

			var utilization int
			if unlimited && entitlement == 0 {
				utilization = 0
			} else if entitlement > 0 {
				used := entitlement - remaining
				utilization = int((used / entitlement) * 100)
			}

			if unlimited || entitlement > 0 {
				periods = append(periods, models.UsagePeriod{
					Name:        "Monthly (Premium)",
					Utilization: utilization,
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    resetsAt,
				})
			}
		}

		// Chat
		if chat, ok := quotas["chat"].(map[string]any); ok {
			entitlement, _ := chat["entitlement"].(float64)
			remaining, _ := chat["remaining"].(float64)
			unlimited, _ := chat["unlimited"].(bool)

			if unlimited && entitlement == 0 {
				periods = append(periods, models.UsagePeriod{
					Name:        "Monthly (Chat)",
					Utilization: 0,
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    resetsAt,
				})
			} else if entitlement > 0 {
				used := entitlement - remaining
				periods = append(periods, models.UsagePeriod{
					Name:        "Monthly (Chat)",
					Utilization: int((used / entitlement) * 100),
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    resetsAt,
				})
			}
		}

		// Completions
		if completions, ok := quotas["completions"].(map[string]any); ok {
			entitlement, _ := completions["entitlement"].(float64)
			remaining, _ := completions["remaining"].(float64)
			unlimited, _ := completions["unlimited"].(bool)

			if unlimited && entitlement == 0 {
				periods = append(periods, models.UsagePeriod{
					Name:        "Monthly (Completions)",
					Utilization: 0,
					PeriodType:  models.PeriodMonthly,
					ResetsAt:    resetsAt,
				})
			} else if entitlement > 0 {
				used := entitlement - remaining
				periods = append(periods, models.UsagePeriod{
					Name:        "Monthly (Completions)",
					Utilization: int((used / entitlement) * 100),
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
	if plan, ok := data["copilot_plan"].(string); ok {
		identity = &models.ProviderIdentity{Plan: plan}
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
func RunDeviceFlow(quiet bool) (bool, error) {
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
	var dcResp map[string]any
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return false, fmt.Errorf("invalid device code response: %w", err)
	}

	deviceCode, _ := dcResp["device_code"].(string)
	userCode, _ := dcResp["user_code"].(string)
	verificationURI, _ := dcResp["verification_uri"].(string)
	interval := 5
	if iv, ok := dcResp["interval"].(float64); ok {
		interval = int(iv)
	}

	// Display instructions
	if !quiet {
		fmt.Println("\n🔐 GitHub Device Flow Authentication")
		fmt.Println()
		fmt.Printf("  1. Open %s\n", verificationURI)
		if len(userCode) == 8 {
			fmt.Printf("  2. Enter code: %s-%s\n", userCode[:4], userCode[4:])
		} else {
			fmt.Printf("  2. Enter code: %s\n", userCode)
		}
		fmt.Println()
		fmt.Println("  Waiting for authorization...")
	} else {
		fmt.Println(verificationURI)
		fmt.Printf("Code: %s\n", userCode)
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

		var tokenData map[string]any
		if err := json.Unmarshal(pollBody, &tokenData); err != nil {
			continue
		}

		if _, ok := tokenData["access_token"]; ok {
			content, _ := json.Marshal(tokenData)
			_ = config.WriteCredential(config.CredentialPath("copilot", "oauth"), content)
			if !quiet {
				fmt.Println("\n  ✓ Authentication successful!")
			}
			return true, nil
		}

		errStr, _ := tokenData["error"].(string)
		switch errStr {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			if !quiet {
				fmt.Println("\n  ✗ Device code expired.")
			}
			return false, nil
		case "access_denied":
			if !quiet {
				fmt.Println("\n  ✗ Authorization denied by user.")
			}
			return false, nil
		default:
			desc, _ := tokenData["error_description"].(string)
			if desc == "" {
				desc = errStr
			}
			return false, fmt.Errorf("authentication error: %s", desc)
		}
	}

	if !quiet {
		fmt.Println("\n  ⏱ Timeout waiting for authorization.")
	}
	return false, nil
}
