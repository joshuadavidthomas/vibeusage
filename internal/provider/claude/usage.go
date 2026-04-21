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
	"iguana_necktie":       "Iguana Necktie",
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
		{resp.IguanaNecktie, codenameLabels["iguana_necktie"], "iguana_necktie"},
	}

	for _, mp := range modelPeriods {
		if mp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        mp.display,
			Utilization: int(mp.data.Utilization),
			PeriodType:  models.PeriodWeekly,
			Model:       strings.ToLower(strings.ReplaceAll(mp.model, " ", "_")),
		}
		p.ResetsAt = models.ParseRFC3339Ptr(mp.data.ResetsAt)
		periods = append(periods, p)
	}

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

// firstOfNextMonthUTC returns midnight UTC on the first day of the month
// following the given time. This is the reset boundary Anthropic uses for
// extra-usage spending caps (matches purchases_reset_at in /prepaid/bundles).
func firstOfNextMonthUTC(now time.Time) time.Time {
	year, month, _ := now.Date()
	return time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
}
