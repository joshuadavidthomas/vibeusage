package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/modelmap"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/routing"
	"github.com/joshuadavidthomas/vibeusage/internal/spinner"
	"github.com/joshuadavidthomas/vibeusage/internal/strutil"
)

var routeCmd = &cobra.Command{
	Use:   "route <model>",
	Short: "Pick the best provider for a model based on available headroom",
	Long: `Given a model name (e.g. "sonnet-4.5", "gpt-4o", "gemini"), fetches usage
from all configured providers that offer it and recommends the one with the
most remaining capacity.

Use "vibeusage route --list" to see all known models and their providers.
Use "vibeusage route --list <provider>" to see models for a specific provider.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		listFlag, _ := cmd.Flags().GetBool("list")

		if listFlag {
			providerFilter := ""
			if len(args) > 0 {
				providerFilter = args[0]
			}
			return listModels(providerFilter)
		}

		if len(args) == 0 {
			return fmt.Errorf("model name required. Use 'vibeusage route --list' to see available models")
		}

		return routeModel(cmd, args[0])
	},
}

func init() {
	routeCmd.Flags().BoolP("list", "l", false, "List known models and their providers")
}

func listModels(providerFilter string) error {
	var allModels []modelmap.ModelInfo

	if providerFilter != "" {
		allModels = modelmap.ListModelsForProvider(providerFilter)
		if len(allModels) == 0 {
			return fmt.Errorf("no models found for provider %q", providerFilter)
		}
	} else {
		allModels = modelmap.ListModels()
	}

	// Filter to models that have at least one configured provider,
	// and only show configured providers in the list.
	var filtered []modelmap.ModelInfo
	for _, m := range allModels {
		configured := configuredProviders(m.Providers)
		if len(configured) > 0 {
			m.Providers = configured
			filtered = append(filtered, m)
		}
	}
	allModels = filtered

	if len(allModels) == 0 {
		if providerFilter != "" {
			return fmt.Errorf("no models found for configured provider %q", providerFilter)
		}
		return fmt.Errorf("no models available — no providers are configured.\nSet up a provider with: vibeusage auth <provider>")
	}

	if jsonOutput {
		type jsonModel struct {
			ID        string   `json:"id"`
			Name      string   `json:"name"`
			Providers []string `json:"providers"`
		}
		var data []jsonModel
		for _, m := range allModels {
			data = append(data, jsonModel{
				ID:        strings.ToLower(m.ID),
				Name:      m.Name,
				Providers: m.Providers,
			})
		}
		display.OutputJSON(outWriter, data)
		return nil
	}

	if quiet {
		for _, m := range allModels {
			out("%s %s\n", strings.ToLower(m.ID), strings.Join(m.Providers, ","))
		}
		return nil
	}

	var rows [][]string
	for _, m := range allModels {
		rows = append(rows, []string{strings.ToLower(m.ID), strings.Join(m.Providers, ", ")})
	}

	title := "Known Models"
	if providerFilter != "" {
		title = fmt.Sprintf("Models for %s", strutil.TitleCase(providerFilter))
	}

	outln(display.NewTableWithOptions(
		[]string{"Model", "Providers"},
		rows,
		display.TableOptions{Title: title, NoColor: noColor, Width: display.TerminalWidth()},
	))

	return nil
}

// configuredProviders filters a list of provider IDs to only those that are
// registered and have at least one available fetch strategy.
func configuredProviders(providerIDs []string) []string {
	var result []string
	for _, pid := range providerIDs {
		p, ok := provider.Get(pid)
		if !ok {
			continue
		}
		for _, s := range p.FetchStrategies() {
			if s.IsAvailable() {
				result = append(result, pid)
				break
			}
		}
	}
	return result
}

func routeModel(cmd *cobra.Command, query string) error {
	info := modelmap.Lookup(query)
	if info == nil {
		// Try search for suggestions.
		suggestions := modelmap.Search(query)
		if len(suggestions) > 0 {
			msg := fmt.Sprintf("unknown model %q. Did you mean:", query)
			for _, s := range suggestions {
				if len(msg) > 200 {
					break
				}
				msg += fmt.Sprintf("\n  %s (%s)", s.ID, s.Name)
			}
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("unknown model %q. Use 'vibeusage route --list' to see available models", query)
	}

	// Filter to only configured providers.
	var configuredIDs []string
	for _, pid := range info.Providers {
		p, ok := provider.Get(pid)
		if !ok {
			continue
		}
		for _, s := range p.FetchStrategies() {
			if s.IsAvailable() {
				configuredIDs = append(configuredIDs, pid)
				break
			}
		}
	}

	if len(configuredIDs) == 0 {
		return fmt.Errorf(
			"%s is available from %s, but none are configured.\nSet up a provider with: vibeusage auth <provider>",
			info.Name, strings.Join(info.Providers, ", "),
		)
	}

	// Build strategy map for only the relevant providers.
	strategyMap := make(map[string][]fetch.Strategy)
	for _, pid := range configuredIDs {
		p, _ := provider.Get(pid)
		strategyMap[pid] = p.FetchStrategies()
	}

	// Fetch usage from all relevant providers.
	ctx := cmd.Context()
	var outcomes map[string]fetch.FetchOutcome

	if spinner.ShouldShow(quiet, jsonOutput, !isTerminal()) {
		err := spinner.Run(configuredIDs, func(onComplete func(spinner.CompletionInfo)) {
			outcomes = fetch.FetchAllProviders(ctx, strategyMap, !refresh, func(o fetch.FetchOutcome) {
				onComplete(outcomeToCompletion(o))
			})
		})
		if err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}
	} else {
		outcomes = fetch.FetchAllProviders(ctx, strategyMap, !refresh, nil)
	}

	// Build provider data for ranking.
	providerData := make(map[string]routing.ProviderData)
	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			providerData[pid] = routing.ProviderData{
				Snapshot: outcome.Snapshot,
				Cached:   outcome.Cached,
			}
		}
	}

	// Build multiplier map for providers that have cost multipliers.
	multipliers := make(map[string]*float64)
	for _, pid := range configuredIDs {
		if pid == "copilot" {
			multipliers[pid] = modelmap.LookupMultiplier(info.Name)
		}
	}

	candidates, unavailable := routing.Rank(configuredIDs, providerData, multipliers)

	rec := routing.Recommendation{
		ModelID:     info.ID,
		ModelName:   info.Name,
		Candidates:  candidates,
		Unavailable: unavailable,
	}
	if len(candidates) > 0 {
		rec.Best = &candidates[0]
	}

	if jsonOutput {
		display.OutputJSON(outWriter, rec)
		return nil
	}

	if quiet {
		if rec.Best != nil {
			out("%s\n", rec.Best.ProviderID)
		}
		return nil
	}

	return displayRecommendation(rec)
}

func displayRecommendation(rec routing.Recommendation) error {
	if rec.Best == nil {
		out("No usage data available for %s from any configured provider.\n", rec.ModelName)
		if len(rec.Unavailable) > 0 {
			out("Failed to fetch: %s\n", strings.Join(rec.Unavailable, ", "))
		}
		return nil
	}

	out("Route: %s\n\n", rec.ModelName)

	// Check if any candidate has a multiplier to decide whether to show the column.
	hasMultiplier := false
	for _, c := range rec.Candidates {
		if c.Multiplier != nil {
			hasMultiplier = true
			break
		}
	}

	// Ranked table.
	var rows [][]string
	var rowStyles []display.RowStyle

	for i, c := range rec.Candidates {
		name := strutil.TitleCase(c.ProviderID)

		bar := display.RenderBar(c.Utilization, 15, models.PaceToColor(nil, c.Utilization))
		util := fmt.Sprintf("%d%%", c.Utilization)

		headroom := fmt.Sprintf("%d%%", c.EffectiveHeadroom)

		cost := "—"
		if c.Multiplier != nil {
			m := *c.Multiplier
			if m == 0 {
				cost = "free"
			} else if m == float64(int(m)) {
				cost = fmt.Sprintf("%dx", int(m))
			} else {
				cost = fmt.Sprintf("%.2gx", m)
			}
		}

		reset := ""
		if d := timeUntilReset(c.ResetsAt); d != nil {
			reset = models.FormatResetCountdown(d)
		}

		plan := c.Plan
		if plan == "" {
			plan = "—"
		}

		if hasMultiplier {
			rows = append(rows, []string{name, bar + " " + util, headroom, cost, string(c.PeriodType), reset, plan})
		} else {
			rows = append(rows, []string{name, bar + " " + util, headroom, string(c.PeriodType), reset, plan})
		}

		if i == 0 {
			rowStyles = append(rowStyles, display.RowBold)
		} else {
			rowStyles = append(rowStyles, display.RowNormal)
		}
	}

	// Unavailable providers as dim rows at the bottom.
	sort.Strings(rec.Unavailable)
	for _, pid := range rec.Unavailable {
		name := strutil.TitleCase(pid)
		if hasMultiplier {
			rows = append(rows, []string{name, "—", "—", "—", "—", "—", "—"})
		} else {
			rows = append(rows, []string{name, "—", "—", "—", "—", "—"})
		}
		rowStyles = append(rowStyles, display.RowDim)
	}

	var headers []string
	if hasMultiplier {
		headers = []string{"Provider", "Usage", "Headroom", "Cost", "Period", "Resets In", "Plan"}
	} else {
		headers = []string{"Provider", "Usage", "Headroom", "Period", "Resets In", "Plan"}
	}

	outln(display.NewTableWithOptions(
		headers,
		rows,
		display.TableOptions{NoColor: noColor, Width: display.TerminalWidth(), RowStyles: rowStyles},
	))

	return nil
}

func timeUntilReset(t *time.Time) *time.Duration {
	if t == nil {
		return nil
	}
	d := time.Until(*t)
	if d < 0 {
		d = 0
	}
	return &d
}
