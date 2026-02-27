package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Show condensed usage for status widgets",
	Long: `Display condensed usage statistics suitable for status bars, widgets,
and terminal multiplexers like tmux, i3blocks, sketchybar, or waybar.

Output modes:
  (default)  Visual bars with utilization
  --short    Compact text format
  --json     Machine-readable JSON for scripts

Examples:
  vibeusage statusline                    # Pretty format, all providers
  vibeusage statusline --short            # Short format
  vibeusage statusline -p claude          # Only show Claude
  vibeusage statusline -p claude -p codex # Show multiple providers
  vibeusage statusline --json             # JSON output for scripting`,
	RunE: runStatusline,
}

var (
	statuslineShort     bool
	statuslineLimit     int
	statuslineProviders []string
)

func init() {
	statuslineCmd.Flags().BoolVarP(&statuslineShort, "short", "s", false, "Compact text format")
	statuslineCmd.Flags().IntVarP(&statuslineLimit, "limit", "n", 0, "Max periods to show per provider (0 = all)")
	statuslineCmd.Flags().StringArrayVarP(&statuslineProviders, "provider", "p", nil, "Provider to show (repeatable). Defaults to all enabled providers.")
}

func runStatusline(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := logging.FromContext(ctx)

	providerMap := buildProviderMap()
	cfg := config.Get()

	var providersToFetch []string
	if len(statuslineProviders) > 0 {
		for _, pid := range statuslineProviders {
			if _, ok := provider.Get(pid); !ok {
				return fmt.Errorf("unknown provider: %s", pid)
			}
			providersToFetch = append(providersToFetch, pid)
		}
	} else {
		for pid := range providerMap {
			if cfg.IsProviderEnabled(pid) {
				providersToFetch = append(providersToFetch, pid)
			}
		}
		sort.Strings(providersToFetch)
	}

	if len(providersToFetch) == 0 {
		return fmt.Errorf("no providers configured. Run 'vibeusage auth <provider>' to set up a provider")
	}

	filteredMap := make(map[string][]fetch.Strategy)
	for _, pid := range providersToFetch {
		filteredMap[pid] = providerMap[pid]
	}

	orchCfg := orchestratorConfigFromConfig(cfg)

	start := time.Now()
	outcomes := fetch.FetchAllProviders(ctx, filteredMap, !noCache, orchCfg, nil)
	durationMs := time.Since(start).Milliseconds()
	logger.Debug("statusline fetch complete", "duration_ms", durationMs, "providers", len(providersToFetch))

	var mode display.StatuslineMode
	switch {
	case jsonOutput:
		mode = display.StatuslineModeJSON
	case statuslineShort:
		mode = display.StatuslineModeShort
	default:
		mode = display.StatuslineModePretty
	}

	return display.RenderStatusline(outWriter, outcomes, display.StatuslineOptions{
		Mode:    mode,
		Limit:   statuslineLimit,
		NoColor: noColor,
	})
}
