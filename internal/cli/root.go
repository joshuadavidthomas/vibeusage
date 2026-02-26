package cli

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
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	// Register all providers
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/amp"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/antigravity"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/claude"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/codex"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/copilot"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/cursor"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/gemini"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/kimicode"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/minimax"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/openrouter"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/warp"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/zai"
)

// version is injected at build time via -ldflags.
var version = "dev"

var (
	jsonOutput bool
	noColor    bool
	verbose    bool
	quiet      bool
	refresh    bool
)

var rootCmd = &cobra.Command{
	Use:          "vibeusage",
	Short:        "Track usage across agentic LLM providers",
	Long:         "A unified CLI tool that aggregates usage statistics from Amp, Antigravity, Claude, Codex, Copilot, Cursor, Gemini, Kimi Code, Minimax, OpenRouter, Warp, and Z.ai.",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose && quiet {
			verbose = false
		}
		l := newConfiguredLogger()
		ctx := logging.WithLogger(cmd.Context(), l)
		cmd.SetContext(ctx)

		// Load config from disk so malformed files surface a warning.
		if _, err := config.Init(); err != nil {
			l.Warn("config file is malformed, using defaults", "err", err)
		}
	},
	RunE: runDefaultUsage,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output")
	rootCmd.PersistentFlags().BoolVarP(&refresh, "refresh", "r", false, "Disable cache fallback — fresh data or error")
	rootCmd.Flags().Bool("version", false, "Show version and exit")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(cacheCmd)

	rootCmd.AddCommand(routeCmd)
	rootCmd.AddCommand(updateCmd)

	ids := provider.ListIDs()
	sort.Strings(ids)
	for _, id := range ids {
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

	if len(config.ReadEnabledProviders()) == 0 && !jsonOutput && !quiet {
		showFirstRunMessage()
		return nil
	}

	return fetchAndDisplayAll(cmd.Context())
}

func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

func fetchAndDisplayAll(ctx context.Context) error {
	start := time.Now()

	providerMap := buildProviderMap()
	cfg := config.Get()
	orchCfg := orchestratorConfigFromConfig(cfg)

	var outcomes map[string]fetch.FetchOutcome

	if display.SpinnerShouldShow(quiet, jsonOutput, !isTerminal()) {
		spinnerIDs := provider.AvailableIDs(cfg)
		err := display.SpinnerRun(spinnerIDs, func(onComplete func(display.CompletionInfo)) {
			outcomes = fetch.FetchEnabledProviders(ctx, providerMap, !refresh, orchCfg, cfg.IsProviderEnabled, func(o fetch.FetchOutcome) {
				onComplete(outcomeToCompletion(o))
			})
		})
		if err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}
	} else {
		outcomes = fetch.FetchEnabledProviders(ctx, providerMap, !refresh, orchCfg, cfg.IsProviderEnabled, nil)
	}

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		return display.OutputMultiProviderJSON(outWriter, outcomes)
	}

	displayMultipleSnapshots(ctx, outcomes, durationMs)
	return nil
}

