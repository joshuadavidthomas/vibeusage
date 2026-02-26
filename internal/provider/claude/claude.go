package claude

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
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

func (c Claude) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.claude/.credentials.json"},
	}
}

func (c Claude) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
		&WebStrategy{HTTPTimeout: timeout},
	}
}

func (c Claude) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.anthropic.com")
}

// Auth returns a manual credential flow for Claude.
func (c Claude) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Paste your claude.ai session key:\n" +
			"  1. Open https://claude.ai in your browser\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://claude.ai\n" +
			"  4. Find the sessionKey cookie\n" +
			"  5. Copy its value (starts with sk-ant-sid01-)",
		Placeholder: "sk-ant-sid01-...",
		Validate:    provider.ValidateAnyPrefix("sk-ant-sid01-"),
		Save:        saveClaudeCredential,
	}
}

func saveClaudeCredential(value string) error {
	value = strings.TrimSpace(value)

	path := config.CredentialPath("claude", "session")
	content, _ := json.Marshal(map[string]string{"session_key": value})
	return config.WriteCredential(path, content)
}

func init() {
	provider.Register(Claude{})
}
