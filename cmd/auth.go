package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/antigravity"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/copilot"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/kimi"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/minimax"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
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
		case "antigravity":
			return authAntigravity()
		case "claude":
			return authClaude()
		case "cursor":
			return authCursor()
		case "copilot":
			return authCopilot()
		case "kimi":
			return authKimi()
		case "minimax":
			return authMinimax()
		case "zai":
			return authZai()
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
		display.OutputJSON(outWriter, data)
		return nil
	}

	if quiet {
		for _, pid := range allProviders {
			hasCreds, _ := config.CheckProviderCredentials(pid)
			status := "not configured"
			if hasCreds {
				status = "authenticated"
			}
			out("%s: %s\n", pid, status)
		}
		return nil
	}

	var rows [][]string
	var unconfigured []string
	for _, pid := range allProviders {
		hasCreds, source := config.CheckProviderCredentials(pid)
		if hasCreds {
			rows = append(rows, []string{pid, "✓ Authenticated", sourceToLabel(source)})
		} else {
			rows = append(rows, []string{pid, "✗ Not configured", "—"})
			unconfigured = append(unconfigured, pid)
		}
	}

	outln(display.NewTableWithOptions(
		[]string{"Provider", "Status", "Source"},
		rows,
		display.TableOptions{Title: "Authentication Status", NoColor: noColor, Width: display.TerminalWidth()},
	))

	if len(unconfigured) > 0 {
		outln()
		outln("To configure a provider, run:")
		for _, pid := range unconfigured {
			out("  vibeusage auth %s\n", pid)
		}
	}

	return nil
}

func authAntigravity() error {
	hasCreds, source := config.CheckProviderCredentials("antigravity")
	if hasCreds && !quiet {
		out("✓ Antigravity is already authenticated (%s)\n", sourceToLabel(source))

		reauth, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: "Re-authenticate?",
		})
		if err != nil {
			return err
		}
		if !reauth {
			return nil
		}
	}

	success, err := antigravity.RunAuthFlow(outWriter, quiet)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

func authClaude() error {
	if !quiet {
		outln("Claude Authentication")
		outln()
		outln("Get your session key from claude.ai:")
		outln("  1. Open https://claude.ai in your browser")
		outln("  2. Open DevTools (F12 or Cmd+Option+I)")
		outln("  3. Go to Application → Cookies → https://claude.ai")
		outln("  4. Find the sessionKey cookie")
		outln("  5. Copy its value (starts with sk-ant-sid01-)")
		outln()
	}

	sessionKey, err := prompt.Default.Input(prompt.InputConfig{
		Title:       "Session key",
		Placeholder: "sk-ant-sid01-...",
		Validate:    prompt.ValidateClaudeSessionKey,
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"session_key": sessionKey})
	if err := config.WriteCredential(config.CredentialPath("claude", "session"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		outln("✓ Claude session key saved")
	}
	return nil
}

func authCursor() error {
	if !quiet {
		outln("Cursor Authentication")
		outln()
		outln("Get your session token from cursor.com:")
		outln("  1. Open https://cursor.com in your browser")
		outln("  2. Open DevTools (F12 or Cmd+Option+I)")
		outln("  3. Go to Application → Cookies → https://cursor.com")
		outln("  4. Find one of: WorkosCursorSessionToken, __Secure-next-auth.session-token")
		outln("  5. Copy its value")
		outln()
	}

	sessionToken, err := prompt.Default.Input(prompt.InputConfig{
		Title:       "Session token",
		Placeholder: "paste token here",
		Validate:    prompt.ValidateNotEmpty,
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"session_token": sessionToken})
	if err := config.WriteCredential(config.CredentialPath("cursor", "session"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		outln("✓ Cursor session token saved")
	}
	return nil
}

func authCopilot() error {
	hasCreds, source := config.CheckProviderCredentials("copilot")
	if hasCreds && !quiet {
		out("✓ Copilot is already authenticated (%s)\n", sourceToLabel(source))

		reauth, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: "Re-authenticate?",
		})
		if err != nil {
			return err
		}
		if !reauth {
			return nil
		}
	}

	success, err := copilot.RunDeviceFlow(outWriter, quiet)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

func authKimi() error {
	hasCreds, source := config.CheckProviderCredentials("kimi")
	if hasCreds && !quiet {
		out("✓ Kimi is already authenticated (%s)\n", sourceToLabel(source))

		reauth, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: "Re-authenticate?",
		})
		if err != nil {
			return err
		}
		if !reauth {
			return nil
		}
	}

	success, err := kimi.RunDeviceFlow(outWriter, quiet)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

func authMinimax() error {
	hasCreds, source := config.CheckProviderCredentials("minimax")
	if hasCreds && !quiet {
		out("✓ Minimax is already authenticated (%s)\n", sourceToLabel(source))

		reauth, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: "Re-authenticate?",
		})
		if err != nil {
			return err
		}
		if !reauth {
			return nil
		}
	}

	if !quiet {
		outln("Minimax Authentication")
		outln()
		outln("Get your Coding Plan API key from Minimax:")
		outln("  1. Open https://platform.minimax.io/user-center/payment/coding-plan")
		outln("  2. Copy your Coding Plan API key (starts with sk-cp-)")
		outln()
		outln("Note: Standard API keys (sk-api-) won't work — you need a Coding Plan key.")
		outln()
	}

	apiKey, err := prompt.Default.Input(prompt.InputConfig{
		Title:       "Coding Plan API key",
		Placeholder: "sk-cp-...",
		Validate: func(s string) error {
			return minimax.ValidateCodingPlanKey(s)
		},
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"api_key": apiKey})
	if err := config.WriteCredential(config.CredentialPath("minimax", "apikey"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		outln("✓ Minimax Coding Plan API key saved")
	}
	return nil
}

func authZai() error {
	hasCreds, source := config.CheckProviderCredentials("zai")
	if hasCreds && !quiet {
		out("✓ Z.ai is already authenticated (%s)\n", sourceToLabel(source))

		reauth, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: "Re-authenticate?",
		})
		if err != nil {
			return err
		}
		if !reauth {
			return nil
		}
	}

	if !quiet {
		outln("Z.ai Authentication")
		outln()
		outln("Get your API key from Z.ai:")
		outln("  1. Open https://z.ai/manage-apikey/apikey-list")
		outln("  2. Create a new API key (or copy an existing one)")
		outln()
	}

	apiKey, err := prompt.Default.Input(prompt.InputConfig{
		Title:       "API key",
		Placeholder: "paste API key here",
		Validate:    prompt.ValidateNotEmpty,
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"api_key": apiKey})
	if err := config.WriteCredential(config.CredentialPath("zai", "apikey"), credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		outln("✓ Z.ai API key saved")
	}
	return nil
}

func authGeneric(providerID string) error {
	hasCreds, source := config.CheckProviderCredentials(providerID)

	if hasCreds {
		if !quiet {
			out("✓ %s is already authenticated (%s)\n",
				strutil.TitleCase(providerID), sourceToLabel(source))
		}
		return nil
	}

	if !quiet {
		out("%s Authentication\n\n", strutil.TitleCase(providerID))
		outln("Set credentials manually:")
		out("  vibeusage key %s set\n", providerID)
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