func outcomeToCompletion(o fetch.FetchOutcome) display.CompletionInfo {
	return display.CompletionInfo{
		ProviderID: o.ProviderID,
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

func displayMultipleSnapshots(ctx context.Context, outcomes map[string]fetch.FetchOutcome, durationMs int64) {
	logger := logging.FromContext(ctx)

	ids := make([]string, 0, len(outcomes))
	for id := range outcomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	type providerError struct{ id, err string }
	var errors []providerError
	for _, pid := range ids {
		outcome := outcomes[pid]
		if !outcome.Success || outcome.Snapshot == nil {
			// Skip unconfigured providers — only show real fetch errors.
			if outcome.Error != "" && outcome.Error != "No strategies available" {
				errors = append(errors, providerError{pid, outcome.Error})
			}
		}
	}

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
			if len(errors) > 0 {
				outln()
				for _, e := range errors {
					outln(display.RenderProviderError(e.id, e.err))
				}
			} else {
				outln("\nSet up a provider with:")
				outln("  vibeusage auth <provider>")
			}
		}
		for _, e := range errors {
			logger.Debug("provider error", "provider", e.id, "error", e.err)
		}
		return
	}

	// Collect all successful snapshots upfront so we can compute globally
	// consistent column widths before rendering any individual panel.
	var snapshots []models.UsageSnapshot
	for _, pid := range ids {
		if o := outcomes[pid]; o.Success && o.Snapshot != nil {
			snapshots = append(snapshots, *o.Snapshot)
		}
	}
	colWidths := display.GlobalPeriodColWidths(snapshots)

	for _, pid := range ids {
		outcome := outcomes[pid]
		if !outcome.Success || outcome.Snapshot == nil {
			continue
		}

		snap := *outcome.Snapshot

		if quiet {
			for _, p := range snap.Periods {
				out("%s %s: %d%%\n", pid, p.Name, p.Utilization)
			}
		} else {
			outln(display.RenderProviderPanel(snap, outcome.Cached, colWidths))
		}
	}

	if !quiet && len(errors) > 0 {
		for _, e := range errors {
			outln(display.RenderProviderError(e.id, e.err))
		}
	}

	if durationMs > 0 {
		logger.Debug("fetch complete", "total_duration_ms", durationMs)
	}
	for _, e := range errors {
		logger.Debug("provider error", "provider", e.id, "error", e.err)
	}
}

func makeProviderCmd(providerID string) *cobra.Command {
	titleName := provider.DisplayName(providerID)
	return &cobra.Command{
		Use:   providerID,
		Short: "Show usage for " + titleName,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndDisplayProvider(cmd.Context(), providerID)
		},
	}
}

func fetchAndDisplayProvider(ctx context.Context, providerID string) error {
	logger := logging.FromContext(ctx)

	p, ok := provider.Get(providerID)
	if !ok {
		return fmt.Errorf("unknown provider: %s. Available: %s", providerID, strings.Join(provider.ListIDs(), ", "))
	}

	start := time.Now()

	strategies := p.FetchStrategies()
	pipeCfg := pipelineConfigFromConfig(config.Get())

	var outcome fetch.FetchOutcome

	if display.SpinnerShouldShow(quiet, jsonOutput, !isTerminal()) {
		err := display.SpinnerRun([]string{providerID}, func(onComplete func(display.CompletionInfo)) {
			outcome = fetch.ExecutePipeline(ctx, providerID, strategies, !refresh, pipeCfg)
			onComplete(outcomeToCompletion(outcome))
		})
		if err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}
	} else {
		outcome = fetch.ExecutePipeline(ctx, providerID, strategies, !refresh, pipeCfg)
	}

	durationMs := time.Since(start).Milliseconds()

	if jsonOutput {
		return display.OutputJSON(outWriter, display.SnapshotToJSON(outcome))
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

	logFields := []any{"provider", providerID}
	if durationMs > 0 {
		logFields = append(logFields, "duration_ms", durationMs)
	}
	if snap.Identity != nil && snap.Identity.Email != "" {
		logFields = append(logFields, "account", snap.Identity.Email)
	}
	if outcome.Source != "" {
		logFields = append(logFields, "source", outcome.Source)
	}
	logger.Debug("fetch complete", logFields...)

	_, _ = fmt.Fprint(outWriter, display.RenderSingleProvider(snap, outcome.Cached))
	return nil
}

func pipelineConfigFromConfig(cfg config.Config) fetch.PipelineConfig {
	return fetch.PipelineConfig{
		Timeout:               time.Duration(cfg.Fetch.Timeout * float64(time.Second)),
		StaleThresholdMinutes: cfg.Fetch.StaleThresholdMinutes,
		Cache:                 config.FileCache{},
	}
}

func orchestratorConfigFromConfig(cfg config.Config) fetch.OrchestratorConfig {
	return fetch.OrchestratorConfig{
		MaxConcurrent: cfg.Fetch.MaxConcurrent,
		Pipeline:      pipelineConfigFromConfig(cfg),
	}
}

func showFirstRunMessage() {
	outln()
	outln("Welcome to vibeusage!")
	outln("Track your usage across AI providers in one place.")
	outln()
	outln("Get started with:")
	outln("  vibeusage auth")
	outln()
}
