package cli

import (
	"context"
	"fmt"
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

// newRoutingService creates a routing.Service wired to the concrete
// implementations: modelmap, provider, fetch, config.
func newRoutingService() *routing.Service {
	return &routing.Service{
		LookupModel:         adaptLookup,
		SearchModels:        adaptSearch,
		ConfiguredProviders: provider.ConfiguredIDs,
		ProviderStrategies: func(id string) []fetch.Strategy {
			p, _ := provider.Get(id)
			return p.FetchStrategies()
		},
		FetchAll: func(ctx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome {
			orchCfg := orchestratorConfigFromConfig(config.Get())
			return fetch.FetchAllProviders(ctx, strategies, useCache, orchCfg, nil)
		},
		LookupMultiplier: func(modelName string, providerID string) *float64 {
			if providerID == "copilot" {
				return catalog.LookupMultiplier(modelName)
			}
			return nil
		},
		GetRole: func(name string) (*routing.RoleConfig, bool) {
			cfg := config.Get()
			role, ok := cfg.GetRole(name)
			if !ok {
				return nil, false
			}
			return &routing.RoleConfig{Models: role.Models}, true
		},
		RoleNames: func() []string {
			return config.Get().RoleNames()
		},
		MatchPrefix: adaptMatchPrefix,
		UseCache:    !refresh,
	}
}

// newRoutingServiceWithSpinner creates a routing.Service where FetchAll
// is wrapped with a spinner for terminal output.
func newRoutingServiceWithSpinner() *routing.Service {
	svc := newRoutingService()
	svc.FetchAll = func(fetchCtx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome {
		providerIDs := make([]string, 0, len(strategies))
		for pid := range strategies {
			providerIDs = append(providerIDs, pid)
		}

		var outcomes map[string]fetch.FetchOutcome
		orchCfg := orchestratorConfigFromConfig(config.Get())
		if display.SpinnerShouldShow(quiet, jsonOutput, !isTerminal()) {
			_ = display.SpinnerRun(providerIDs, func(onComplete func(display.CompletionInfo)) {
				outcomes = fetch.FetchAllProviders(fetchCtx, strategies, useCache, orchCfg, func(o fetch.FetchOutcome) {
					onComplete(outcomeToCompletion(o))
				})
			})
		} else {
			outcomes = fetch.FetchAllProviders(fetchCtx, strategies, useCache, orchCfg, nil)
		}
		return outcomes
	}
	return svc
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
	roles := make(map[string]routing.RoleConfig)
	for name, role := range cfg.Roles {
		roles[name] = routing.RoleConfig{Models: role.Models}
	}
	modelRoles := routing.BuildModelRolesMap(roles, adaptMatchPrefix, adaptLookup)

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

func routeModel(cmd *cobra.Command, query string) error {
	svc := newRoutingServiceWithSpinner()

	rec, err := svc.RouteModel(cmd.Context(), query)
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

func displayRecommendation(rec routing.Recommendation) error {
	if rec.Best == nil {
		out("No usage data available for %s from any configured provider.\n", rec.ModelName)
		if len(rec.Unavailable) > 0 {
			out("Failed to fetch: %s\n", strings.Join(rec.Unavailable, ", "))
		}
		return nil
	}

	out("Route: %s\n\n", rec.ModelName)
	renderRouteTable(routing.FormatRecommendationRows(rec, routeRenderBar, routeFormatReset))
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

func routeByRole(cmd *cobra.Command, roleName string) error {
	svc := newRoutingServiceWithSpinner()

	rec, err := svc.RouteByRole(cmd.Context(), roleName)
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
	renderRouteTable(routing.FormatRoleRecommendationRows(rec, routeRenderBar, routeFormatReset))
	return nil
}

// Adapter functions to convert between modelmap types and routing types.

func adaptLookup(query string) *routing.ModelInfo {
	info := catalog.Lookup(query)
	if info == nil {
		return nil
	}
	return &routing.ModelInfo{ID: info.ID, Name: info.Name, Providers: info.Providers}
}

func adaptSearch(query string) []routing.ModelInfo {
	results := catalog.Search(query)
	out := make([]routing.ModelInfo, len(results))
	for i, r := range results {
		out[i] = routing.ModelInfo{ID: r.ID, Name: r.Name, Providers: r.Providers}
	}
	return out
}

func adaptMatchPrefix(prefix string) []routing.ModelInfo {
	results := catalog.MatchPrefix(prefix)
	out := make([]routing.ModelInfo, len(results))
	for i, r := range results {
		out[i] = routing.ModelInfo{ID: r.ID, Name: r.Name, Providers: r.Providers}
	}
	return out
}

// renderRouteTable renders a FormattedTable to the output writer using the
// shared route table options (noColor flag, terminal width, row styles).
func renderRouteTable(ft routing.FormattedTable) {
	outln(display.NewTableWithOptions(
		ft.Headers,
		ft.Rows,
		display.TableOptions{NoColor: noColor, Width: display.TerminalWidth(), RowStyles: toDisplayStyles(ft.Styles)},
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

// toDisplayStyles converts routing.RowStyle to display.RowStyle.
func toDisplayStyles(styles []routing.RowStyle) []display.RowStyle {
	result := make([]display.RowStyle, len(styles))
	for i, s := range styles {
		switch s {
		case routing.RowBold:
			result[i] = display.RowBold
		case routing.RowDim:
			result[i] = display.RowDim
		default:
			result[i] = display.RowNormal
		}
	}
	return result
}
