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
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage credentials for providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return displayAllCredentialStatus()
	},
}

func init() {
	for _, id := range []string{"antigravity", "claude", "codex", "copilot", "cursor", "gemini", "kimi", "zai"} {
		keyCmd.AddCommand(makeKeyProviderCmd(id))
	}
}

func displayAllCredentialStatus() error {
	allStatus := config.GetAllCredentialStatus()

	if jsonOutput {
		data := make(map[string]any)
		for pid, info := range allStatus {
			data[pid] = map[string]any{
				"configured": info.HasCredentials,
				"source":     info.Source,
			}
		}
		display.OutputJSON(outWriter, data)
		return nil
	}

	ids := make([]string, 0, len(allStatus))
	for id := range allStatus {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if quiet {
		for _, pid := range ids {
			info := allStatus[pid]
			status := "not configured"
			if info.HasCredentials {
				status = "configured"
			}
			out("%s: %s\n", pid, status)
		}
		return nil
	}

	var rows [][]string
	for _, pid := range ids {
		info := allStatus[pid]

		status := "✗ Not configured"
		srcLabel := "—"
		if info.HasCredentials {
			status = "✓ Configured"
			srcLabel = sourceToLabel(info.Source)
		}
		rows = append(rows, []string{pid, status, srcLabel})
	}

	outln(display.NewTableWithOptions(
		[]string{"Provider", "Status", "Source"},
		rows,
		display.TableOptions{Title: "Credential Status", NoColor: noColor},
	))

	outln()
	outln("Set credentials with:")
	outln("  vibeusage key <provider> set")
	return nil
}

// credentialKey returns the JSON field name used when storing a credential
// for a provider. This must match what the provider's loadCredentials reads.
var credentialKeyMap = map[string]string{
	"antigravity": "access_token",
	"claude":      "session_key",
	"codex":       "access_token",
	"copilot":     "access_token",
	"cursor":      "session_token",
	"gemini":      "access_token",
	"kimi":        "api_key",
	"zai":         "api_key",
}

func makeKeyProviderCmd(providerID string) *cobra.Command {
	credType := "session"
	switch providerID {
	case "antigravity", "codex", "copilot", "gemini":
		credType = "oauth"
	case "kimi", "zai":
		credType = "apikey"
	}

	titleName := strutil.TitleCase(providerID)

	provCmd := &cobra.Command{
		Use:   providerID,
		Short: fmt.Sprintf("Manage %s credentials", titleName),
		RunE: func(cmd *cobra.Command, args []string) error {
			found, source, path := config.FindProviderCredential(providerID)

			if jsonOutput {
				display.OutputJSON(outWriter, map[string]any{
					"provider":   providerID,
					"configured": found,
					"source":     source,
					"path":       path,
				})
				return nil
			}

			if found {
				out("✓ %s credentials configured (%s)\n", titleName, sourceToLabel(source))
				if path != "" {
					out("  Location: %s\n", path)
				}
			} else {
				out("✗ %s credentials not configured\n", titleName)
				out("\nRun 'vibeusage key %s set' to configure\n", providerID)
			}
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:   "set [value]",
		Short: "Set a credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			var value string
			if len(args) > 0 {
				value = args[0]
			} else {
				var err error
				value, err = prompt.Default.Input(prompt.InputConfig{
					Title:       fmt.Sprintf("%s %s credential", titleName, credType),
					Placeholder: "paste credential here",
					Validate:    prompt.ValidateNotEmpty,
				})
				if err != nil {
					return err
				}
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("credential cannot be empty")
			}

			jsonKey := credentialKeyMap[providerID]
			credData, _ := json.Marshal(map[string]string{jsonKey: value})
			path := config.CredentialPath(providerID, credType)
			if err := config.WriteCredential(path, credData); err != nil {
				return fmt.Errorf("error saving credential: %w", err)
			}

			config.ClearProviderCache(providerID)
			out("✓ Credential saved for %s\n", providerID)
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				ok, err := prompt.Default.Confirm(prompt.ConfirmConfig{
					Title: fmt.Sprintf("Delete %s %s credential?", titleName, credType),
				})
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}

			path := config.CredentialPath(providerID, credType)
			if config.DeleteCredential(path) {
				out("✓ Deleted %s credential for %s\n", credType, providerID)
			} else {
				out("No %s credential found for %s\n", credType, providerID)
			}
			return nil
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	provCmd.AddCommand(setCmd)
	provCmd.AddCommand(deleteCmd)
	return provCmd
}
