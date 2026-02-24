package kimik2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type KimiK2 struct{}

func (k KimiK2) Meta() provider.Metadata {
	return provider.Metadata{
		ID:          "kimik2",
		Name:        "Kimi K2",
		Description: "Kimi K2 API usage",
		Homepage:    "https://kimi-k2.ai",
	}
}

func (k KimiK2) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{EnvVars: []string{"KIMI_K2_API_KEY", "KIMI_API_KEY", "KIMI_KEY"}}
}

func (k KimiK2) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{&APIKeyStrategy{HTTPTimeout: timeout}}
}

func (k KimiK2) FetchStatus(_ context.Context) models.ProviderStatus {
	return models.ProviderStatus{Level: models.StatusUnknown}
}

func (k KimiK2) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your Kimi K2 API key from your Kimi account settings.",
		Placeholder:  "k2-...",
		Validate:     provider.ValidateNotEmpty,
		CredPath:     config.CredentialPath("kimik2", "apikey"),
		JSONKey:      "api_key",
	}
}

func init() {
	provider.Register(KimiK2{})
}

var creditsURL = "https://kimi-k2.ai/api/user/credits"

// APIKeyStrategy fetches Kimi K2 credits using an API key.
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
		return fetch.ResultFail("No API key found. Set KIMI_K2_API_KEY or use 'vibeusage key kimik2 set'"), nil
	}
	return fetchCredits(ctx, token, s.HTTPTimeout)
}

func (s *APIKeyStrategy) loadToken() string {
	for _, envVar := range []string{"KIMI_K2_API_KEY", "KIMI_API_KEY", "KIMI_KEY"} {
		if token := strings.TrimSpace(os.Getenv(envVar)); token != "" {
			return token
		}
	}
	data, err := config.ReadCredential(config.CredentialPath("kimik2", "apikey"))
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

func fetchCredits(ctx context.Context, token string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	var parsed CreditsResponse
	resp, err := client.GetJSONCtx(ctx, creditsURL, &parsed, httpclient.WithBearer(token))
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fetch.ResultFatal("API key is invalid or expired. Run `vibeusage auth kimik2` to re-authenticate."), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFail(fmt.Sprintf("Kimi K2 credits request failed: HTTP %d (%s)", resp.StatusCode, summarizeBody(resp.Body))), nil
	}
	if resp.JSONErr != nil {
		return fetch.ResultFail(fmt.Sprintf("Invalid response from Kimi K2 API: %v", resp.JSONErr)), nil
	}

	snapshot, err := parseCreditsSnapshot(parsed, resp.Header)
	if err != nil {
		return fetch.ResultFail("Failed to parse Kimi K2 credits: " + err.Error()), nil
	}
	return fetch.ResultOK(*snapshot), nil
}

type CreditsResponse struct {
	Consumed  any          `json:"consumed,omitempty"`
	Remaining any          `json:"remaining,omitempty"`
	Used      any          `json:"used,omitempty"`
	Balance   any          `json:"balance,omitempty"`
	Data      *CreditsData `json:"data,omitempty"`
}

type CreditsData struct {
	Consumed  any `json:"consumed,omitempty"`
	Remaining any `json:"remaining,omitempty"`
	Used      any `json:"used,omitempty"`
	Balance   any `json:"balance,omitempty"`
	Left      any `json:"left,omitempty"`
}

func parseCreditsSnapshot(resp CreditsResponse, headers http.Header) (*models.UsageSnapshot, error) {
	consumed, okConsumed := firstNumber(
		resp.Consumed,
		resp.Used,
		resp.DataField(func(d *CreditsData) any { return d.Consumed }),
		resp.DataField(func(d *CreditsData) any { return d.Used }),
	)
	remaining, okRemaining := firstNumber(
		resp.Remaining,
		resp.Balance,
		resp.DataField(func(d *CreditsData) any { return d.Remaining }),
		resp.DataField(func(d *CreditsData) any { return d.Balance }),
		resp.DataField(func(d *CreditsData) any { return d.Left }),
	)
	if !okRemaining {
		if h, ok := remainingFromHeaders(headers); ok {
			remaining = h
			okRemaining = true
		}
	}
	if !okConsumed {
		consumed = 0
	}
	if !okRemaining {
		return nil, fmt.Errorf("missing remaining credits in response")
	}

	if consumed < 0 {
		consumed = 0
	}
	if remaining < 0 {
		remaining = 0
	}

	total := consumed + remaining
	utilization := 0
	if total > 0 {
		utilization = int((consumed / total) * 100)
		if utilization < 0 {
			utilization = 0
		}
		if utilization > 100 {
			utilization = 100
		}
	}

	return &models.UsageSnapshot{
		Provider:  "kimik2",
		FetchedAt: time.Now().UTC(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Credits",
				Utilization: utilization,
				PeriodType:  models.PeriodMonthly,
			},
		},
		Source: "api_key",
	}, nil
}

func (r CreditsResponse) DataField(fn func(*CreditsData) any) any {
	if r.Data == nil {
		return nil
	}
	return fn(r.Data)
}

func firstNumber(values ...any) (float64, bool) {
	for _, v := range values {
		if n, ok := parseNumber(v); ok {
			return n, true
		}
	}
	return 0, false
}

func parseNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		n, err := x.Float64()
		return n, err == nil
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func remainingFromHeaders(h http.Header) (float64, bool) {
	if h == nil {
		return 0, false
	}
	for _, key := range []string{"X-Remaining-Credits", "X-Credits-Remaining", "x-remaining-credits", "x-credits-remaining"} {
		raw := strings.TrimSpace(h.Get(key))
		if raw == "" {
			continue
		}
		n, err := strconv.ParseFloat(raw, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
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
