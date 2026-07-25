package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/device"
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
	"opencode":    "OpenCode AI coding agent (opencode.ai)",
	"openrouter":  "OpenRouter unified model gateway (openrouter.ai)",
	"warp":        "Warp terminal AI (warp.dev)",
	"zai":         "Z.ai coding assistant (z.ai)",
}

func init() {
	authCmd.Flags().Bool("status", false, "Show authentication status")
	authCmd.Flags().Bool("delete", false, "Remove a provider and its vibeusage-stored credentials")
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

	cfg := config.Get()
	detectedSet := make(map[string]bool)
	configuredSet := make(map[string]bool)
	for _, pid := range allProviders {
		hasCreds, _ := provider.CheckCredentials(pid)
		if hasCreds {
			detectedSet[pid] = true
			if cfg.IsProviderEnabled(pid) {
				configuredSet[pid] = true
			}
		}
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
			Selected: configuredSet[pid],
		})
	}

	title := "Choose providers to set up"
	if len(configuredSet) > 0 {
		title = "Manage configured providers"
	}

	selected, err := prompt.Default.MultiSelect(prompt.MultiSelectConfig{
		Title:       title,
		Description: "Space to select, Enter to confirm, Esc to cancel",
		Options:     options,
	})
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	// Only auth newly-selected providers; already-configured ones stay as-is.
	selectedSet := make(map[string]bool, len(selected))
	for _, pid := range selected {
		selectedSet[pid] = true
	}

	var newProviders []string
	for _, pid := range selected {
		if !detectedSet[pid] {
			newProviders = append(newProviders, pid)
		}
	}

	states := make(map[string]bool, len(detectedSet))
	for pid := range detectedSet {
		states[pid] = selectedSet[pid]
	}
	if err := config.SetProvidersEnabled(states); err != nil {
		return fmt.Errorf("saving provider selection: %w", err)
	}

	// Remove vibeusage-owned credentials and cached data for deselected
	// providers. External CLI and environment credentials remain in place but
	// the persisted exclusion keeps them out of automatic fetches.
	var removed []string
	for pid := range configuredSet {
		if !selectedSet[pid] {
			if _, err := config.DeleteProviderCredentials(pid); err != nil {
				return fmt.Errorf("removing stored credentials for %s: %w", pid, err)
			}
			config.ClearProviderCache(pid)
			removed = append(removed, pid)
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
	var configured []string
	cfg = config.Get()
	for _, pid := range allProviders {
		hasCreds, _ := provider.CheckCredentials(pid)
		if hasCreds && cfg.IsProviderEnabled(pid) {
			configured = append(configured, pid)
		}
	}
	if len(configured) > 0 {
		out("Configured: %s\n", strings.Join(configured, ", "))
	}
	if len(removed) > 0 {
		out("Removed:    %s\n", strings.Join(removed, ", "))
	}
	if len(failed) > 0 {
		out("Failed:     %s\n", strings.Join(failed, ", "))
		outln("Retry with: vibeusage auth <provider>")
	}
	return nil
}

func authStatusCommand() error {
	allProviders := provider.ListIDs()
	sort.Strings(allProviders)

	cfg := config.Get()

	if jsonOutput {
		data := make(map[string]display.AuthStatusEntryJSON)
		for _, pid := range allProviders {
			hasCreds, source := provider.CheckCredentials(pid)
			data[pid] = display.AuthStatusEntryJSON{
				Authenticated: hasCreds,
				Source:        sourceToLabel(source),
				Disabled:      !cfg.IsProviderEnabled(pid),
			}
		}
		return display.OutputJSON(outWriter, data)
	}

	if quiet {
		for _, pid := range allProviders {
			hasCreds, _ := provider.CheckCredentials(pid)
			status := "not configured"
			if hasCreds && cfg.IsProviderEnabled(pid) {
				status = "configured"
			} else if hasCreds {
				status = "configured (disabled in config)"
			}
			out("%s: %s\n", pid, status)
		}
		return nil
	}

	var rows [][]string
	var unconfigured []string
	for _, pid := range allProviders {
		hasCreds, source := provider.CheckCredentials(pid)
		if hasCreds && cfg.IsProviderEnabled(pid) {
			rows = append(rows, []string{pid, "✓ Configured", sourceToLabel(source)})
		} else if hasCreds {
			rows = append(rows, []string{pid, "✓ Configured (disabled)", sourceToLabel(source)})
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
		outln("To set up a provider, run:")
		for _, pid := range unconfigured {
			out("  vibeusage auth %s\n", pid)
		}
	}

	return nil
}

// authProvider dispatches to the appropriate auth flow based on what the
// provider declares via the Authenticator interface.
func authProvider(providerID string, p provider.Provider) error {
	auth, ok := p.(provider.Authenticator)
	if !ok {
		return authGeneric(providerID)
	}

	flow := auth.Auth()
	if flow == nil {
		return authGeneric(providerID)
	}

	// Show provider heading.
	if !quiet {
		bold := lipgloss.NewStyle().Bold(true)
		out("%s\n\n", bold.Render(provider.DisplayName(providerID)))
	}

	// Offer to reuse existing credentials before running the flow.
	// Device/custom flows verify tokens (they can expire); manual key flows accept as-is.
	_, isDevice := flow.(provider.DeviceAuthFlow)
	_, isCustom := flow.(provider.CustomAuthFlow)
	verify := isDevice || isCustom
	if skip, err := offerExistingCredentials(providerID, verify); err != nil {
		return err
	} else if skip {
		return enableProvider(providerID)
	}

	// Run the flow.
	var err error
	switch f := flow.(type) {
	case provider.DeviceAuthFlow:
		var success bool
		success, err = device.Run(outWriter, quiet, f.Config)
		if err == nil && !success {
			err = fmt.Errorf("authentication failed")
		}
	case provider.CustomAuthFlow:
		var success bool
		success, err = f.RunFlow(outWriter, quiet)
		if err == nil && !success {
			err = fmt.Errorf("authentication failed")
		}
	case provider.ManualKeyAuthFlow:
		err = authManualKey(providerID, f)
	default:
		return authGeneric(providerID)
	}

	if err != nil {
		return err
	}
	return enableProvider(providerID)
}

// offerExistingCredentials checks for detected credentials and asks the user
// whether to reuse them. When verify is true, credentials are tested via a
// real fetch before accepting. Returns (true, nil) if the caller should skip
// the auth flow.
func offerExistingCredentials(providerID string, verify bool) (bool, error) {
	hasCreds, source := provider.CheckCredentials(providerID)
	if !hasCreds || quiet {
		return false, nil
	}

	out("✓ %s credentials detected (%s)\n",
		provider.DisplayName(providerID), sourceToLabel(source))

	useExisting, err := prompt.Default.Confirm(prompt.ConfirmConfig{
		Title:       "Use detected credentials?",
		Affirmative: "Yes",
		Negative:    "No, enter manually",
		Default:     true,
	})
	if err != nil {
		return false, err
	}
	if !useExisting {
		return false, nil
	}

	if verify {
		if verifyCredentialsFn(providerID) {
			return true, nil
		}
		if !quiet {
			out("✗ Detected credentials are expired or invalid, re-authenticating...\n")
		}
		return false, nil
	}

	return true, nil
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
	if !quiet {
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
		if err := config.WriteCredential(flow.ProviderID, flow.CredType, credData); err != nil {
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
		return enableProvider(providerID)
	}

	if quiet {
		return fmt.Errorf("no auth flow for %s; set credentials with --token or an environment variable", providerID)
	}

	bold := lipgloss.NewStyle().Bold(true)
	out("%s\n\n", bold.Render(provider.DisplayName(providerID)))

	value, err := prompt.Default.Input(prompt.InputConfig{
		Title:       fmt.Sprintf("%s credential", provider.DisplayName(providerID)),
		Placeholder: "paste credential here",
		Validate:    provider.ValidateNotEmpty,
	})
	if err != nil {
		return err
	}

	credData, _ := json.Marshal(map[string]string{"api_key": value})
	if err := config.WriteCredential(providerID, "apikey", credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}

	if err := enableProvider(providerID); err != nil {
		return err
	}
	if !quiet {
		out("✓ %s credential saved\n", provider.DisplayName(providerID))
	}
	return nil
}

func enableProvider(providerID string) error {
	if err := config.SetProviderEnabled(providerID, true); err != nil {
		return fmt.Errorf("enabling %s: %w", providerID, err)
	}
	return nil
}

// authDeleteProvider removes credentials and disables a provider.
func authDeleteProvider(providerID string) error {
	if !quiet {
		ok, err := prompt.Default.Confirm(prompt.ConfirmConfig{
			Title: fmt.Sprintf("Remove %s from vibeusage?", provider.DisplayName(providerID)),
		})
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	if err := config.SetProviderEnabled(providerID, false); err != nil {
		return fmt.Errorf("disabling %s: %w", providerID, err)
	}
	if _, err := config.DeleteProviderCredentials(providerID); err != nil {
		return fmt.Errorf("removing stored credentials for %s: %w", providerID, err)
	}
	config.ClearProviderCache(providerID)

	if !quiet {
		out("✓ Removed %s from vibeusage\n", provider.DisplayName(providerID))
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

	if auth, ok := p.(provider.Authenticator); ok {
		switch f := auth.Auth().(type) {
		case provider.ManualKeyAuthFlow:
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
				if err := config.WriteCredential(f.ProviderID, f.CredType, credData); err != nil {
					return fmt.Errorf("error saving credential: %w", err)
				}
			}
			if err := enableProvider(providerID); err != nil {
				return err
			}
			if !quiet {
				out("✓ %s credential saved\n", provider.DisplayName(providerID))
			}
			return nil
		default:
			// The provider's primary flow doesn't accept a manually pasted
			// token (e.g. device flow, browser redirect, or a CLI-owned
			// chain). Some such providers still expose a secondary stored
			// credential path — they opt in via TokenAcceptor. Without that
			// opt-in we refuse rather than write a credential fetch will
			// silently ignore.
			if acceptor, ok := p.(provider.TokenAcceptor); ok {
				if err := acceptor.AcceptToken(token); err != nil {
					return fmt.Errorf("error saving credential: %w", err)
				}
				if err := enableProvider(providerID); err != nil {
					return err
				}
				if !quiet {
					out("✓ %s credential saved\n", provider.DisplayName(providerID))
				}
				return nil
			}
			return fmt.Errorf("%s does not support --token; run `vibeusage auth %s` for the supported flow", provider.DisplayName(providerID), providerID)
		}
	}

	// Provider doesn't implement Authenticator at all — generic apikey fallback.
	credData, _ := json.Marshal(map[string]string{"api_key": token})
	if err := config.WriteCredential(providerID, "apikey", credData); err != nil {
		return fmt.Errorf("error saving credential: %w", err)
	}
	if err := enableProvider(providerID); err != nil {
		return err
	}
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
