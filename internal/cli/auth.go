package cli

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
		p, ok := provider.Get(providerID)
		if !ok {
			return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
		}

		return authProvider(providerID, p)
	},
}

func init() {
	authCmd.Flags().Bool("status", false, "Show authentication status")
}

func authStatusCommand() error {
	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	if jsonOutput {
		data := make(map[string]display.AuthStatusEntryJSON)
		for _, pid := range allProviders {
			hasCreds, source := provider.CheckCredentials(pid)
			data[pid] = display.AuthStatusEntryJSON{
				Authenticated: hasCreds,
				Source:        sourceToLabel(source),
			}
		}
		return display.OutputJSON(outWriter, data)
	}

	if quiet {
		for _, pid := range allProviders {
			hasCreds, _ := provider.CheckCredentials(pid)
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
		hasCreds, source := provider.CheckCredentials(pid)
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

// authProvider dispatches to the appropriate auth flow based on what the
// provider declares via the Authenticator interface. On success, the
// provider is added to enabled_providers so only explicitly authed
// providers are tracked.
func authProvider(providerID string, p provider.Provider) error {
	auth, ok := p.(provider.Authenticator)
	if !ok {
		return authGeneric(providerID)
	}

	flow := auth.Auth()
	if flow == nil {
		return authGeneric(providerID)
	}

	var err error
	switch f := flow.(type) {
	case provider.DeviceAuthFlow:
		err = authDeviceFlow(providerID, f)
	case provider.ManualKeyAuthFlow:
		err = authManualKey(providerID, f)
	default:
		return authGeneric(providerID)
	}

	if err == nil {
		enableProvider(providerID)
	}
	return err
}

// enableProvider adds a provider to the enabled_providers list in config,
// making provider tracking opt-in via the auth command.
func enableProvider(providerID string) {
	cfg, err := config.Load("")
	if err != nil {
		return
	}
	for _, id := range cfg.EnabledProviders {
		if id == providerID {
			return
		}
	}
	cfg.EnabledProviders = append(cfg.EnabledProviders, providerID)
	sort.Strings(cfg.EnabledProviders)
	if err := config.Save(cfg, ""); err != nil {
		return
	}
	config.SetGlobal(cfg)
}

// authDeviceFlow runs an OAuth/device-code flow with detected-credential check.
func authDeviceFlow(providerID string, flow provider.DeviceAuthFlow) error {
	hasCreds, source := provider.CheckCredentials(providerID)
	if hasCreds && !quiet {
		out("✓ %s credentials detected (%s)\n",
			provider.DisplayName(providerID), sourceToLabel(source))

		useExisting, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title:       "Use detected credentials?",
			Affirmative: "Yes",
			Negative:    "No, enter new",
		})
		if err != nil {
			return err
		}
		if useExisting {
			return nil
		}
	}

	success, err := flow.Authenticate(outWriter, quiet)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// authManualKey runs an interactive manual-key input flow.
func authManualKey(providerID string, flow provider.ManualKeyAuthFlow) error {
	hasCreds, source := provider.CheckCredentials(providerID)
	if hasCreds && !quiet {
		out("✓ %s credentials detected (%s)\n",
			provider.DisplayName(providerID), sourceToLabel(source))

		useExisting, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title:       "Use detected credentials?",
			Affirmative: "Yes",
			Negative:    "No, enter new",
		})
		if err != nil {
			return err
		}
		if useExisting {
			return nil
		}
	}

	if !quiet {
		out("%s Authentication\n\n", provider.DisplayName(providerID))
		outln(flow.Instructions)
		outln()
	}

	title := "Credential"
	if flow.JSONKey != "" {
		title = strings.ToUpper(flow.JSONKey[:1]) + flow.JSONKey[1:]
	}

	value, err := prompt.Default.Input(prompt.InputConfig{
		Title:       title,
		Placeholder: flow.Placeholder,
		Validate:    flow.Validate,
	})
	if err != nil {
		return err
	}

	if flow.Save != nil {
		if err := flow.Save(value); err != nil {
			return fmt.Errorf("error saving credential: %w", err)
		}
	} else {
		credData, _ := json.Marshal(map[string]string{flow.JSONKey: value})
		if err := config.WriteCredential(flow.CredPath, credData); err != nil {
			return fmt.Errorf("error saving credential: %w", err)
		}
	}

	if !quiet {
		out("✓ %s credential saved\n", provider.DisplayName(providerID))
	}
	return nil
}

func authGeneric(providerID string) error {
	hasCreds, source := provider.CheckCredentials(providerID)

	if hasCreds {
		if !quiet {
			out("✓ %s is already authenticated (%s)\n",
				provider.DisplayName(providerID), sourceToLabel(source))
		}
		return nil
	}

	if !quiet {
		out("%s Authentication\n\n", provider.DisplayName(providerID))
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
