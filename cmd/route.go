package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
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
	Use:   "route [model]",
	Short: "Pick the best provider for a model or role based on available headroom",
	Long: `Given a model name (e.g. "sonnet-4.5", "gpt-4o", "gemini"), fetches usage
from all configured providers that offer it and recommends the one with the
most remaining capacity.

Use --role to route by a user-defined role instead of a specific model.
Roles are configured in config.toml under [roles.<name>] with a list of model IDs.

Use "vibeusage route --list" to see all known models and their providers.
Use "vibeusage route --list <provider>" to see models for a specific provider.
Use "vibeusage route --list-roles" to see configured roles and their models.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		listFlag, _ := cmd.Flags().GetBool("list")
		listRolesFlag, _ := cmd.Flags().GetBool("list-roles")
		roleFlag, _ := cmd.Flags().GetString("role")

		if listRolesFlag {
			return listRoles()
		}

		if listFlag {
			providerFilter := ""
			if len(args) > 0 {
				providerFilter = args[0]
			}
			return listModels(providerFilter)
		}

		if roleFlag != "" {
			if len(args) > 0 {
				return fmt.Errorf("cannot use both --role and a model argument")
			}
			return routeByRole(cmd, roleFlag)
		}

		if len(args) == 0 {
			return fmt.Errorf("model name or --role required. Use 'vibeusage route --list' to see available models")
		}

		return routeModel(cmd, args[0])
	},
}

func init() {
	routeCmd.Flags().BoolP("list", "l", false, "List known models and their providers")
	routeCmd.Flags().Bool("list-roles", false, "List configured roles and their models")
	routeCmd.Flags().String("role", "", "Route by role instead of specific model")
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

	// Build a reverse map: model ID → role names.
	modelRoles := buildModelRolesMap()

	if jsonOutput {
		type jsonModel struct {
			ID        string   `json:"id"`
			Name      string   `json:"name"`
			Providers []string `json:"providers"`
			Roles     []string `json:"roles,omitempty"`
		}
		var data []jsonModel
		for _, m := range allModels {
			data = append(data, jsonModel{
				ID:        strings.ToLower(m.ID),
				Name:      m.Name,
				Providers: m.Providers,
				Roles:     modelRoles[m.ID],
			})
		}
		display.OutputJSON(outWriter, data)
		return nil
	}

	if quiet {
		for _, m := range allModels {
			roles := modelRoles[m.ID]
			if len(roles) > 0 {
				out("%s %s %s\n", strings.ToLower(m.ID), strings.Join(m.Providers, ","), strings.Join(roles, ","))
			} else {
				out("%s %s\n", strings.ToLower(m.ID), strings.Join(m.Providers, ","))
			}
		}
		return nil
	}

	var rows [][]string
	for _, m := range allModels {
		roles := strings.Join(modelRoles[m.ID], ", ")
		rows = append(rows, []string{strings.ToLower(m.ID), strings.Join(m.Providers, ", "), roles})
	}

	title := "Known Models"
	if providerFilter != "" {
		title = fmt.Sprintf("Models for %s", strutil.TitleCase(providerFilter))
	}

	outln(display.NewTableWithOptions(
		[]string{"Model", "Providers", "Roles"},
		rows,
		display.TableOptions{Title: title, NoColor: noColor, Width: display.TerminalWidth()},
	))

	return nil
}

// buildModelRolesMap returns a map from canonical model ID to sorted role names.
// Uses prefix matching so "claude-opus-4-5" in a role also tags "claude-opus-4-5-20251101".
func buildModelRolesMap() map[string][]string {
	cfg := config.Get()
	result := make(map[string][]string)

	for roleName, role := range cfg.Roles {
		for _, modelID := range role.Models {
			matches := modelmap.MatchPrefix(modelID)
			if len(matches) == 0 {
				// Fall back to exact lookup (might match an alias).
				if info := modelmap.Lookup(modelID); info != nil {
					matches = []modelmap.ModelInfo{*info}
				}
			}
			for _, info := range matches {
				result[info.ID] = append(result[info.ID], roleName)
			}
		}
	}

	// Deduplicate and sort role names per model.
	for id, roles := range result {
		sort.Strings(roles)
		deduped := roles[:0]
		for i, r := range roles {
			if i == 0 || r != roles[i-1] {
				deduped = append(deduped, r)
			}
		}
		result[id] = deduped
	}

	return result
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

func listRoles() error {
	cfg := config.Get()
	names := cfg.RoleNames()

	if len(names) == 0 {
		if jsonOutput {
			display.OutputJSON(outWriter, []struct{}{})
			return nil
		}
		return fmt.Errorf("no roles configured.\nAdd roles to your config:\n  vibeusage config edit\n\nExample:\n  [roles.thinking]\n  models = [\"claude-opus-4-6\", \"o4\", \"gpt-5-2\"]")
	}

	if jsonOutput {
		type jsonRole struct {
			Name   string   `json:"name"`
			Models []string `json:"models"`
		}
		var data []jsonRole
		for _, name := range names {
			role, _ := cfg.GetRole(name)
			data = append(data, jsonRole{Name: name, Models: role.Models})
		}
		display.OutputJSON(outWriter, data)
		return nil
	}

	if quiet {
		for _, name := range names {
			role, _ := cfg.GetRole(name)
			out("%s %s\n", name, strings.Join(role.Models, ","))
		}
		return nil
	}

	var rows [][]string
	for _, name := range names {
		role, _ := cfg.GetRole(name)
		rows = append(rows, []string{name, strings.Join(role.Models, ", ")})
	}

	outln(display.NewTableWithOptions(
		[]string{"Role", "Models"},
		rows,
		display.TableOptions{Title: "Configured Roles", NoColor: noColor, Width: display.TerminalWidth()},
	))

	return nil
}

func routeByRole(cmd *cobra.Command, roleName string) error {
	cfg := config.Get()
	role, ok := cfg.GetRole(roleName)
	if !ok {
		names := cfg.RoleNames()
		if len(names) > 0 {
			return fmt.Errorf("unknown role %q. Available roles: %s", roleName, strings.Join(names, ", "))
		}
		return fmt.Errorf("unknown role %q. No roles configured.\nAdd roles to your config:\n  vibeusage config edit\n\nExample:\n  [roles.%s]\n  models = [\"claude-sonnet-4-6\", \"gpt-5\"]", roleName, roleName)
	}

	if len(role.Models) == 0 {
		return fmt.Errorf("role %q has no models configured", roleName)
	}

	// Resolve each model ID to its providers and build model entries.
	// Uses prefix matching so "claude-opus-4-5" also picks up dated variants,
	// but prefers the shortest (non-dated) ID per provider to avoid duplicates.
	var modelEntries []routing.RoleModelEntry
	allProviderIDs := make(map[string]bool)

	for _, modelID := range role.Models {
		matches := modelmap.MatchPrefix(modelID)
		if len(matches) == 0 {
			// Fall back to exact lookup (might match an alias).
			if info := modelmap.Lookup(modelID); info != nil {
				matches = []modelmap.ModelInfo{*info}
			}
		}
		if len(matches) == 0 {
			continue
		}

		// Prefer the shortest ID (the "latest" pointer, not the dated variant).
		// MatchPrefix returns sorted by length, so first match is shortest.
		best := matches[0]
		configured := configuredProviders(best.Providers)
		if len(configured) == 0 {
			continue
		}

		modelEntries = append(modelEntries, routing.RoleModelEntry{
			ModelID:     best.ID,
			ModelName:   best.Name,
			ProviderIDs: configured,
		})
		for _, pid := range configured {
			allProviderIDs[pid] = true
		}
	}

	if len(modelEntries) == 0 {
		return fmt.Errorf("no models in role %q are available from configured providers.\nModels in role: %s", roleName, strings.Join(role.Models, ", "))
	}

	// Build strategy map for all relevant providers.
	strategyMap := make(map[string][]fetch.Strategy)
	var providerList []string
	for pid := range allProviderIDs {
		p, _ := provider.Get(pid)
		strategyMap[pid] = p.FetchStrategies()
		providerList = append(providerList, pid)
	}
	sort.Strings(providerList)

	// Fetch usage from all relevant providers.
	ctx := cmd.Context()
	var outcomes map[string]fetch.FetchOutcome

	if spinner.ShouldShow(quiet, jsonOutput, !isTerminal()) {
		err := spinner.Run(providerList, func(onComplete func(spinner.CompletionInfo)) {
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

	// Multiplier lookup function.
	multiplierFn := func(modelName string, providerID string) *float64 {
		if providerID == "copilot" {
			return modelmap.LookupMultiplier(modelName)
		}
		return nil
	}

	candidates, unavailable := routing.RankByRole(modelEntries, providerData, multiplierFn)

	rec := routing.RoleRecommendation{
		Role:        roleName,
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
			out("%s %s\n", rec.Best.ModelID, rec.Best.ProviderID)
		}
		return nil
	}

	return displayRoleRecommendation(rec)
}

func displayRoleRecommendation(rec routing.RoleRecommendation) error {
	if rec.Best == nil {
		out("No usage data available for role %q from any configured provider.\n", rec.Role)
		if len(rec.Unavailable) > 0 {
			out("Failed to fetch:")
			for _, u := range rec.Unavailable {
				out(" %s(%s)", u.ModelID, u.ProviderID)
			}
			outln("")
		}
		return nil
	}

	out("Route: %s (role)\n\n", rec.Role)

	// Check if any candidate has a multiplier.
	hasMultiplier := false
	for _, c := range rec.Candidates {
		if c.Multiplier != nil {
			hasMultiplier = true
			break
		}
	}

	var rows [][]string
	var rowStyles []display.RowStyle

	for i, c := range rec.Candidates {
		model := c.ModelID
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
			rows = append(rows, []string{model, name, bar + " " + util, headroom, cost, string(c.PeriodType), reset, plan})
		} else {
			rows = append(rows, []string{model, name, bar + " " + util, headroom, string(c.PeriodType), reset, plan})
		}

		if i == 0 {
			rowStyles = append(rowStyles, display.RowBold)
		} else {
			rowStyles = append(rowStyles, display.RowNormal)
		}
	}

	// Unavailable as dim rows.
	for _, u := range rec.Unavailable {
		if hasMultiplier {
			rows = append(rows, []string{u.ModelID, strutil.TitleCase(u.ProviderID), "—", "—", "—", "—", "—", "—"})
		} else {
			rows = append(rows, []string{u.ModelID, strutil.TitleCase(u.ProviderID), "—", "—", "—", "—", "—"})
		}
		rowStyles = append(rowStyles, display.RowDim)
	}

	var headers []string
	if hasMultiplier {
		headers = []string{"Model", "Provider", "Usage", "Headroom", "Cost", "Period", "Resets In", "Plan"}
	} else {
		headers = []string{"Model", "Provider", "Usage", "Headroom", "Period", "Resets In", "Plan"}
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
