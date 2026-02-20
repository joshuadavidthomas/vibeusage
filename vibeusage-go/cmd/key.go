package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage credentials for providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return displayAllCredentialStatus()
	},
}

func init() {
	// Register provider-specific key subcommands
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
			fmt.Printf("%s: %s\n", pid, status)
		}
		return nil
	}

	ids := make([]string, 0, len(allStatus))
	for id := range allStatus {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	fmt.Println("Credential Status")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("%-12s %-18s %s\n", "Provider", "Status", "Source")
	fmt.Println(strings.Repeat("─", 50))

	for _, pid := range ids {
		info := allStatus[pid]
		hasCreds := info["has_credentials"].(bool)
		source := info["source"].(string)

		if hasCreds {
			fmt.Printf("%-12s %-18s %s\n", pid, "✓ Configured", sourceToLabel(source))
		} else {
			fmt.Printf("%-12s %-18s %s\n", pid, "✗ Not configured", "—")
		}
	}

	fmt.Println("\nSet credentials with:")
	fmt.Println("  vibeusage key <provider> set")
	return nil
}

func makeKeyProviderCmd(providerID string) *cobra.Command {
	credType := "session"
	switch providerID {
	case "codex", "copilot", "gemini":
		credType = "oauth"
	}

	provCmd := &cobra.Command{
		Use:   providerID,
		Short: fmt.Sprintf("Manage %s credentials", strings.Title(providerID)),
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
				fmt.Printf("✓ %s credentials configured (%s)\n", strings.Title(providerID), sourceToLabel(source))
				if path != "" {
					fmt.Printf("  Location: %s\n", path)
				}
			} else {
				fmt.Printf("✗ %s credentials not configured\n", strings.Title(providerID))
				fmt.Printf("\nRun 'vibeusage key %s set' to configure\n", providerID)
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
				fmt.Printf("Enter %s %s credential: ", providerID, credType)
				fmt.Scanln(&value)
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
			fmt.Printf("✓ Credential saved for %s\n", providerID)
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				fmt.Printf("Delete %s %s credential? [y/N] ", strings.Title(providerID), credType)
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return nil
				}
			}

			path := config.CredentialPath(providerID, credType)
			if config.DeleteCredential(path) {
				fmt.Printf("✓ Deleted %s credential for %s\n", credType, providerID)
			} else {
				fmt.Printf("No %s credential found for %s\n", credType, providerID)
			}
			return nil
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	provCmd.AddCommand(setCmd)
	provCmd.AddCommand(deleteCmd)
	return provCmd
}
