package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/copilot"
)

var authCmd = &cobra.Command{
	Use:   "auth [provider]",
	Short: "Authenticate with a provider or show auth status",
	RunE: func(cmd *cobra.Command, args []string) error {
		showStatus, _ := cmd.Flags().GetBool("status")

		if showStatus || len(args) == 0 {
			return authStatusCommand()
		}

		providerID := args[0]
		if _, ok := provider.Get(providerID); !ok {
			return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
		}

		switch providerID {
		case "claude":
			return authClaude()
		case "cursor":
			return authCursor()
		case "copilot":
			return authCopilot()
		default:
			return authGeneric(providerID)
		}
	},
}

func init() {
	authCmd.Flags().Bool("status", false, "Show authentication status")
}

func authStatusCommand() error {
	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	if jsonOutput {
		data := make(map[string]any)
		for _, pid := range allProviders {
			hasCreds, source := config.CheckProviderCredentials(pid)
			sourceLabel := sourceToLabel(source)
			data[pid] = map[string]any{
				"authenticated": hasCreds,
				"source":        sourceLabel,
			}
		}
		display.OutputJSON(data)
		return nil
	}

	if quiet {
		for _, pid := range allProviders {
			hasCreds, _ := config.CheckProviderCredentials(pid)
			status := "not configured"
			if hasCreds {
				status = "authenticated"
			}
			fmt.Printf("%s: %s\n", pid, status)
		}
		return nil
	}

	fmt.Println("Authentication Status")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("%-12s %-16s %s\n", "Provider", "Status", "Source")
	fmt.Println(strings.Repeat("─", 60))

	var unconfigured []string
	for _, pid := range allProviders {
		hasCreds, source := config.CheckProviderCredentials(pid)
		if hasCreds {
			fmt.Printf("%-12s %-16s %s\n", pid, "✓ Authenticated", sourceToLabel(source))
		} else {
			fmt.Printf("%-12s %-16s %s\n", pid, "✗ Not configured", "—")
			unconfigured = append(unconfigured, pid)
		}
	}

	if len(unconfigured) > 0 {
		fmt.Println("\nTo configure a provider, run:")
		for _, pid := range unconfigured {
			fmt.Printf("  vibeusage auth %s\n", pid)
		}
	}

	return nil
}

func authClaude() error {
	if !quiet {
		fmt.Println("Claude Authentication")
		fmt.Println()
		fmt.Println("Get your session key from claude.ai:")
		fmt.Println("  1. Open https://claude.ai in your browser")
		fmt.Println("  2. Open DevTools (F12 or Cmd+Option+I)")
		fmt.Println("  3. Go to Application → Cookies → https://claude.ai")
		fmt.Println("  4. Find the sessionKey cookie")
		fmt.Println("  5. Copy its value (starts with sk-ant-sid01-)")
		fmt.Println()
	}

	fmt.Print("Session key: ")
	var sessionKey string
	fmt.Scanln(&sessionKey)
	sessionKey = strings.TrimSpace(sessionKey)

	if sessionKey == "" {
		return fmt.Errorf("session key cannot be empty")
	}

	if !strings.HasPrefix(sessionKey, "sk-ant-sid01-") && !quiet {
		fmt.Println("Warning: Session key doesn't match expected format (sk-ant-sid01-...)")
		fmt.Print("Save anyway? [y/N] ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return nil
		}
	}

	credData, _ := json.Marshal(map[string]string{"session_key": sessionKey})
	if err := config.WriteCredential(config.CredentialPath("claude", "session"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		fmt.Println("✓ Claude session key saved")
	}
	return nil
}

func authCursor() error {
	if !quiet {
		fmt.Println("Cursor Authentication")
		fmt.Println()
		fmt.Println("Get your session token from cursor.com:")
		fmt.Println("  1. Open https://cursor.com in your browser")
		fmt.Println("  2. Open DevTools (F12 or Cmd+Option+I)")
		fmt.Println("  3. Go to Application → Cookies → https://cursor.com")
		fmt.Println("  4. Find one of: WorkosCursorSessionToken, __Secure-next-auth.session-token")
		fmt.Println("  5. Copy its value")
		fmt.Println()
	}

	fmt.Print("Session token: ")
	var sessionToken string
	fmt.Scanln(&sessionToken)
	sessionToken = strings.TrimSpace(sessionToken)

	if sessionToken == "" {
		return fmt.Errorf("session token cannot be empty")
	}

	credData, _ := json.Marshal(map[string]string{"session_token": sessionToken})
	if err := config.WriteCredential(config.CredentialPath("cursor", "session"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		fmt.Println("✓ Cursor session token saved")
	}
	return nil
}

func authCopilot() error {
	// Check if already authenticated
	hasCreds, source := config.CheckProviderCredentials("copilot")
	if hasCreds && !quiet {
		fmt.Printf("✓ Copilot is already authenticated (%s)\n", sourceToLabel(source))
		fmt.Print("Re-authenticate? [y/N] ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return nil
		}
	}

	success, err := copilot.RunDeviceFlow(quiet)
	if err != nil {
		return err
	}
	if !success {
		os.Exit(2)
	}
	return nil
}

func authGeneric(providerID string) error {
	hasCreds, source := config.CheckProviderCredentials(providerID)

	if hasCreds {
		if !quiet {
			fmt.Printf("✓ %s is already authenticated (%s)\n", strings.Title(providerID), sourceToLabel(source))
		}
		return nil
	}

	if !quiet {
		fmt.Printf("%s Authentication\n\n", strings.Title(providerID))
		fmt.Printf("Set credentials manually:\n")
		fmt.Printf("  vibeusage key %s set\n", providerID)
	}
	return nil
}

func sourceToLabel(source string) string {
	switch source {
	case "vibeusage":
		return "vibeusage storage"
	case "provider_cli":
		return "provider CLI"
	case "env":
		return "environment variable"
	default:
		return source
	}
}
