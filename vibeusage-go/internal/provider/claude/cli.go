package claude

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

var (
	ansiPattern  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	usagePattern = regexp.MustCompile(`█\s*([\d.]+)%\s*(?:\(([^)]+)\)|\[([^\]]+)\])`)
)

type CLIStrategy struct{}

func (s *CLIStrategy) Name() string { return "cli" }

func (s *CLIStrategy) IsAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (s *CLIStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	cmd := exec.CommandContext(ctx, "claude", "/usage")
	output, err := cmd.Output()
	if err != nil {
		return fetch.ResultFail("claude CLI failed: " + err.Error()), nil
	}

	snapshot := s.parseCLIOutput(string(output))
	if snapshot == nil {
		return fetch.ResultFail("Failed to parse claude CLI output"), nil
	}

	return fetch.ResultOK(*snapshot), nil
}

func (s *CLIStrategy) parseCLIOutput(output string) *models.UsageSnapshot {
	clean := ansiPattern.ReplaceAllString(output, "")

	var periods []models.UsagePeriod
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "█") {
			continue
		}
		matches := usagePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		util, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			continue
		}
		periodName := matches[2]
		if periodName == "" {
			periodName = matches[3]
		}
		if periodName == "" {
			periodName = "Usage"
		}
		periodName = strings.TrimSpace(periodName)

		periods = append(periods, models.UsagePeriod{
			Name:        periodName,
			Utilization: int(util),
			PeriodType:  classifyPeriod(periodName),
		})
	}

	if len(periods) == 0 {
		return nil
	}

	now := time.Now().UTC()
	return &models.UsageSnapshot{
		Provider:  "claude",
		FetchedAt: now,
		Periods:   periods,
		Source:    "cli",
	}
}

func classifyPeriod(name string) models.PeriodType {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "hour") || strings.Contains(lower, "session"):
		return models.PeriodSession
	case strings.Contains(lower, "day"):
		return models.PeriodDaily
	case strings.Contains(lower, "week"):
		return models.PeriodWeekly
	case strings.Contains(lower, "month") || strings.Contains(lower, "billing"):
		return models.PeriodMonthly
	default:
		return models.PeriodDaily
	}
}
