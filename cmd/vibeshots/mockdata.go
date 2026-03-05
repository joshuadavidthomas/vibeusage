package main

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/routing"
)

// resetIn returns a *time.Time that is d in the future from now.
func resetIn(d time.Duration) *time.Time {
	t := time.Now().Add(d)
	return &t
}

// justNow returns a *time.Time set to the current moment.
func justNow() *time.Time {
	t := time.Now()
	return &t
}

// f64 returns a pointer to a float64 value.
func f64(v float64) *float64 {
	return &v
}

func mockClaudeSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Source:    "oauth",
		Identity: &models.ProviderIdentity{
			Plan: "Pro",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Session (5h)",
				Utilization: 38,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(1*time.Hour + 40*time.Minute),
			},
			{
				Name:        "Weekly",
				Utilization: 98,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(16*time.Hour + 40*time.Minute),
			},
		},
		Overage: &models.OverageUsage{
			Used:      73.72,
			Limit:     0,
			Currency:  "USD",
			IsEnabled: true,
		},
	}
}

func mockClaudeDashboardSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Source:    "oauth",
		Identity: &models.ProviderIdentity{
			Plan: "Pro",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Session (5h)",
				Utilization: 38,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(1*time.Hour + 40*time.Minute),
			},
			{
				Name:        "Weekly",
				Utilization: 99,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(24 * time.Hour),
			},
		},
		Overage: &models.OverageUsage{
			Used:      73.72,
			Limit:     0,
			Currency:  "USD",
			IsEnabled: true,
		},
	}
}

func mockClaudeDetailSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Source:    "oauth",
		Identity: &models.ProviderIdentity{
			Plan: "Pro",
		},
		Status: &models.ProviderStatus{
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Session (5h)",
				Utilization: 10,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(2*time.Hour + 22*time.Minute),
			},
			{
				Name:        "Weekly",
				Utilization: 2,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(6*24*time.Hour + 21*time.Hour),
			},
			{
				Name:        "Sonnet",
				Model:       "sonnet",
				Utilization: 0,
				PeriodType:  models.PeriodWeekly,
			},
		},
		Overage: &models.OverageUsage{
			Used:      73.72,
			Limit:     0,
			Currency:  "USD",
			IsEnabled: true,
		},
	}
}

func mockCodexSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "codex",
		FetchedAt: time.Now(),
		Source:    "oauth",
		Identity: &models.ProviderIdentity{
			Plan: "Plus",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Session",
				Utilization: 15,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(15 * time.Minute),
			},
			{
				Name:        "Weekly",
				Utilization: 12,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(6*24*time.Hour + 19*time.Hour),
			},
		},
	}
}

func mockCursorSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "cursor",
		FetchedAt: time.Now(),
		Source:    "api_key",
		Identity: &models.ProviderIdentity{
			Plan: "Pro",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Plan Usage",
				Utilization: 72,
				PeriodType:  models.PeriodMonthly,
				ResetsAt:    resetIn(12*24*time.Hour + 6*time.Hour),
			},
		},
	}
}

func mockGeminiSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "gemini",
		FetchedAt: time.Now(),
		Source:    "api_key",
		Identity: &models.ProviderIdentity{
			Plan: "Free",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Daily",
				Utilization: 4,
				PeriodType:  models.PeriodDaily,
				ResetsAt:    resetIn(9*time.Hour + 32*time.Minute),
			},
		},
	}
}

func mockCopilotSnapshot() models.UsageSnapshot {
	return models.UsageSnapshot{
		Provider:  "copilot",
		FetchedAt: time.Now(),
		Source:    "device_flow",
		Identity: &models.ProviderIdentity{
			Plan: "Individual",
		},
		Periods: []models.UsagePeriod{
			{
				Name:        "Monthly (Premium)",
				Utilization: 11,
				PeriodType:  models.PeriodMonthly,
				ResetsAt:    resetIn(4*24*time.Hour + 1*time.Hour),
			},
			{
				Name:        "Monthly (Chat)",
				Utilization: 0,
				PeriodType:  models.PeriodMonthly,
				ResetsAt:    resetIn(4*24*time.Hour + 1*time.Hour),
			},
			{
				Name:        "Monthly (Completions)",
				Utilization: 0,
				PeriodType:  models.PeriodMonthly,
				ResetsAt:    resetIn(4*24*time.Hour + 1*time.Hour),
			},
		},
	}
}

func mockStatuslineOutcomes() map[string]fetch.FetchOutcome {
	// Use slightly different numbers for the statusline to match README examples.
	claude := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Session (5h)",
				Utilization: 62,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(1 * time.Hour),
			},
			{
				Name:        "Weekly",
				Utilization: 98,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(16 * time.Hour),
			},
		},
	}
	codex := models.UsageSnapshot{
		Provider:  "codex",
		FetchedAt: time.Now(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Session",
				Utilization: 15,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(15 * time.Minute),
			},
			{
				Name:        "Weekly",
				Utilization: 12,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(6*24*time.Hour + 9*time.Hour),
			},
		},
	}
	copilot := models.UsageSnapshot{
		Provider:  "copilot",
		FetchedAt: time.Now(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Monthly (Premium)",
				Utilization: 11,
				PeriodType:  models.PeriodMonthly,
				ResetsAt:    resetIn(4*24*time.Hour + 1*time.Hour),
			},
		},
	}

	return map[string]fetch.FetchOutcome{
		"claude":  {ProviderID: "claude", Success: true, Snapshot: &claude},
		"codex":   {ProviderID: "codex", Success: true, Snapshot: &codex},
		"copilot": {ProviderID: "copilot", Success: true, Snapshot: &copilot},
	}
}

