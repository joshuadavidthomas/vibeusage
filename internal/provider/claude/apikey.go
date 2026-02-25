package claude

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
)

// APIKeyStrategy recognizes Anthropic API keys and preserves them for auth
// workflows. Claude consumer quota metrics still come from OAuth/session data.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

func (s *APIKeyStrategy) IsAvailable() bool {
	return s.loadAPIKey() != ""
}

func (s *APIKeyStrategy) Fetch(_ context.Context) (fetch.FetchResult, error) {
	key := s.loadAPIKey()
	if key == "" {
		return fetch.ResultFail("No API key found"), nil
	}

	if !strings.HasPrefix(key, "sk-ant-") {
		return fetch.ResultFatal("Invalid Anthropic API key format"), nil
	}

	return fetch.ResultFail("Anthropic API keys are configured, but claude.ai plan usage requires Claude OAuth/session credentials."), nil
}

func (s *APIKeyStrategy) loadAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); v != "" {
		return v
	}

	data, err := config.ReadCredential(config.CredentialPath("claude", "apikey"))
	if err != nil || data == nil {
		return ""
	}

	var key struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &key); err != nil {
		return ""
	}
	return strings.TrimSpace(key.APIKey)
}
