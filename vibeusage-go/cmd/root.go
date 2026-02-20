package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"

	// Register all providers
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/claude"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/codex"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/copilot"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/cursor"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/gemini"
)

const version = "0.1.0"

var (
	jsonOutput bool
	noColor    bool
	verbose    bool
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:   "vibeusage",
	Short: "Track usage across agentic LLM providers",
	Long:  "A unified CLI tool that aggregates usage statistics from Claude, Codex, Copilot, Cursor, and Gemini.",
	RunE:  runDefaultUsage,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output")
	rootCmd.Flags().Bool("version", false, "Show version and exit")

	rootCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(initCmd)

	// Provider alias commands
	for _, id := range []string{"claude", "codex", "copilot", "cursor", "gemini"} {
		rootCmd.AddCommand(makeProviderCmd(id))
	}
}

func Execute() error {
	return rootCmd.Execute()
}

func runDefaultUsage(cmd *cobra.Command, args []string) error {
	if v, _ := cmd.Flags().GetBool("version"); v {
		fmt.Printf("vibeusage %s\n", version)
		return nil
	}

	// Resolve conflicts
	if verbose && quiet {
		verbose = false
	}

	// First-run check
	if config.IsFirstRun() && !jsonOutput && !quiet {
		showFirstRunMessage()
		return nil
	}

	return fetchAndDisplayAll(false)
}

func fetchAndDisplayAll(refresh bool) error {
	ctx := context.Background()
	start := time.Now()

	providerMap := buildProviderMap()
	outcomes := fetch.FetchEnabledProviders(ctx, providerMap, nil)

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		display.OutputMultiProviderJSON(outcomes)
		return nil
	}

	displayMultipleSnapshots(outcomes, durationMs)
	return nil
}

func buildProviderMap() map[string][]fetch.Strategy {
	pm := make(map[string][]fetch.Strategy)
	for id, p := range provider.All() {
		pm[id] = p.FetchStrategies()
	}
	return pm
}

func displayMultipleSnapshots(outcomes map[string]fetch.FetchOutcome, durationMs int64) {
	hasData := false
	for _, o := range outcomes {
		if o.Success && o.Snapshot != nil {
			hasData = true
			break
		}
	}

	if !hasData {
		if !quiet {
			fmt.Println("No usage data available")
			fmt.Println("\nConfigure credentials with:")
			fmt.Println("  vibeusage key <provider> set")
		}
		return
	}

	cfg := config.Get()
	staleThreshold := cfg.Fetch.StaleThresholdMinutes

	// Sort providers for consistent output
	ids := make([]string, 0, len(outcomes))
	for id := range outcomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var errors []struct{ id, err string }

	for _, pid := range ids {
		outcome := outcomes[pid]
		if !outcome.Success || outcome.Snapshot == nil {
			if outcome.Error != "" {
				errors = append(errors, struct{ id, err string }{pid, outcome.Error})
			}
			continue
		}

		snap := *outcome.Snapshot

		// Stale warning
		if outcome.Cached && !quiet {
			if w := display.RenderStaleWarning(snap, staleThreshold); w != "" {
				fmt.Println(w)
				fmt.Println()
			}
		}

		if quiet {
			for _, p := range snap.Periods {
				fmt.Printf("%s %s: %d%%\n", pid, p.Name, p.Utilization)
			}
		} else {
			fmt.Println(display.RenderProviderPanel(snap, outcome.Cached))
		}
	}

	if verbose && !quiet {
		if durationMs > 0 {
			fmt.Printf("\nTotal fetch time: %dms\n", durationMs)
		}
		if len(errors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range errors {
				fmt.Printf("  %s: %s\n", e.id, e.err)
			}
		}
	}
}

func makeProviderCmd(providerID string) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:   providerID,
		Short: "Show usage for " + strings.Title(providerID),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndDisplayProvider(providerID, refresh)
		},
	}
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Bypass cache and fetch fresh data")
	return cmd
}

func fetchAndDisplayProvider(providerID string, refresh bool) error {
	p, ok := provider.Get(providerID)
	if !ok {
		return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
	}

	ctx := context.Background()
	start := time.Now()

	strategies := p.FetchStrategies()
	outcome := fetch.ExecutePipeline(ctx, providerID, strategies, !refresh)

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		display.OutputJSON(display.SnapshotToJSON(outcome))
		return nil
	}

	if !outcome.Success || outcome.Snapshot == nil {
		if !quiet {
			fmt.Printf("Error: %s\n", outcome.Error)
		}
		os.Exit(1)
	}

	snap := *outcome.Snapshot

	if quiet {
		for _, p := range snap.Periods {
			fmt.Printf("%s %s: %d%%\n", providerID, p.Name, p.Utilization)
		}
		return nil
	}

	cfg := config.Get()
	if outcome.Cached {
		if w := display.RenderStaleWarning(snap, cfg.Fetch.StaleThresholdMinutes); w != "" {
			fmt.Println(w)
			fmt.Println()
		}
	}

	if verbose {
		if durationMs > 0 {
			fmt.Printf("Fetched in %dms\n", durationMs)
		}
		if snap.Identity != nil && snap.Identity.Email != "" {
			fmt.Printf("Account: %s\n", snap.Identity.Email)
		}
		if outcome.Source != "" {
			fmt.Printf("Source: %s\n", outcome.Source)
		}
		fmt.Println()
	}

	fmt.Print(display.RenderSingleProvider(snap, outcome.Cached, verbose))
	return nil
}

func showFirstRunMessage() {
	fmt.Println()
	fmt.Println("  ✨ Welcome to vibeusage!")
	fmt.Println()
	fmt.Println("  No providers are configured yet.")
	fmt.Println("  Track your usage across AI providers in one place.")
	fmt.Println()
	fmt.Println("  Quick start:")
	fmt.Println("    vibeusage init        - Run the setup wizard")
	fmt.Println("    vibeusage init --quick - Quick setup with Claude")
	fmt.Println()
	fmt.Println("  Or set up a provider directly:")

	ids := provider.ListIDs()
	sort.Strings(ids)
	count := 3
	if len(ids) < count {
		count = len(ids)
	}
	for _, id := range ids[:count] {
		fmt.Printf("    vibeusage auth %s\n", id)
	}
	fmt.Println()
}
