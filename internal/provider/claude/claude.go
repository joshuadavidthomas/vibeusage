package claude

import (
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
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
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
		&WebStrategy{HTTPTimeout: timeout},
	}
}

func (c Claude) FetchStatus() models.ProviderStatus {
	return provider.FetchStatuspageStatus("https://status.anthropic.com/api/v2/status.json")
}

// Auth returns the manual session key flow for Claude.
func (c Claude) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your session key from claude.ai:\n" +
			"  1. Open https://claude.ai in your browser\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://claude.ai\n" +
			"  4. Find the sessionKey cookie\n" +
			"  5. Copy its value (starts with sk-ant-sid01-)",
		Placeholder: "sk-ant-sid01-...",
		Validate:    prompt.ValidateClaudeSessionKey,
		CredPath:    config.CredentialPath("claude", "session"),
		JSONKey:     "session_key",
	}
}

func init() {
	provider.Register(Claude{})
}
