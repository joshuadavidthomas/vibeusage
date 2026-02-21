package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/spinner"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"

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
	Use:          "vibeusage",
	Short:        "Track usage across agentic LLM providers",
	Long:         "A unified CLI tool that aggregates usage statistics from Claude, Codex, Copilot, Cursor, and Gemini.",
	SilenceUsage: true,
	RunE:         runDefaultUsage,
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

	for _, id := range []string{"claude", "codex", "copilot", "cursor", "gemini"} {
		rootCmd.AddCommand(makeProviderCmd(id))
	}
}

func Execute() error {
	return rootCmd.Execute()
}

// ExecuteContext runs the root command with the given context.
// Commands access it via cmd.Context().
func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func runDefaultUsage(cmd *cobra.Command, args []string) error {
	if v, _ := cmd.Flags().GetBool("version"); v {
		out("vibeusage %s\n", version)
		return nil
	}

	if verbose && quiet {
		verbose = false
	}

	if config.IsFirstRun() && !jsonOutput && !quiet {
		showFirstRunMessage()
		return nil
	}

	return fetchAndDisplayAll(cmd.Context(), false)
}

func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

func fetchAndDisplayAll(ctx context.Context, refresh bool) error {
	start := time.Now()

	providerMap := buildProviderMap()

	useCache := !refresh
	var outcomes map[string]fetch.FetchOutcome

	if spinner.ShouldShow(quiet, jsonOutput, !isTerminal()) {
		providerIDs := enabledProviderIDs(providerMap)
		err := spinner.Run(providerIDs, func(onComplete func(spinner.CompletionInfo)) {
			outcomes = fetch.FetchEnabledProviders(ctx, providerMap, useCache, func(o fetch.FetchOutcome) {
				onComplete(outcomeToCompletion(o))
			})
		})
		if err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}
	} else {
		outcomes = fetch.FetchEnabledProviders(ctx, providerMap, useCache, nil)
	}

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		display.OutputMultiProviderJSON(outWriter, outcomes)
		return nil
	}

	displayMultipleSnapshots(outcomes, durationMs)
	return nil
}

func enabledProviderIDs(providerMap map[string][]fetch.Strategy) []string {
	cfg := config.Get()
	var ids []string
	for pid := range providerMap {
		if cfg.IsProviderEnabled(pid) {
			ids = append(ids, pid)
		}
	}
	sort.Strings(ids)
	return ids
}

func outcomeToCompletion(o fetch.FetchOutcome) spinner.CompletionInfo {
	durationMs := 0
	for _, a := range o.Attempts {
		durationMs += a.DurationMs
	}
	return spinner.CompletionInfo{
		ProviderID: o.ProviderID,
		Source:     o.Source,
		DurationMs: durationMs,
		Success:    o.Success,
		Error:      o.Error,
	}
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
			outln("No usage data available")
			outln("\nConfigure credentials with:")
			outln("  vibeusage key <provider> set")
		}
		return
	}

	cfg := config.Get()
	staleThreshold := cfg.Fetch.StaleThresholdMinutes

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

		if outcome.Cached && !quiet {
			if w := display.RenderStaleWarning(snap, staleThreshold); w != "" {
				outln(w)
				outln()
			}
		}

		if quiet {
			for _, p := range snap.Periods {
				out("%s %s: %d%%\n", pid, p.Name, p.Utilization)
			}
		} else {
			outln(display.RenderProviderPanel(snap))
		}
	}

	if verbose && !quiet {
		if durationMs > 0 {
			out("\nTotal fetch time: %dms\n", durationMs)
		}
		if len(errors) > 0 {
			outln("\nErrors:")
			for _, e := range errors {
				out("  %s: %s\n", e.id, e.err)
			}
		}
	}
}

func makeProviderCmd(providerID string) *cobra.Command {
	var refresh bool
	titleName := strutil.TitleCase(providerID)
	cmd := &cobra.Command{
		Use:   providerID,
		Short: "Show usage for " + titleName,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndDisplayProvider(cmd.Context(), providerID, refresh)
		},
	}
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Bypass cache and fetch fresh data")
	return cmd
}

func fetchAndDisplayProvider(ctx context.Context, providerID string, refresh bool) error {
	p, ok := provider.Get(providerID)
	if !ok {
		return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
	}

	start := time.Now()

	strategies := p.FetchStrategies()

	var outcome fetch.FetchOutcome

	if spinner.ShouldShow(quiet, jsonOutput, !isTerminal()) {
		err := spinner.Run([]string{providerID}, func(onComplete func(spinner.CompletionInfo)) {
			outcome = fetch.ExecutePipeline(ctx, providerID, strategies, !refresh)
			onComplete(outcomeToCompletion(outcome))
		})
		if err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}
	} else {
		outcome = fetch.ExecutePipeline(ctx, providerID, strategies, !refresh)
	}

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		display.OutputJSON(outWriter, display.SnapshotToJSON(outcome))
		return nil
	}

	if !outcome.Success || outcome.Snapshot == nil {
		errMsg := outcome.Error
		if errMsg == "" {
			errMsg = "fetch failed"
		}
		return fmt.Errorf("%s", errMsg)
	}

	snap := *outcome.Snapshot

	if quiet {
		for _, p := range snap.Periods {
			out("%s %s: %d%%\n", providerID, p.Name, p.Utilization)
		}
		return nil
	}

	cfg := config.Get()
	if outcome.Cached {
		if w := display.RenderStaleWarning(snap, cfg.Fetch.StaleThresholdMinutes); w != "" {
			outln(w)
			outln()
		}
	}

	if verbose {
		if durationMs > 0 {
			out("Fetched in %dms\n", durationMs)
		}
		if snap.Identity != nil && snap.Identity.Email != "" {
			out("Account: %s\n", snap.Identity.Email)
		}
		if outcome.Source != "" {
			out("Source: %s\n", outcome.Source)
		}
		outln()
	}

	_, _ = fmt.Fprint(outWriter, display.RenderSingleProvider(snap))
	return nil
}

func showFirstRunMessage() {
	outln()
	outln("  ✨ Welcome to vibeusage!")
	outln()
	outln("  No providers are configured yet.")
	outln("  Track your usage across AI providers in one place.")
	outln()
	outln("  Quick start:")
	outln("    vibeusage init        - Run the setup wizard")
	outln("    vibeusage init --quick - Quick setup with Claude")
	outln()
	outln("  Or set up a provider directly:")

	ids := provider.ListIDs()
	sort.Strings(ids)
	count := 3
	if len(ids) < count {
		count = len(ids)
	}
	for _, id := range ids[:count] {
		out("    vibeusage auth %s\n", id)
	}
	outln()
}
