package kimicode

import (
	"context"
	"encoding/json"
	"os"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
)

// APIKeyStrategy fetches Kimi usage using an API key.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadAPIKey() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	apiKey := s.loadAPIKey()
	if apiKey == "" {
		return fetch.ResultFail("No API key found. Set KIMI_CODE_API_KEY or use 'vibeusage key kimicode set'"), nil
	}

	return fetchUsage(ctx, apiKey, "api_key", s.HTTPTimeout)
}

func (s *APIKeyStrategy) loadAPIKey() string {
	if key := os.Getenv("KIMI_CODE_API_KEY"); key != "" {
		return key
	}
	path := config.CredentialPath("kimicode", "apikey")
	data, err := config.ReadCredential(path)
	if err != nil || data == nil {
		return ""
	}
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return creds.APIKey
}
