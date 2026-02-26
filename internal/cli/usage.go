package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage for AI providers",
	Long:  "Display usage statistics for configured AI providers, or fetch data for a specific provider.",
	RunE:  runDefaultUsage,
}

func init() {
	ids := provider.ListIDs()
	sort.Strings(ids)
	for _, id := range ids {
		usageCmd.AddCommand(makeProviderCmd(id))
	}
}
