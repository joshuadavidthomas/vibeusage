package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/charmbracelet/lipgloss"

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

		if showStatus {
			return authStatusCommand()
		}

		if len(args) == 0 {
			return authSetup()
		}

		providerID := args[0]
		p, ok := provider.Get(providerID)
		if !ok {
			return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
		}

		deleteFlag, _ := cmd.Flags().GetBool("delete")
		if deleteFlag {
			return authDeleteProvider(providerID)
		}

		token, _ := cmd.Flags().GetString("token")
		if token != "" {
			return authSetToken(providerID, p, token)
		}

		return authProvider(providerID, p)
	},
}

var providerDescriptions = map[string]string{
	"amp":         "Amp coding assistant (ampcode.com)",
	"antigravity": "Antigravity AI (antigravity.ai)",
	"claude":      "Anthropic's Claude AI assistant (claude.ai)",
	"codex":       "OpenAI's Codex/ChatGPT (platform.openai.com)",
	"copilot":     "GitHub Copilot (github.com)",
	"cursor":      "Cursor AI code editor (cursor.com)",
	"gemini":      "Google's Gemini AI (gemini.google.com)",
	"kimicode":    "Kimi Code coding assistant (kimi.com)",
	"minimax":     "MiniMax AI (minimax.io)",
	"openrouter":  "OpenRouter unified model gateway (openrouter.ai)",
	"warp":        "Warp terminal AI (warp.dev)",
	"zai":         "Z.ai coding assistant (z.ai)",
}

func init() {
	authCmd.Flags().Bool("status", false, "Show authentication status")
	authCmd.Flags().Bool("delete", false, "Delete credentials for a provider")
	authCmd.Flags().String("token", "", "Set a credential non-interactively")
}

