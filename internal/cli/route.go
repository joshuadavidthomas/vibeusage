package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/catalog"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/routing"
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
		// Preload model registry data explicitly so any network fetch happens
		// here (with optional spinner) rather than silently on first Lookup.
		preloadModelData(cmd.Context())

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

// preloadModelData loads the modelmap registry and Copilot multipliers up
// front, at a known point in the route command lifecycle. When a network fetch
// is likely (cache stale or absent) and a spinner is appropriate, a brief
// spinner is shown so the user knows what is happening. Once data is cached on
// disk the call returns near-instantly with no visible output.
func preloadModelData(ctx context.Context) {
	if catalog.CacheIsFresh() {
		// Fast path: data is on disk and within TTL — load silently.
		catalog.Preload(ctx)
		return
	}

	if !display.SpinnerShouldShow(quiet, jsonOutput, !isTerminal()) {
		catalog.Preload(ctx)
		return
	}

	_ = display.SpinnerRun([]string{"models.dev"}, func(onComplete func(display.CompletionInfo)) {
		catalog.Preload(ctx)
		onComplete(display.CompletionInfo{ProviderID: "models.dev", Success: true})
	})
}

// fetchAllWithSpinner fetches usage from all providers in the strategy map,
// wrapping with a spinner for terminal output when appropriate.
func fetchAllWithSpinner(ctx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome {
	providerIDs := make([]string, 0, len(strategies))
	for pid := range strategies {
		providerIDs = append(providerIDs, pid)
	}

	orchCfg := orchestratorConfigFromConfig(config.Get())

	if display.SpinnerShouldShow(quiet, jsonOutput, !isTerminal()) {
		var outcomes map[string]fetch.FetchOutcome
		_ = display.SpinnerRun(providerIDs, func(onComplete func(display.CompletionInfo)) {
			outcomes = fetch.FetchAllProviders(ctx, strategies, useCache, orchCfg, func(o fetch.FetchOutcome) {
				onComplete(outcomeToCompletion(o))
			})
		})
		return outcomes
	}

	return fetch.FetchAllProviders(ctx, strategies, useCache, orchCfg, nil)
}

// lookupStrategies returns fetch strategies for a provider.
func lookupStrategies(id string) []fetch.Strategy {
	p, _ := provider.Get(id)
	return p.FetchStrategies()
}

// lookupMultiplier returns the cost multiplier for a model+provider pair.
func lookupMultiplier(modelName string, providerID string) *float64 {
	if providerID == "copilot" {
		return catalog.LookupMultiplier(modelName)
	}
	return nil
}

// routeModel resolves a model query, fetches usage from all configured
// providers that offer it, and returns a ranked recommendation.
func routeModel(cmd *cobra.Command, query string) error {
	rec, err := doRouteModel(cmd.Context(), query)
	if err != nil {
		return err
	}

	if jsonOutput {
		return display.OutputJSON(outWriter, rec)
	}

	if quiet {
		if rec.Best != nil {
			out("%s\n", rec.Best.ProviderID)
		}
		return nil
	}

	return displayRecommendation(rec)
}

// doRouteModel resolves a model query, fetches usage from all configured
// providers that offer it, and returns a ranked recommendation.
func doRouteModel(ctx context.Context, query string) (routing.Recommendation, error) {
	info := catalog.Lookup(query)
	if info == nil {
		suggestions := catalog.Search(query)
		if len(suggestions) > 0 {
			msg := fmt.Sprintf("unknown model %q. Did you mean:", query)
			for _, sug := range suggestions {
				if len(msg) > 200 {
					break
				}
				msg += fmt.Sprintf("\n  %s (%s)", sug.ID, sug.Name)
			}
			return routing.Recommendation{}, fmt.Errorf("%s", msg)
		}
		return routing.Recommendation{}, fmt.Errorf("unknown model %q. Use 'vibeusage route --list' to see available models", query)
	}

	configuredIDs := provider.ConfiguredIDs(info.Providers)
	if len(configuredIDs) == 0 {
		return routing.Recommendation{}, fmt.Errorf(
			"%s is available from %s, but none are configured.\nSet up a provider with: vibeusage auth <provider>",
			info.Name, strings.Join(info.Providers, ", "),
		)
	}

	strategyMap := routing.BuildStrategyMap(configuredIDs, lookupStrategies)
	outcomes := fetchAllWithSpinner(ctx, strategyMap, !refresh)
	providerData := routing.BuildProviderData(outcomes)

	multipliers := make(map[string]*float64)
	for _, pid := range configuredIDs {
		if mult := lookupMultiplier(info.Name, pid); mult != nil {
			multipliers[pid] = mult
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

	return rec, nil
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
	renderRouteTable(display.FormatRecommendationRows(rec, routeRenderBar, routeFormatReset))
	return nil
}

func listModels(providerFilter string) error {
	var allModels []catalog.ModelInfo

	if providerFilter != "" {
		allModels = catalog.ListModelsForProvider(providerFilter)
		if len(allModels) == 0 {
			return fmt.Errorf("no models found for provider %q", providerFilter)
		}
	} else {
		allModels = catalog.ListModels()
	}

	// Filter to models that have at least one configured provider,
	// and only show configured providers in the list.
	var filtered []catalog.ModelInfo
	for _, m := range allModels {
		configured := provider.ConfiguredIDs(m.Providers)
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
	cfg := config.Get()
	roles := make(map[string][]string)
	for name, role := range cfg.Roles {
		roles[name] = role.Models
	}
	modelRoles := buildModelRolesMap(roles)

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
		return display.OutputJSON(outWriter, data)
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
		roleStr := strings.Join(modelRoles[m.ID], ", ")
		rows = append(rows, []string{strings.ToLower(m.ID), strings.Join(m.Providers, ", "), roleStr})
	}

	title := "Known Models"
	if providerFilter != "" {
		title = fmt.Sprintf("Models for %s", provider.DisplayName(providerFilter))
	}

	outln(display.NewTableWithOptions(
		[]string{"Model", "Providers", "Roles"},
		rows,
		display.TableOptions{Title: title, NoColor: noColor, Width: display.TerminalWidth()},
	))

	return nil
}

func routeByRole(cmd *cobra.Command, roleName string) error {
	rec, err := doRouteByRole(cmd.Context(), roleName)
	if err != nil {
		return err
	}

	if jsonOutput {
		return display.OutputJSON(outWriter, rec)
	}

	if quiet {
		if rec.Best != nil {
			out("%s %s\n", rec.Best.ModelID, rec.Best.ProviderID)
		}
		return nil
	}

	return displayRoleRecommendation(rec)
}

// doRouteByRole resolves a role to its constituent models, fetches usage from
// all relevant providers, and returns a ranked recommendation.
func doRouteByRole(ctx context.Context, roleName string) (routing.RoleRecommendation, error) {
	cfg := config.Get()
	role, ok := cfg.GetRole(roleName)
	if !ok {
		names := cfg.RoleNames()
		if len(names) > 0 {
			return routing.RoleRecommendation{}, fmt.Errorf("unknown role %q. Available roles: %s", roleName, strings.Join(names, ", "))
		}
		return routing.RoleRecommendation{}, fmt.Errorf("unknown role %q. No roles configured.\nAdd roles to your config:\n  vibeusage config edit\n\nExample:\n  [roles.%s]\n  models = [\"claude-sonnet-4-6\", \"gpt-5\"]", roleName, roleName)
	}

	if len(role.Models) == 0 {
		return routing.RoleRecommendation{}, fmt.Errorf("role %q has no models configured", roleName)
	}

	var modelEntries []routing.RoleModelEntry
	allProviderIDs := make(map[string]bool)

	for _, modelID := range role.Models {
		matches := catalog.MatchPrefix(modelID)
		if len(matches) == 0 {
			if info := catalog.Lookup(modelID); info != nil {
				matches = []catalog.ModelInfo{*info}
			}
		}
		if len(matches) == 0 {
			continue
		}

		best := matches[0]
		configured := provider.ConfiguredIDs(best.Providers)
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
		return routing.RoleRecommendation{}, fmt.Errorf("no models in role %q are available from configured providers.\nModels in role: %s", roleName, strings.Join(role.Models, ", "))
	}

	var providerList []string
	for pid := range allProviderIDs {
		providerList = append(providerList, pid)
	}
	sort.Strings(providerList)

	strategyMap := routing.BuildStrategyMap(providerList, lookupStrategies)
	outcomes := fetchAllWithSpinner(ctx, strategyMap, !refresh)
	providerData := routing.BuildProviderData(outcomes)

	candidates, unavailable := routing.RankByRole(modelEntries, providerData, lookupMultiplier)

	rec := routing.RoleRecommendation{
		Role:        roleName,
		Candidates:  candidates,
		Unavailable: unavailable,
	}
	if len(candidates) > 0 {
		rec.Best = &candidates[0]
	}

	return rec, nil
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
	renderRouteTable(display.FormatRoleRecommendationRows(rec, routeRenderBar, routeFormatReset))
	return nil
}

func listRoles() error {
	cfg := config.Get()
	names := cfg.RoleNames()

	if len(names) == 0 {
		if jsonOutput {
			return display.OutputJSON(outWriter, []struct{}{})
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
		return display.OutputJSON(outWriter, data)
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

// buildModelRolesMap returns a map from canonical model ID to sorted role names.
func buildModelRolesMap(roles map[string][]string) map[string][]string {
	result := make(map[string][]string)

	for roleName, models := range roles {
		for _, modelID := range models {
			matches := catalog.MatchPrefix(modelID)
			if len(matches) == 0 {
				if info := catalog.Lookup(modelID); info != nil {
					matches = []catalog.ModelInfo{*info}
				}
			}
			for _, info := range matches {
				result[info.ID] = append(result[info.ID], roleName)
			}
		}
	}

	for id, roleNames := range result {
		sort.Strings(roleNames)
		deduped := roleNames[:0]
		for i, r := range roleNames {
			if i == 0 || r != roleNames[i-1] {
				deduped = append(deduped, r)
			}
		}
		result[id] = deduped
	}

	return result
}

// renderRouteTable renders a FormattedTable to the output writer using the
// shared route table options (noColor flag, terminal width, row styles).
func renderRouteTable(ft display.FormattedTable) {
	outln(display.NewTableWithOptions(
		ft.Headers,
		ft.Rows,
		display.TableOptions{NoColor: noColor, Width: display.TerminalWidth(), RowStyles: ft.Styles},
	))
}

// routeRenderBar renders a utilization bar with color for the route table.
func routeRenderBar(utilization int) string {
	return display.RenderBar(utilization, 15, display.PaceToColor(nil, utilization))
}

// routeFormatReset formats a duration until reset for the route table.
func routeFormatReset(d *time.Duration) string {
	if d == nil {
		return ""
	}
	return display.FormatResetCountdown(d)
}
