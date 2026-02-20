package cmd

import (
	"fmt"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage [provider]",
	Short: "Show usage statistics",
	Long:  "Show usage statistics for all enabled providers or a specific provider.",
	RunE: func(cmd *cobra.Command, args []string) error {
		refresh, _ := cmd.Flags().GetBool("refresh")

		if len(args) > 0 {
			providerID := args[0]
			if _, ok := provider.Get(providerID); !ok {
				return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
			}
			return fetchAndDisplayProvider(providerID, refresh)
		}

		return fetchAndDisplayAll(refresh)
	},
}

func init() {
	usageCmd.Flags().BoolP("refresh", "r", false, "Bypass cache and fetch fresh data")
}
