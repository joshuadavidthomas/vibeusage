package cli

import (
	"sort"
	"strings"

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
	initCmd.Flags().Bool("quick", false, "Quick setup with default provider (Claude)")
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

	p, ok := provider.Get("claude")
	if !ok {
		outln("Claude provider not available.")
		return nil
	}

	if err := authProvider("claude", p); err != nil {
		out("✗ Claude setup failed: %v\n", err)
		outln("  Retry with: vibeusage auth claude")
		return nil
	}

	// Save claude as an enabled provider.
	saveEnabledProviders([]string{"claude"})

	outln()
	outln("✓ Claude is ready!")
	outln("Run 'vibeusage' to see your usage.")
	outln("Add more providers with: vibeusage init")
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
		outln("  vibeusage init")
		return nil
	}

	// Save selected providers as the enabled set.
	saveEnabledProviders(selected)

	// Authenticate each selected provider inline.
	outln()
	var succeeded, failed, skipped []string
	for _, pid := range selected {
		p, ok := provider.Get(pid)
		if !ok {
			continue
		}

		hasCreds, _ := provider.CheckCredentials(pid)
		if hasCreds {
			out("  ✓ %s already configured\n", pid)
			skipped = append(skipped, pid)
			continue
		}

		outln()
		if err := authProvider(pid, p); err != nil {
			out("  ✗ %s: %v\n", pid, err)
			failed = append(failed, pid)
		} else {
			succeeded = append(succeeded, pid)
		}
	}

	// Seed default roles if none exist.
	seedDefaultRoles()

	// Print summary.
	outln()
	if len(succeeded) > 0 {
		out("  ✓ Authenticated: %s\n", strings.Join(succeeded, ", "))
	}
	if len(skipped) > 0 {
		out("  ✓ Already configured: %s\n", strings.Join(skipped, ", "))
	}
	if len(failed) > 0 {
		out("  ✗ Failed: %s\n", strings.Join(failed, ", "))
		outln("    Retry with: vibeusage init")
	}
	outln()
	outln("  Run 'vibeusage' to see your usage.")
	return nil
}

// saveEnabledProviders persists the selected provider IDs into config so only
// those providers are tracked. This makes provider tracking opt-in.
func saveEnabledProviders(providerIDs []string) {
	cfg, err := config.Load("")
	if err != nil {
		return
	}
	sort.Strings(providerIDs)
	cfg.EnabledProviders = providerIDs
	if err := config.Save(cfg, ""); err != nil {
		return
	}
	config.SetGlobal(cfg)
}

func seedDefaultRoles() {
	if !config.SeedDefaultRoles() {
		return
	}

	roles := config.DefaultRoles()
	outln()
	outln("  Default roles added to config:")
	outln("    thinking — deep reasoning models (" + joinModelNames(roles["thinking"].Models) + ")")
	outln("    coding   — agentic coding models (" + joinModelNames(roles["coding"].Models) + ")")
	outln("    fast     — quick lightweight models (" + joinModelNames(roles["fast"].Models) + ")")
	outln()
	outln("  Customize roles with: vibeusage config edit")
	outln("  Route by role with:   vibeusage route --role thinking")
}

func joinModelNames(models []string) string {
	return strings.Join(models, ", ")
}
