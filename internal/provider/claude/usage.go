package claude

import (
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// parseUsageResponse converts an OAuthUsageResponse (returned by both the
// OAuth and web session endpoints) into a UsageSnapshot. The source parameter
// identifies which strategy produced the data ("oauth" or "web"). An optional
// overage override can be provided for strategies that fetch overage data from
// a separate endpoint.
func parseUsageResponse(resp OAuthUsageResponse, source string, overageOverride *models.OverageUsage) *models.UsageSnapshot {
	var periods []models.UsagePeriod

	type periodInfo struct {
		name       string
		periodType models.PeriodType
	}

	standardPeriods := []struct {
		data *UsagePeriodResponse
		info periodInfo
	}{
		{resp.FiveHour, periodInfo{"Session (5h)", models.PeriodSession}},
		{resp.SevenDay, periodInfo{"All Models", models.PeriodWeekly}},
		{resp.Monthly, periodInfo{"Monthly", models.PeriodMonthly}},
	}

	for _, sp := range standardPeriods {
		if sp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        sp.info.name,
			Utilization: int(sp.data.Utilization),
			PeriodType:  sp.info.periodType,
		}
		p.ResetsAt = models.ParseRFC3339Ptr(sp.data.ResetsAt)
		periods = append(periods, p)
	}

	modelPeriods := []struct {
		data      *UsagePeriodResponse
		modelName string
	}{
		{resp.SevenDaySonnet, "Sonnet"},
		{resp.SevenDayOpus, "Opus"},
		{resp.SevenDayHaiku, "Haiku"},
	}

	for _, mp := range modelPeriods {
		if mp.data == nil {
			continue
		}
		p := models.UsagePeriod{
			Name:        mp.modelName,
			Utilization: int(mp.data.Utilization),
			PeriodType:  models.PeriodWeekly,
			Model:       strings.ToLower(mp.modelName),
		}
		p.ResetsAt = models.ParseRFC3339Ptr(mp.data.ResetsAt)
		periods = append(periods, p)
	}

	// Prefer inline extra_usage from the response; fall back to the
	// override provided by the caller (e.g. from a separate endpoint).
	var overage *models.OverageUsage
	if resp.ExtraUsage != nil && resp.ExtraUsage.IsEnabled {
		overage = &models.OverageUsage{
			Used:      resp.ExtraUsage.UsedCredits / 100.0,
			Limit:     resp.ExtraUsage.MonthlyLimit / 100.0,
			Currency:  "USD",
			IsEnabled: true,
		}
	} else if overageOverride != nil {
		overage = overageOverride
	}

	// Build identity from plan field or infer from usage features.
	plan := resp.Plan
	if plan == "" && resp.BillingType != "" {
		plan = resp.BillingType
	}
	if plan == "" {
		plan = inferClaudePlan(resp)
	}

	var identity *models.ProviderIdentity
	if plan != "" {
		identity = &models.ProviderIdentity{Plan: plan}
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Overage:   overage,
		Identity:  identity,
		Source:    source,
	}
}
