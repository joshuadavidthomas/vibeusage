package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var providerDescriptions = map[string]string{
	"amp":        "Amp coding assistant (ampcode.com)",
	"claude":     "Anthropic's Claude AI assistant (claude.ai)",
	"codex":      "OpenAI's Codex/ChatGPT (platform.openai.com)",
	"copilot":    "GitHub Copilot (github.com)",
	"cursor":     "Cursor AI code editor (cursor.com)",
	"gemini":     "Google's Gemini AI (gemini.google.com)",
	"kimicode":   "Kimi Code coding assistant (kimi.com)",
	"openrouter": "OpenRouter unified model gateway (openrouter.ai)",
	"warp":       "Warp terminal AI (warp.dev)",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Run first-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		quick, _ := cmd.Flags().GetBool("quick")

		if jsonOutput {
			return display.OutputJSON(outWriter, display.InitStatusJSON{
				FirstRun:            provider.IsFirstRun(),
				ConfiguredProviders: provider.CountConfigured(),
				AvailableProviders:  provider.ListIDs(),
			})
		}

		if quick {
			return quickSetup()
		}

		return interactiveWizard()
	},
}

func init() {
	initCmd.Flags().BoolP("quick", "q", false, "Quick setup with default provider (Claude)")
}

func quickSetup() error {
	if quiet {
		outln("Run 'vibeusage auth claude' to set up Claude")
		return nil
	}

	outln("Quick Setup: Claude")
	outln("Claude is the most popular AI assistant for agentic workflows.")
	outln()

	hasCreds, _ := provider.CheckCredentials("claude")
	if hasCreds {
		outln("✓ Claude is already configured!")
		outln("\nRun 'vibeusage' to see your usage.")
		return nil
	}

	outln("To set up Claude, run:")
	outln("  vibeusage auth claude")
	outln()
	outln("After setup, run 'vibeusage' to see your usage.")
	return nil
}

func interactiveWizard() error {
	if quiet {
		outln("Use 'vibeusage auth <provider>' to set up providers")
		return nil
	}

	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	outln()
	outln("  ✨ Welcome to vibeusage!")
	outln()
	outln("  Track your usage across AI providers in one place.")
	outln()

	// Build options for multi-select
	options := make([]prompt.SelectOption, 0, len(allProviders))
	for _, pid := range allProviders {
		hasCreds, _ := provider.CheckCredentials(pid)
		status := ""
		if hasCreds {
			status = "✓ "
		}
		desc := providerDescriptions[pid]
		if desc == "" {
			desc = provider.DisplayName(pid) + " AI"
		}
		label := status + pid + " — " + desc
		options = append(options, prompt.SelectOption{Label: label, Value: pid})
	}

	selected, err := prompt.Default.MultiSelect(prompt.MultiSelectConfig{
		Title:       "Choose providers to set up",
		Description: "Space to select, Enter to confirm",
		Options:     options,
	})
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		outln("\nNo providers selected. You can set up providers later:")
		for _, pid := range allProviders {
			out("  vibeusage auth %s\n", pid)
		}
		return nil
	}

	out("\nSet up %d provider(s):\n\n", len(selected))

	for _, pid := range selected {
		hasCreds, _ := provider.CheckCredentials(pid)
		if hasCreds {
			out("  ✓ %s already configured\n", pid)
		} else {
			out("  → %s: vibeusage auth %s\n", pid, pid)
		}
	}

	// Seed default roles if none exist.
	seedDefaultRoles()

	outln()
	outln("Run the commands above to authenticate each provider.")
	outln("After setup, run 'vibeusage' to see your usage.")
	return nil
}

// defaultRoles defines the starter roles seeded during init.
var defaultRoles = map[string]config.RoleConfig{
	"thinking": {Models: []string{"claude-opus-4-6", "o4", "gpt-5-2"}},
	"coding":   {Models: []string{"claude-sonnet-4-6", "gemini-3.1-pro-preview", "gpt-5"}},
	"fast":     {Models: []string{"claude-haiku-4-5", "gemini-3-flash", "gpt-4o-mini"}},
}

func seedDefaultRoles() {
	cfg := config.Get()
	if len(cfg.Roles) > 0 {
		return
	}

	for name, role := range defaultRoles {
		cfg.Roles[name] = role
	}

	if err := config.Save(cfg, ""); err != nil {
		// Non-fatal — roles are a convenience, not a requirement.
		return
	}
	// We just saved this config, so a parse error here would be unexpected.
	_, _ = config.Reload()

	outln()
	outln("  Default roles added to config:")
	outln("    thinking — deep reasoning models (Opus, o4, GPT-5.2)")
	outln("    coding   — agentic coding models (Sonnet, Codex, GPT-5)")
	outln("    fast     — quick lightweight models (Haiku, Flash, GPT-4o-mini)")
	outln()
	outln("  Customize roles with: vibeusage config edit")
	outln("  Route by role with:   vibeusage route --role thinking")
}
