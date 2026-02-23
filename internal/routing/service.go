package routing

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
)

// ModelInfo holds resolved model information. Mirrors modelmap.ModelInfo
// to avoid coupling routing directly to the modelmap package.
type ModelInfo struct {
	ID        string
	Name      string
	Providers []string
}

// RoleConfig holds a role's model list. Mirrors the config role type.
type RoleConfig struct {
	Models []string
}

// Service provides the business logic for routing queries. Dependencies
// are injected as function fields so the cmd layer can wire them to the
// concrete implementations (modelmap, provider, fetch, config).
type Service struct {
	// LookupModel resolves a query string to a model, or nil if not found.
	LookupModel func(query string) *ModelInfo
	// SearchModels returns fuzzy-match suggestions for a query.
	SearchModels func(query string) []ModelInfo
	// ConfiguredProviders filters provider IDs to those that are registered
	// and have at least one available fetch strategy.
	ConfiguredProviders func(providerIDs []string) []string
	// ProviderStrategies returns fetch strategies for a provider.
	ProviderStrategies func(providerID string) []fetch.Strategy
	// FetchAll fetches usage from all providers in the strategy map.
	FetchAll func(ctx context.Context, strategies map[string][]fetch.Strategy, useCache bool) map[string]fetch.FetchOutcome
	// LookupMultiplier returns the cost multiplier for a model+provider pair.
	LookupMultiplier func(modelName string, providerID string) *float64
	// GetRole returns a role config by name.
	GetRole func(name string) (*RoleConfig, bool)
	// RoleNames returns all configured role names (sorted).
	RoleNames func() []string
	// MatchPrefix returns models matching a prefix.
	MatchPrefix func(prefix string) []ModelInfo
	// UseCache controls whether fetches can use cached data.
	UseCache bool
}

// RouteModel resolves a model query, fetches usage from all configured
// providers that offer it, and returns a ranked recommendation.
func (s *Service) RouteModel(ctx context.Context, query string) (Recommendation, error) {
	info := s.LookupModel(query)
	if info == nil {
		if s.SearchModels != nil {
			suggestions := s.SearchModels(query)
			if len(suggestions) > 0 {
				msg := fmt.Sprintf("unknown model %q. Did you mean:", query)
				for _, sug := range suggestions {
					if len(msg) > 200 {
						break
					}
					msg += fmt.Sprintf("\n  %s (%s)", sug.ID, sug.Name)
				}
				return Recommendation{}, fmt.Errorf("%s", msg)
			}
		}
		return Recommendation{}, fmt.Errorf("unknown model %q. Use 'vibeusage route --list' to see available models", query)
	}

	configuredIDs := s.ConfiguredProviders(info.Providers)
	if len(configuredIDs) == 0 {
		return Recommendation{}, fmt.Errorf(
			"%s is available from %s, but none are configured.\nSet up a provider with: vibeusage auth <provider>",
			info.Name, strings.Join(info.Providers, ", "),
		)
	}

	strategyMap := BuildStrategyMap(configuredIDs, s.ProviderStrategies)

	outcomes := s.FetchAll(ctx, strategyMap, s.UseCache)

	providerData := BuildProviderData(outcomes)

	multipliers := make(map[string]*float64)
	for _, pid := range configuredIDs {
		if mult := s.LookupMultiplier(info.Name, pid); mult != nil {
			multipliers[pid] = mult
		}
	}

	candidates, unavailable := Rank(configuredIDs, providerData, multipliers)

	rec := Recommendation{
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

// RouteByRole resolves a role to its constituent models, fetches usage from
// all relevant providers, and returns a ranked recommendation.
func (s *Service) RouteByRole(ctx context.Context, roleName string) (RoleRecommendation, error) {
	role, ok := s.GetRole(roleName)
	if !ok {
		names := s.RoleNames()
		if len(names) > 0 {
			return RoleRecommendation{}, fmt.Errorf("unknown role %q. Available roles: %s", roleName, strings.Join(names, ", "))
		}
		return RoleRecommendation{}, fmt.Errorf("unknown role %q. No roles configured.\nAdd roles to your config:\n  vibeusage config edit\n\nExample:\n  [roles.%s]\n  models = [\"claude-sonnet-4-6\", \"gpt-5\"]", roleName, roleName)
	}

	if len(role.Models) == 0 {
		return RoleRecommendation{}, fmt.Errorf("role %q has no models configured", roleName)
	}

	var modelEntries []RoleModelEntry
	allProviderIDs := make(map[string]bool)

	for _, modelID := range role.Models {
		matches := s.MatchPrefix(modelID)
		if len(matches) == 0 {
			if info := s.LookupModel(modelID); info != nil {
				matches = []ModelInfo{*info}
			}
		}
		if len(matches) == 0 {
			continue
		}

		best := matches[0]
		configured := s.ConfiguredProviders(best.Providers)
		if len(configured) == 0 {
			continue
		}

		modelEntries = append(modelEntries, RoleModelEntry{
			ModelID:     best.ID,
			ModelName:   best.Name,
			ProviderIDs: configured,
		})
		for _, pid := range configured {
			allProviderIDs[pid] = true
		}
	}

	if len(modelEntries) == 0 {
		return RoleRecommendation{}, fmt.Errorf("no models in role %q are available from configured providers.\nModels in role: %s", roleName, strings.Join(role.Models, ", "))
	}

	var providerList []string
	for pid := range allProviderIDs {
		providerList = append(providerList, pid)
	}
	sort.Strings(providerList)

	strategyMap := BuildStrategyMap(providerList, s.ProviderStrategies)

	outcomes := s.FetchAll(ctx, strategyMap, s.UseCache)

	providerData := BuildProviderData(outcomes)

	multiplierFn := s.LookupMultiplier

	candidates, unavailable := RankByRole(modelEntries, providerData, multiplierFn)

	rec := RoleRecommendation{
		Role:        roleName,
		Candidates:  candidates,
		Unavailable: unavailable,
	}
	if len(candidates) > 0 {
		rec.Best = &candidates[0]
	}

	return rec, nil
}

// BuildProviderData converts fetch outcomes into the ProviderData map
// used by Rank and RankByRole.
func BuildProviderData(outcomes map[string]fetch.FetchOutcome) map[string]ProviderData {
	data := make(map[string]ProviderData)
	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			data[pid] = ProviderData{
				Snapshot: outcome.Snapshot,
				Cached:   outcome.Cached,
			}
		}
	}
	return data
}

// BuildStrategyMap builds a provider→strategies map using the given lookup function.
func BuildStrategyMap(providerIDs []string, lookupStrategies func(string) []fetch.Strategy) map[string][]fetch.Strategy {
	m := make(map[string][]fetch.Strategy, len(providerIDs))
	for _, pid := range providerIDs {
		m[pid] = lookupStrategies(pid)
	}
	return m
}

// BuildModelRolesMap returns a map from canonical model ID to sorted role names.
func BuildModelRolesMap(roles map[string]RoleConfig, matchPrefix func(string) []ModelInfo, lookupModel func(string) *ModelInfo) map[string][]string {
	result := make(map[string][]string)

	for roleName, role := range roles {
		for _, modelID := range role.Models {
			matches := matchPrefix(modelID)
			if len(matches) == 0 {
				if info := lookupModel(modelID); info != nil {
					matches = []ModelInfo{*info}
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
