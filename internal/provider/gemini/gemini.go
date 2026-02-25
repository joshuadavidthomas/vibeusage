package gemini

import (
	"context"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

type Gemini struct{}

func (g Gemini) Meta() provider.Metadata {
	return provider.Metadata{
		ID:           "gemini",
		Name:         "Gemini",
		Description:  "Google Gemini AI",
		Homepage:     "https://gemini.google.com",
		DashboardURL: "https://aistudio.google.com/app/usage",
	}
}

func (g Gemini) CredentialSources() provider.CredentialInfo {
	return provider.CredentialInfo{
		CLIPaths: []string{"~/.gemini/oauth_creds.json"},
		EnvVars:  []string{"GEMINI_API_KEY"},
	}
}

func (g Gemini) FetchStrategies() []fetch.Strategy {
	timeout := config.Get().Fetch.Timeout
	return []fetch.Strategy{
		&OAuthStrategy{HTTPTimeout: timeout},
		&APIKeyStrategy{HTTPTimeout: timeout},
	}
}

func (g Gemini) FetchStatus(ctx context.Context) models.ProviderStatus {
	return provider.FetchGoogleAppsStatus(ctx, []string{
		"gemini", "ai studio", "aistudio", "generative ai", "vertex ai", "cloud code",
	})
}

// Auth returns the manual API key flow for Gemini.
// Gemini OAuth is managed by the Gemini CLI — users who want the richer OAuth
// flow should install the CLI and run `gemini login`. Those with an AI Studio
// API key can provide it here instead.
func (g Gemini) Auth() provider.AuthFlow {
	return provider.ManualKeyAuthFlow{
		Instructions: "Get your Gemini API key from Google AI Studio:\n" +
			"  1. Open https://aistudio.google.com/app/apikey\n" +
			"  2. Create a new API key (or copy an existing one)\n" +
			"\n" +
			"Alternatively, install the Gemini CLI and run `gemini login` to use\n" +
			"OAuth credentials, which will be picked up automatically.",
		Placeholder: "AI Studio API key",
		Validate:    provider.ValidateNotEmpty,
		CredPath:    config.CredentialPath("gemini", "api_key"),
		JSONKey:     "api_key",
	}
}

func init() {
	provider.Register(Gemini{})
}

func nextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}

// titleCase capitalizes the first letter of each space-separated word.
// Used for formatting model display names (e.g. "gemini 2 5 flash" → "Gemini 2 5 Flash").
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
