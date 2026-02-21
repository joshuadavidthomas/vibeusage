package claude

import (
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

const (
	oauthUsageURL    = "https://api.anthropic.com/api/oauth/usage"
	oauthTokenURL    = "https://api.anthropic.com/oauth/token"
	webBaseURL       = "https://claude.ai/api/organizations"
	anthropicBetaTag = "oauth-2025-04-20"
)

type Claude struct{}

func (c Claude) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "claude",
		Name:         "Claude",
		Description:  "Anthropic's Claude AI assistant",
		Homepage:     "https://claude.ai",
		StatusURL:    "https://status.anthropic.com",
		DashboardURL: "https://claude.ai/settings/usage",
	}
}

func (c Claude) FetchStrategies() []fetch.Strategy {
	return []fetch.Strategy{
		&OAuthStrategy{},
		&WebStrategy{},
		&CLIStrategy{},
	}
}

func (c Claude) FetchStatus() models.ProviderStatus {
	return provider.FetchStatuspageStatus("https://status.anthropic.com/api/v2/status.json")
}

func init() {
	provider.Register(Claude{})
}