func mockStatuslineSingleOutcome() map[string]fetch.FetchOutcome {
	claude := models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: time.Now(),
		Periods: []models.UsagePeriod{
			{
				Name:        "Session (5h)",
				Utilization: 12,
				PeriodType:  models.PeriodSession,
				ResetsAt:    resetIn(2 * time.Hour),
			},
			{
				Name:        "Weekly",
				Utilization: 11,
				PeriodType:  models.PeriodWeekly,
				ResetsAt:    resetIn(5*24*time.Hour + 12*time.Hour),
			},
		},
	}
	return map[string]fetch.FetchOutcome{
		"claude": {ProviderID: "claude", Success: true, Snapshot: &claude},
	}
}

func mockRouteModelRecommendation() routing.Recommendation {
	return routing.Recommendation{
		ModelID:   "claude-opus-4-6",
		ModelName: "Claude Opus 4.6",
		Candidates: []routing.Candidate{
			{
				ProviderID:        "antigravity",
				Headroom:          100,
				Utilization:       0,
				EffectiveHeadroom: 100,
				PeriodType:        models.PeriodWeekly,
				ResetsAt:          resetIn(1*time.Hour + 19*time.Minute),
				Plan:              "Antigravity",
			},
			{
				ProviderID:        "copilot",
				Headroom:          89,
				Utilization:       11,
				Multiplier:        f64(3),
				EffectiveHeadroom: 29,
				PeriodType:        models.PeriodMonthly,
				ResetsAt:          resetIn(4*24*time.Hour + 1*time.Hour),
				Plan:              "individual",
			},
			{
				ProviderID:        "claude",
				Headroom:          1,
				Utilization:       99,
				EffectiveHeadroom: 1,
				PeriodType:        models.PeriodWeekly,
				ResetsAt:          resetIn(16*time.Hour + 24*time.Minute),
				Plan:              "Pro",
			},
		},
	}
}

func mockRouteRoleRecommendation() routing.RoleRecommendation {
	return routing.RoleRecommendation{
		Role: "coding",
		Candidates: []routing.RoleCandidate{
			{
				ModelID:   "claude-opus-4-6",
				ModelName: "Claude Opus 4.6",
				Candidate: routing.Candidate{
					ProviderID:        "antigravity",
					Headroom:          100,
					Utilization:       0,
					EffectiveHeadroom: 100,
					PeriodType:        models.PeriodWeekly,
					ResetsAt:          resetIn(2*time.Hour + 24*time.Minute),
					Plan:              "Antigravity",
				},
			},
			{
				ModelID:   "gpt-5.3-codex",
				ModelName: "GPT-5.3 Codex",
				Candidate: routing.Candidate{
					ProviderID:        "codex",
					Headroom:          88,
					Utilization:       12,
					EffectiveHeadroom: 88,
					PeriodType:        models.PeriodWeekly,
					ResetsAt:          resetIn(6*24*time.Hour + 18*time.Hour),
					Plan:              "plus",
				},
			},
			{
				ModelID:   "claude-opus-4-6",
				ModelName: "Claude Opus 4.6",
				Candidate: routing.Candidate{
					ProviderID:        "copilot",
					Headroom:          89,
					Utilization:       11,
					Multiplier:        f64(3),
					EffectiveHeadroom: 29,
					PeriodType:        models.PeriodMonthly,
					ResetsAt:          resetIn(4*24*time.Hour + 1*time.Hour),
					Plan:              "individual",
				},
			},
			{
				ModelID:   "claude-opus-4-6",
				ModelName: "Claude Opus 4.6",
				Candidate: routing.Candidate{
					ProviderID:        "claude",
					Headroom:          1,
					Utilization:       99,
					EffectiveHeadroom: 1,
					PeriodType:        models.PeriodWeekly,
					ResetsAt:          resetIn(16*time.Hour + 24*time.Minute),
					Plan:              "Pro",
				},
			},
		},
	}
}

func mockProviderStatuses() map[string]models.ProviderStatus {
	// Descriptions are now derived from Level via DisplayDescription().
	// Only non-operational statuses with active incidents set Description
	// as an override (e.g. incident titles from upstream APIs).
	return map[string]models.ProviderStatus{
		"amp": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"antigravity": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"claude": {
			Level:       models.StatusPartialOutage,
			Description: "Partial System Outage",
			UpdatedAt:   justNow(),
		},
		"codex": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"copilot": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"cursor": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"gemini": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"kimicode": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"minimax": {
			Level: models.StatusUnknown,
		},
		"openrouter": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"warp": {
			Level:     models.StatusOperational,
			UpdatedAt: justNow(),
		},
		"zai": {
			Level: models.StatusUnknown,
		},
	}
}
