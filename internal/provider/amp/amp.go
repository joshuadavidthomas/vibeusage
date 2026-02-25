package amp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Amp struct{}

func (a Amp) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "amp",
		Name:        "Amp",
		Description: "Amp coding assistant",
		Homepage:    "https://ampcode.com",
	}
}

func (a Amp) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.local/share/amp/secrets.json"},
		EnvVars:  []string{"AMP_API_KEY"},
	}
}

func (a Amp) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&CLISecretsStrategy{HTTPTimeout: timeout},
		&APIKeyStrategy{HTTPTimeout: timeout},
	}
}

func (a Amp) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

func (a Amp) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Paste your Amp API key. If Amp CLI is installed, vibeusage can also auto-detect ~/.local/share/amp/secrets.json.",
		Placeholder:  "amp-...",
		Validate:     provider.ValidateNotEmpty,
		CredPath:     config.CredentialPath("amp", "apikey"),
		JSONKey:      "api_key",
	}
}

func init() {
	provider.Register(Amp{})
}

var internalRPCURL = "https://ampcode.com/api/internal"

// CLISecretsStrategy loads Amp credentials from the local secrets file.
type CLISecretsStrategy struct {
	HTTPTimeout float64
}

func (s *CLISecretsStrategy) Name() string { return "provider_cli" }

func (s *CLISecretsStrategy) IsAvailable() bool {
	if !provider.ExternalCredentialReuseEnabled() {
		return false
	}
	_, ok := loadCLISecretsToken()
	return ok
}

func (s *CLISecretsStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token, ok := loadCLISecretsToken()
	if !ok {
		return fetch.ResultFail("No Amp CLI credentials found in ~/.local/share/amp/secrets.json"), nil
	}
	return fetchBalance(ctx, token, s.Name(), s.HTTPTimeout)
}

// APIKeyStrategy supports manual key entry or AMP_API_KEY.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) Name() string { return "api_key" }

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadToken() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := s.loadToken()
	if token == "" {
		return fetch.ResultFail("No API key found. Set AMP_API_KEY or use 'vibeusage key amp set'"), nil
	}
	return fetchBalance(ctx, token, s.Name(), s.HTTPTimeout)
}

func (s *APIKeyStrategy) loadToken() string {
	if token := strings.TrimSpace(os.Getenv("AMP_API_KEY")); token != "" {
		return token
	}
	data, err := config.ReadCredential(config.CredentialPath("amp", "apikey"))
	if err != nil || data == nil {
		return ""
	}
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return strings.TrimSpace(creds.APIKey)
}

func loadCLISecretsToken() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	path := filepath.Join(home, ".local", "share", "amp", "secrets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return "", false
	}
	token := strings.TrimSpace(secrets["apiKey@https://ampcode.com/"])
	if token == "" {
		return "", false
	}
	return token, true
}

func fetchBalance(ctx context.Context, token string, source string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	body := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "vibeusage",
		Method:  "userDisplayBalanceInfo",
		Params:  []string{},
	}

	var rpcResp rpcResponse
	resp, err := client.PostJSONCtx(ctx, internalRPCURL, body, &rpcResp,
		httpclient.WithBearer(token),
		httpclient.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fetch.ResultFatal("Token invalid or expired. Run `vibeusage auth amp` to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Amp usage request failed: HTTP %d (%s)", resp.StatusCode, summarizeBody(resp.Body))), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Amp API: %v", resp.JSONErr)), nil
	}
	if rpcResp.Error != nil {
		msg := strings.TrimSpace(rpcResp.Error.Message)
		if msg == "" {
			msg = fmt.Sprintf("rpc error code %d", rpcResp.Error.Code)
		}
		if strings.Contains(strings.ToLower(msg), "unauthor") {
			return fetch.ResultFatal("Token invalid or expired. Run `vibeusage auth amp` to re-authenticate."), nil
		}
		return fetch.ResultFail(msg), nil
	}
	if rpcResp.Result == nil {
		return fetch.ResultFail("Amp response missing result payload"), nil
	}

	snapshot, err := parseDisplayBalance(*rpcResp.Result, source)
	if err != nil {
		return fetch.ResultFail("Failed to parse Amp balance text: " + err.Error()), nil
	}
	return fetch.ResultOK(*snapshot), nil
}

type jsonRPCRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      string   `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

type rpcResponse struct {
	Result *balanceResult `json:"result"`
	Error  *rpcError      `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type balanceResult struct {
	DisplayText string `json:"displayText"`
}

var (
	quotaPattern   = regexp.MustCompile(`(?i)\$([0-9]+(?:\.[0-9]+)?)\s*/\s*\$([0-9]+(?:\.[0-9]+)?)`)
	hourlyPattern  = regexp.MustCompile(`(?i)\$([0-9]+(?:\.[0-9]+)?)\s*/\s*hour`)
	creditsPattern = regexp.MustCompile(`(?i)(?:bonus\s+)?credits?:\s*\$([0-9]+(?:\.[0-9]+)?)`)
)

func parseDisplayBalance(result balanceResult, source string) (*models.UsageSnapshot, error) {
	text := strings.TrimSpace(result.DisplayText)
	if text == "" {
		return nil, fmt.Errorf("empty displayText")
	}

	periods := make([]models.UsagePeriod, 0, 1)

	quotaMatch := quotaPattern.FindStringSubmatch(text)
	if len(quotaMatch) == 3 {
		remaining, err := strconv.ParseFloat(quotaMatch[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse remaining quota: %w", err)
		}
		total, err := strconv.ParseFloat(quotaMatch[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse total quota: %w", err)
		}

		utilization := 0
		if total > 0 {
			used := total - remaining
			if used < 0 {
				used = 0
			}
			utilization = int((used / total) * 100)
			if utilization < 0 {
				utilization = 0
			}
			if utilization > 100 {
				utilization = 100
			}
		}

		period := models.UsagePeriod{
			Name:        "Daily Free Quota",
			Utilization: utilization,
			PeriodType:  models.PeriodDaily,
		}

		rateMatch := hourlyPattern.FindStringSubmatch(text)
		if len(rateMatch) == 2 {
			rate, err := strconv.ParseFloat(rateMatch[1], 64)
			if err == nil && rate > 0 && total > remaining {
				hoursUntilFull := (total - remaining) / rate
				if hoursUntilFull > 0 {
					reset := time.Now().UTC().Add(time.Duration(hoursUntilFull * float64(time.Hour)))
					period.ResetsAt = &reset
				}
			}
		}

		periods = append(periods, period)
	}

	var overage *models.OverageUsage
	creditMatch := creditsPattern.FindStringSubmatch(text)
	if len(creditMatch) == 2 {
		credits, err := strconv.ParseFloat(creditMatch[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse credits amount: %w", err)
		}
		overage = &models.OverageUsage{
			Used:      0,
			Limit:     credits,
			Currency:  "USD",
			IsEnabled: true,
		}
	}

	if len(periods) == 0 {
		if overage == nil {
			return nil, fmt.Errorf("unrecognized displayText format: %q", text)
		}
		periods = append(periods, models.UsagePeriod{
			Name:        "Credits Balance",
			Utilization: 0,
			PeriodType:  models.PeriodMonthly,
		})
	}

	snapshot := &models.UsageSnapshot{
		Provider:  "amp",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Overage:   overage,
		Source:    source,
	}
	if overage != nil {
		snapshot.Identity = &models.ProviderIdentity{Organization: fmt.Sprintf("Credits: $%.2f", overage.Limit)}
	}
	return snapshot, nil
}

func summarizeBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "empty body"
	}
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
