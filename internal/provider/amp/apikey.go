package amp

import (
	"context"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

// APIKeyStrategy supports manual key entry or AMP_API_KEY.
type APIKeyStrategy struct {
	HTTPTimeout float64
}

var ampAPIKey = provider.APIKeySource{
	EnvVars:  []string{"AMP_API_KEY"},
	CredPath: config.CredentialPath("amp", "apikey"),
	JSONKeys: []string{"api_key"},
}

func (s *APIKeyStrategy) IsAvailable() bool {
	return ampAPIKey.Load() != ""
}

func (s *APIKeyStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token := ampAPIKey.Load()
	if token == "" {
		return fetch.ResultFail("No API key found. Set AMP_API_KEY or use 'vibeusage auth amp'"), nil
	}
	return fetchBalance(ctx, token, "api_key", s.HTTPTimeout)
}
