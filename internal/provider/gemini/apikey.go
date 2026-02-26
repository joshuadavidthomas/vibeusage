package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

const modelsURL = "https://generativelanguage.googleapis.com/v1beta/models"

// APIKeyStrategy fetches Gemini usage using an API key.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) IsAvailable() bool {
	if os.Getenv("GEMINI_API_KEY") != "" {
		return true
	}
	credDir := filepath.Join(config.CredentialsDir(), "gemini")
	for _, name := range []string{"api_key.txt", "api_key.json"} {
		if _, err := os.Stat(filepath.Join(credDir, name)); err == nil {
			return true
		}
	}
	return false
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	apiKey := s.loadAPIKey()
	if apiKey == "" {
		return fetch.ResultFail("No API key found. Set GEMINI_API_KEY or use 'vibeusage auth gemini'"), nil
	}

	// Validate key by fetching models
	client := httpclient.NewFromConfig(s.HTTPTimeout)
	var modelsResp ModelsResponse
	resp, err := client.GetJSONCtx(ctx, modelsURL+"?key="+apiKey, &modelsResp)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if resp.StatusCode == 401 {
		return fetch.ResultFatal("API key is invalid or expired"), nil
	}
	if resp.StatusCode == 403 {
		return fetch.ResultFatal("API key does not have access to Generative Language API"), nil
	}
	if resp.StatusCode == 429 {
		return fetch.ResultFatal("Rate limit exceeded"), nil
	}
	if resp.StatusCode != 200 {
		return fetch.ResultFatal(fmt.Sprintf("Failed to validate API key: %d", resp.StatusCode)), nil
	}

	modelCount := len(modelsResp.Models)

	tomorrow := nextMidnightUTC()
	snapshot := models.UsageSnapshot{
		Provider:  "gemini",
		FetchedAt: time.Now().UTC(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Daily",
				Utilization: 0,
				PeriodType:  models.PeriodDaily,
				ResetsAt:    &tomorrow,
			},
		},
		Identity: &models.ProviderIdentity{
			Plan:         "API Key",
			Organization: "Available models: " + strconv.Itoa(modelCount),
		},
		Source: "api_key",
	}

	return fetch.ResultOK(snapshot), nil
}

// apiKeyFile represents JSON credential files that contain an API key.
type apiKeyFile struct {
	APIKey string `json:"api_key,omitempty"`
	Key    string `json:"key,omitempty"`
}

func (a *apiKeyFile) effectiveKey() string {
	if a.APIKey != "" {
		return a.APIKey
	}
	return a.Key
}

func (s *APIKeyStrategy) loadAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	// Check credential files
	for _, suffix := range []string{".txt", ".json"} {
		path := filepath.Join(config.CredentialsDir(), "gemini", "api_key"+suffix)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if suffix == ".json" {
			var keyFile apiKeyFile
			if json.Unmarshal(data, &keyFile) == nil {
				if key := keyFile.effectiveKey(); key != "" {
					return key
				}
			}
			continue // Don't return raw JSON as an API key
		}
		return strings.TrimSpace(string(data))
	}
	return ""
}
