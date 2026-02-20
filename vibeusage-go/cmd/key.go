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
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage credentials for providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return displayAllCredentialStatus()
	},
}

func init() {
	for _, id := range []string{"claude", "codex", "copilot", "cursor", "gemini"} {
		keyCmd.AddCommand(makeKeyProviderCmd(id))
	}
}

func displayAllCredentialStatus() error {
	allStatus := config.GetAllCredentialStatus()

	if jsonOutput {
		data := make(map[string]any)
		for pid, info := range allStatus {
			hasCreds := info["has_credentials"].(bool)
			source := info["source"].(string)
			data[pid] = map[string]any{
				"configured": hasCreds,
				"source":     source,
			}
		}
		display.OutputJSON(data)
		return nil
	}

	if quiet {
		for pid, info := range allStatus {
			hasCreds := info["has_credentials"].(bool)
			status := "not configured"
			if hasCreds {
				status = "configured"
			}
			out("%s: %s\n", pid, status)
		}
		return nil
	}

	ids := make([]string, 0, len(allStatus))
	for id := range allStatus {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	outln("Credential Status")
	out("%-12s %-18s %s\n", "Provider", "Status", "Source")

	for _, pid := range ids {
		info := allStatus[pid]
		hasCreds := info["has_credentials"].(bool)
		source := info["source"].(string)

		if hasCreds {
			out("%-12s %-18s %s\n", pid, "✓ Configured", sourceToLabel(source))
		} else {
			out("%-12s %-18s %s\n", pid, "✗ Not configured", "—")
		}
	}

	outln("\nSet credentials with:")
	outln("  vibeusage key <provider> set")
	return nil
}

func makeKeyProviderCmd(providerID string) *cobra.Command {
	credType := "session"
	switch providerID {
	case "codex", "copilot", "gemini":
		credType = "oauth"
	}

	titleName := strings.ToUpper(providerID[:1]) + providerID[1:]

	provCmd := &cobra.Command{
		Use:   providerID,
		Short: fmt.Sprintf("Manage %s credentials", titleName),
		RunE: func(cmd *cobra.Command, args []string) error {
			found, source, path := config.FindProviderCredential(providerID)

			if jsonOutput {
				display.OutputJSON(map[string]any{
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

			credData, _ := json.Marshal(map[string]string{"credential": value})
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