// authSetup runs an interactive multi-select to pick and authenticate
// providers. Used when `vibeusage auth` is run with no configured providers.
func authSetup() error {
	if quiet {
		outln("Use 'vibeusage auth <provider>' to set up providers")
		return nil
	}

	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	enabledSet := make(map[string]bool)
	for _, id := range config.ReadEnabledProviders() {
		enabledSet[id] = true
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	options := make([]prompt.SelectOption, 0, len(allProviders))
	for _, pid := range allProviders {
		hasCreds, source := provider.CheckCredentials(pid)
		desc := providerDescriptions[pid]
		if desc == "" {
			desc = provider.DisplayName(pid)
		}
		label := pid + " — " + desc
		if hasCreds {
			label += " " + dim.Render("[detected: "+sourceToLabel(source)+"]")
		}
		options = append(options, prompt.SelectOption{
			Label:    label,
			Value:    pid,
			Selected: enabledSet[pid],
		})
	}

	title := "Choose providers to set up"
	if len(enabledSet) > 0 {
		title = "Manage enabled providers"
	}

	selected, err := prompt.Default.MultiSelect(prompt.MultiSelectConfig{
		Title:       title,
		Description: "Space to select, Enter to confirm",
		Options:     options,
		Validate: func(selected []string) error {
			if len(selected) == 0 {
				return errors.New("select at least one provider (use Space to toggle)")
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Only auth newly-selected providers; already-enabled ones stay as-is.
	var newProviders []string
	for _, pid := range selected {
		if !enabledSet[pid] {
			newProviders = append(newProviders, pid)
		}
	}

	// Build the final enabled set from the selection.
	// Deselected providers get removed; authProvider calls enableProvider
	// for successful new auths, so start with just the kept ones.
	selectedSet := make(map[string]bool, len(selected))
	for _, pid := range selected {
		selectedSet[pid] = true
	}
	var kept []string
	for _, pid := range config.ReadEnabledProviders() {
		if selectedSet[pid] {
			kept = append(kept, pid)
		} else {
			removeProviderCredentials(pid)
		}
	}
	_ = config.WriteEnabledProviders(kept)

	// Track removals for summary.
	var removed []string
	for id := range enabledSet {
		if !selectedSet[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(removed)

	outln()
	var failed []string
	for _, pid := range newProviders {
		p, ok := provider.Get(pid)
		if !ok {
			continue
		}
		if err := authProvider(pid, p); err != nil {
			out("✗ %s: %v\n", pid, err)
			failed = append(failed, pid)
		}
	}

	// Summary
	outln()
	finalEnabled := config.ReadEnabledProviders()
	if len(finalEnabled) > 0 {
		out("Enabled: %s\n", strings.Join(finalEnabled, ", "))
	}
	if len(removed) > 0 {
		out("Removed: %s\n", strings.Join(removed, ", "))
	}
	if len(failed) > 0 {
		out("Failed:  %s\n", strings.Join(failed, ", "))
		outln("Retry with: vibeusage auth <provider>")
	}
	return nil
}

func authStatusCommand() error {
	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	enabledSet := make(map[string]bool)
	for _, id := range config.ReadEnabledProviders() {
		enabledSet[id] = true
	}

	if jsonOutput {
		data := make(map[string]display.AuthStatusEntryJSON)
		for _, pid := range allProviders {
			hasCreds, source := provider.CheckCredentials(pid)
			data[pid] = display.AuthStatusEntryJSON{
				Authenticated: hasCreds,
				Source:        sourceToLabel(source),
				Enabled:       enabledSet[pid],
			}
		}
		return display.OutputJSON(outWriter, data)
	}

	if quiet {
		for _, pid := range allProviders {
			hasCreds, _ := provider.CheckCredentials(pid)
			status := "not configured"
			if hasCreds && enabledSet[pid] {
				status = "enabled"
			} else if hasCreds {
				status = "authenticated (not enabled)"
			}
			out("%s: %s\n", pid, status)
		}
		return nil
	}

	var rows [][]string
	var unenabled []string
	for _, pid := range allProviders {
		hasCreds, source := provider.CheckCredentials(pid)
		if hasCreds && enabledSet[pid] {
			rows = append(rows, []string{pid, "✓ Enabled", sourceToLabel(source)})
		} else if hasCreds {
			rows = append(rows, []string{pid, "✓ Authenticated", sourceToLabel(source)})
			unenabled = append(unenabled, pid)
		} else {
			rows = append(rows, []string{pid, "✗ Not configured", "—"})
			unenabled = append(unenabled, pid)
		}
	}

	outln(display.NewTableWithOptions(
		[]string{"Provider", "Status", "Source"},
		rows,
		display.TableOptions{Title: "Authentication Status", NoColor: noColor, Width: display.TerminalWidth()},
	))

	if len(unenabled) > 0 {
		outln()
		outln("To enable a provider, run:")
		for _, pid := range unenabled {
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

// enableProvider adds a provider to the enabled list in the data directory,
// making provider tracking opt-in via the auth command.
func enableProvider(providerID string) {
	config.AddEnabledProvider(providerID)
}

// removeProviderCredentials deletes all vibeusage-stored credentials for a provider.
func removeProviderCredentials(providerID string) {
	for _, credType := range []string{"oauth", "session", "apikey"} {
		config.DeleteCredential(config.CredentialPath(providerID, credType))
	}
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
			Negative:    "No, enter manually",
			Default:     true,
		})
		if err != nil {
			return err
		}
		if useExisting {
			// Verify the detected credentials actually work.
			if verifyCredentialsFn(providerID) {
				return nil
			}
			if !quiet {
				out("✗ Detected credentials are expired or invalid, re-authenticating...\n")
			}
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

// HACK: package-level var to allow test stubbing. This should be replaced
// with a proper interface (e.g. a Verifier on the auth command struct) once
// the CLI is refactored away from package-level state.
var verifyCredentialsFn = verifyCredentialsDefault

func verifyCredentialsDefault(providerID string) bool {
	p, ok := provider.Get(providerID)
	if !ok {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range p.FetchStrategies() {
		if !s.IsAvailable() {
			continue
		}
		result, err := s.Fetch(ctx)
		if err != nil {
			continue
		}
		return result.Success
	}
	return false
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
			Negative:    "No, enter manually",
			Default:     true,
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
		enableProvider(providerID)
		return nil
	}

	if quiet {
		return fmt.Errorf("no auth flow for %s; set credentials with --token or an environment variable", providerID)
	}

	out("%s Authentication\n\n", provider.DisplayName(providerID))

	value, err := prompt.Default.Input(prompt.InputConfig{
		Title:       fmt.Sprintf("%s credential", provider.DisplayName(providerID)),
		Placeholder: "paste credential here",
		Validate:    provider.ValidateNotEmpty,
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"api_key": value})
	path := config.CredentialPath(providerID, "apikey")
	if err := config.WriteCredential(path, credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if !quiet {
		out("✓ %s credential saved\n", provider.DisplayName(providerID))
	}
	enableProvider(providerID)
	return nil
}

// authDeleteProvider removes credentials and disables a provider.
func authDeleteProvider(providerID string) error {
	if !quiet {
		ok, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: fmt.Sprintf("Delete %s credentials?", provider.DisplayName(providerID)),
		})
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	removeProviderCredentials(providerID)

	// Remove from enabled list.
	enabled := config.ReadEnabledProviders()
	var kept []string
	for _, id := range enabled {
		if id != providerID {
			kept = append(kept, id)
		}
	}
	_ = config.WriteEnabledProviders(kept)

	config.ClearProviderCache(providerID)

	if !quiet {
		out("✓ Deleted credentials for %s\n", provider.DisplayName(providerID))
	}
	return nil
}

// authSetToken sets a credential non-interactively via --token and enables
// the provider. Uses the provider's ManualKeyAuthFlow if available for
// proper validation and storage, otherwise falls back to generic storage.
func authSetToken(providerID string, p provider.Provider, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("credential cannot be empty")
	}

	auth, ok := p.(provider.Authenticator)
	if ok {
		flow := auth.Auth()
		if f, isManual := flow.(provider.ManualKeyAuthFlow); isManual {
			if f.Validate != nil {
				if err := f.Validate(token); err != nil {
					return err
				}
			}
			if f.Save != nil {
				if err := f.Save(token); err != nil {
					return fmt.Errorf("error saving credential: %w", err)
				}
			} else if f.JSONKey != "" {
				credData, _ := json.Marshal(map[string]string{f.JSONKey: token})
				if err := config.WriteCredential(f.CredPath, credData); err != nil {
					return fmt.Errorf("error saving credential: %w", err)
				}
			}
			enableProvider(providerID)
			if !quiet {
				out("✓ %s credential saved\n", provider.DisplayName(providerID))
			}
			return nil
		}
	}

	// Fallback for providers without a ManualKeyAuthFlow.
	credData, _ := json.Marshal(map[string]string{"api_key": token})
	path := config.CredentialPath(providerID, "apikey")
	if err := config.WriteCredential(path, credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}
	enableProvider(providerID)
	if !quiet {
		out("✓ %s credential saved\n", provider.DisplayName(providerID))
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
