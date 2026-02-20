package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
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
		fmt.Println("Run 'vibeusage auth claude' to set up Claude")
		return nil
	}

	fmt.Println("Quick Setup: Claude")
	fmt.Println("Claude is the most popular AI assistant for agentic workflows.")
	fmt.Println()

	hasCreds, _ := config.CheckProviderCredentials("claude")
	if hasCreds {
		fmt.Println("✓ Claude is already configured!")
		fmt.Println("\nRun 'vibeusage' to see your usage.")
		return nil
	}

	fmt.Println("To set up Claude, run:")
	fmt.Println("  vibeusage auth claude")
	fmt.Println()
	fmt.Println("After setup, run 'vibeusage' to see your usage.")
	return nil
}

func interactiveWizard() error {
	if quiet {
		fmt.Println("Use 'vibeusage auth <provider>' to set up providers")
		return nil
	}

	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	// Welcome
	fmt.Println()
	fmt.Println("  ✨ Welcome to vibeusage!")
	fmt.Println()
	fmt.Println("  Track your usage across AI providers in one place.")
	fmt.Println()

	// Step 1: Show providers
	fmt.Println("Step 1: Choose providers to set up")
	fmt.Println()

	for i, pid := range allProviders {
		hasCreds, _ := config.CheckProviderCredentials(pid)
		status := "  "
		if hasCreds {
			status = "✓ "
		}
		desc := providerDescriptions[pid]
		if desc == "" {
			desc = strings.Title(pid) + " AI"
		}
		fmt.Printf("  %d. %s%-10s %s\n", i+1, status, pid, desc)
	}

	fmt.Println()
	fmt.Println("Enter provider numbers separated by spaces (e.g., '1 3 5')")
	fmt.Println("Press Enter to skip setup")

	fmt.Print("Providers: ")
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "" {
		fmt.Println("\nNo providers selected. You can set up providers later:")
		for _, pid := range allProviders {
			fmt.Printf("  vibeusage auth %s\n", pid)
		}
		return nil
	}

	// Parse selection
	parts := strings.Fields(input)
	var selected []string
	for _, p := range parts {
		idx, err := strconv.Atoi(p)
		if err != nil || idx < 1 || idx > len(allProviders) {
			continue
		}
		selected = append(selected, allProviders[idx-1])
	}

	if len(selected) == 0 {
		fmt.Println("No valid providers selected.")
		return nil
	}

	// Step 2: Show setup commands
	fmt.Printf("\nStep 2: Set up %d provider(s)\n\n", len(selected))

	for _, pid := range selected {
		hasCreds, _ := config.CheckProviderCredentials(pid)
		if hasCreds {
			fmt.Printf("  ✓ %s already configured\n", pid)
		} else {
			fmt.Printf("  → %s: vibeusage auth %s\n", pid, pid)
		}
	}

	fmt.Println()
	fmt.Println("Run the commands above to authenticate each provider.")
	fmt.Println("After setup, run 'vibeusage' to see your usage.")
	return nil
}
