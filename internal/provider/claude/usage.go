package claude

import (
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// codenameLabels maps Anthropic's internal weekly-bucket codenames to
// their human-readable display names.
var codenameLabels = map[string]string{
	"omelette":             "Claude Design",
	"omelette_promotional": "Claude Design (Promo)",
}

// parseUsageResponse converts an OAuthUsageResponse (returned by both the
// OAuth and web session endpoints) into a UsageSnapshot. The source parameter
// identifies which strategy produced the data ("oauth" or "web").
func parseUsageResponse(resp OAuthUsageResponse, source string) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	standardPeriods := []struct {
		data       *UsagePeriodResponse
		name       string
		periodType models.PeriodType
	}{
		{resp.FiveHour, "Session (5h)", models.PeriodSession},
		{resp.SevenDay, "All Models", models.PeriodWeekly},
	}

	for _, sp := range standardPeriods {
		if sp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        sp.name,
			Utilization: int(sp.data.Utilization),
			PeriodType:  sp.periodType,
		}
		p.ResetsAt = models.ParseRFC3339Ptr(sp.data.ResetsAt)
		periods = append(periods, p)
	}

	modelPeriods := []struct {
		data    *UsagePeriodResponse
		display string
		model   string
	}{
		{resp.SevenDaySonnet, "Sonnet", "sonnet"},
		{resp.SevenDayOpus, "Opus", "opus"},
		{resp.SevenDayOAuthApps, "OAuth Apps", "oauth_apps"},
		{resp.SevenDayCowork, "Cowork", "cowork"},
		{resp.SevenDayOmelette, codenameLabels["omelette"], "omelette"},
		{resp.OmelettePromotional, codenameLabels["omelette_promotional"], "omelette_promotional"},
	}

	for _, mp := range modelPeriods {
		if mp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        mp.display,
			Utilization: int(mp.data.Utilization),
			PeriodType:  models.PeriodWeekly,
			Model:       modelSlug(mp.model),
		}
		p.ResetsAt = models.ParseRFC3339Ptr(mp.data.ResetsAt)
		periods = append(periods, p)
	}

	periods = appendStructuredLimitPeriods(periods, resp.Limits)

	var overage *models.OverageUsage
	if resp.ExtraUsage != nil && resp.ExtraUsage.IsEnabled {
		var limit float64
		if resp.ExtraUsage.MonthlyLimit != nil {
			limit = *resp.ExtraUsage.MonthlyLimit / 100.0
		}
		currency := resp.ExtraUsage.Currency
		if currency == "" {
			currency = "USD"
		}
		resetsAt := firstOfNextMonthUTC(time.Now().UTC())
		overage = &models.OverageUsage{
			Used:      resp.ExtraUsage.UsedCredits / 100.0,
			Limit:     limit,
			Currency:  currency,
			IsEnabled: true,
			ResetsAt:  &resetsAt,
		}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Source:    source,
	}
}

func appendStructuredLimitPeriods(periods []models.UsagePeriod, limits []OAuthLimitResponse) []models.UsagePeriod {
	for _, limit := range limits {
		period := structuredLimitPeriod(limit)
		if period == nil || hasEquivalentPeriod(periods, *period) {
			continue
		}
		periods = append(periods, *period)
	}
	return periods
}

func structuredLimitPeriod(limit OAuthLimitResponse) *models.UsagePeriod {
	name, model := limitDisplay(limit)
	if name == "" {
		return nil
	}
	return &models.UsagePeriod{
		Name:        name,
		Utilization: models.ClampPct(int(limit.Percent)),
		PeriodType:  limitPeriodType(limit),
		Model:       model,
		ResetsAt:    models.ParseRFC3339Ptr(limit.ResetsAt),
	}
}

func limitDisplay(limit OAuthLimitResponse) (string, string) {
	if limit.Scope != nil {
		if limit.Scope.Model != nil {
			name := strings.TrimSpace(limit.Scope.Model.DisplayName)
			model := strings.TrimSpace(limit.Scope.Model.ID)
			if model == "" {
				model = modelSlug(name)
			}
			if name != "" {
				return name, model
			}
		}
		if limit.Scope.Surface != nil {
			name := strings.TrimSpace(limit.Scope.Surface.DisplayName)
			model := strings.TrimSpace(limit.Scope.Surface.ID)
			if model == "" {
				model = modelSlug(name)
			}
			if name != "" {
				return name, model
			}
		}
	}

	switch limit.Kind {
	case "session":
		return "Session (5h)", ""
	case "weekly_all":
		return "All Models", ""
	case "weekly_scoped":
		return "Weekly Scoped", "weekly_scoped"
	default:
		return humanizeUsageKey(limit.Kind), modelSlug(limit.Kind)
	}
}

func limitPeriodType(limit OAuthLimitResponse) models.PeriodType {
	switch limit.Group {
	case "session":
		return models.PeriodSession
	case "daily":
		return models.PeriodDaily
	case "weekly":
		return models.PeriodWeekly
	case "monthly":
		return models.PeriodMonthly
	}
	return periodTypeFromKey(limit.Kind)
}

func periodTypeFromKey(key string) models.PeriodType {
	if strings.HasPrefix(key, "five_hour") || strings.Contains(key, "session") {
		return models.PeriodSession
	}
	if strings.Contains(key, "daily") {
		return models.PeriodDaily
	}
	if strings.Contains(key, "monthly") {
		return models.PeriodMonthly
	}
	return models.PeriodWeekly
}

func humanizeUsageKey(key string) string {
	parts := strings.Fields(strings.ReplaceAll(key, "_", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func modelSlug(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", "_"))
}

func hasEquivalentPeriod(periods []models.UsagePeriod, period models.UsagePeriod) bool {
	for _, existing := range periods {
		if existing.Name == period.Name && existing.PeriodType == period.PeriodType && existing.Model == period.Model {
			return true
		}
	}
	return false
}

// firstOfNextMonthUTC returns midnight UTC on the first day of the month
// following the given time. This is the reset boundary Anthropic uses for
// extra-usage spending caps (matches purchases_reset_at in /prepaid/bundles).
func firstOfNextMonthUTC(now time.Time) time.Time {
	year, month, _ := now.Date()
	return time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
}
