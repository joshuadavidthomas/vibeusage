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
		EnvVars:  []string{"ANTHROPIC_API_KEY"},
	}
}

func (c Claude) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
		&APIKeyStrategy{HTTPTimeout: timeout},
		&WebStrategy{HTTPTimeout: timeout},
	}
}

func (c Claude) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchStatuspageStatus(ctx, "https://status.anthropic.com/api/v2/status.json")
}

// Auth returns a manual credential flow for Claude.
//
// Accepted inputs:
// - Anthropic API key (sk-ant-api... / sk-ant-admin-...)
// - claude.ai sessionKey cookie (sk-ant-sid01-...) as web fallback
func (c Claude) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Provide one of the following credentials:\n" +
			"\n" +
			"Option A (recommended): Claude CLI OAuth\n" +
			"  Run `claude auth login` and vibeusage will auto-detect it.\n" +
			"\n" +
			"Option B: Anthropic API key\n" +
			"  Use a key from https://platform.claude.com/settings/keys (starts with sk-ant-api or sk-ant-admin-).\n" +
			"\n" +
			"Option C (fallback): claude.ai session key\n" +
			"  1. Open https://claude.ai in your browser\n" +
			"  2. Open DevTools (F12 or Cmd+Option+I)\n" +
			"  3. Go to Application → Cookies → https://claude.ai\n" +
			"  4. Find the sessionKey cookie\n" +
			"  5. Copy its value (starts with sk-ant-sid01-)",
		Placeholder: "sk-ant-sid01-... or sk-ant-api...",
		Validate:    provider.ValidateAnyPrefix("sk-ant-sid01-", "sk-ant-api", "sk-ant-admin-"),
		Save:        saveClaudeCredential,
	}
}

func saveClaudeCredential(value string) error {
	value = strings.TrimSpace(value)

	path := config.CredentialPath("claude", "session")
	key := "session_key"
	if strings.HasPrefix(value, "sk-ant-api") || strings.HasPrefix(value, "sk-ant-admin-") {
		path = config.CredentialPath("claude", "apikey")
		key = "api_key"
	}

	content, _ := json.Marshal(map[string]string{key: value})
	return config.WriteCredential(path, content)
}

func init() {
	provider.Register(Claude{})
}
