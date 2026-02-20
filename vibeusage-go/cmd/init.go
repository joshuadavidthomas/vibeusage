package cmd

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

var providerDescriptions = map[string]string{
	"claude":  "Anthropic's Claude AI assistant (claude.ai)",
	"codex":   "OpenAI's Codex/ChatGPT (platform.openai.com)",
	"copilot": "GitHub Copilot (github.com)",
	"cursor":  "Cursor AI code editor (cursor.com)",
	"gemini":  "Google's Gemini AI (gemini.google.com)",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Run first-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		quick, _ := cmd.Flags().GetBool("quick")

		if jsonOutput {
			display.OutputJSON(map[string]any{
				"first_run":            config.IsFirstRun(),
				"configured_providers": config.CountConfiguredProviders(),
				"available_providers":  provider.ListIDs(),
			})
			return nil
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

	hasCreds, _ := config.CheckProviderCredentials("claude")
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
		hasCreds, _ := config.CheckProviderCredentials(pid)
		status := ""
		if hasCreds {
			status = "✓ "
		}
		desc := providerDescriptions[pid]
		if desc == "" {
			desc = strutil.TitleCase(pid) + " AI"
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
		hasCreds, _ := config.CheckProviderCredentials(pid)
		if hasCreds {
			out("  ✓ %s already configured\n", pid)
		} else {
			out("  → %s: vibeusage auth %s\n", pid, pid)
		}
	}

	outln()
	outln("Run the commands above to authenticate each provider.")
	outln("After setup, run 'vibeusage' to see your usage.")
	return nil
}
