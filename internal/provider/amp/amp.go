package amp

import (
	"context"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
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
