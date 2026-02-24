package zai

import (
	"fmt"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// QuotaResponse represents the response from the Z.ai quota/limit endpoint.
type QuotaResponse struct {
	Code    int        `json:"code"`
	Msg     string     `json:"msg"`
	Data    *QuotaData `json:"data,omitempty"`
	Success bool       `json:"success"`
}

// QuotaData contains the quota limits and subscription level.
type QuotaData struct {
	Limits []QuotaLimit `json:"limits"`
	Level  string       `json:"level"`
}

// QuotaLimit represents a single quota limit entry.
type QuotaLimit struct {
	Type          string        `json:"type"`
	Unit          int           `json:"unit"`
	Number        int           `json:"number"`
	Percentage    int           `json:"percentage"`
	NextResetTime int64         `json:"nextResetTime"` // Unix millis
	Usage         int           `json:"usage,omitempty"`
	CurrentValue  int           `json:"currentValue,omitempty"`
	Remaining     int           `json:"remaining,omitempty"`
	UsageDetails  []UsageDetail `json:"usageDetails,omitempty"`
}

// UsageDetail represents per-tool usage within a TIME_LIMIT entry.
type UsageDetail struct {
	ModelCode string `json:"modelCode"`
	Usage     int    `json:"usage"`
}

// PeriodType maps the unit+number fields to a vibeusage period type.
func (q QuotaLimit) PeriodType() models.PeriodType {
	switch q.Unit {
	case unitHours:
		if q.Number <= 5 {
			return models.PeriodSession
		}
		if q.Number <= 24 {
			return models.PeriodDaily
		}
		return models.PeriodWeekly
	case unitDays:
		if q.Number <= 1 {
			return models.PeriodDaily
		}
		if q.Number <= 7 {
			return models.PeriodWeekly
		}
		return models.PeriodMonthly
	case unitMonths:
		return models.PeriodMonthly
	default:
		return models.PeriodMonthly
	}
}

// ResetTime converts the millisecond Unix timestamp to a time.Time.
func (q QuotaLimit) ResetTime() *time.Time {
	if q.NextResetTime == 0 {
		return nil
	}
	t := time.Unix(q.NextResetTime/1000, (q.NextResetTime%1000)*int64(time.Millisecond))
	return &t
}

// DisplayName returns a human-readable name for the limit type.
//
// TOKENS_LIMIT hourly windows use "Session (Nh)" to match the convention used
// by Claude and Antigravity. Other token periods fall back to a labelled quota.
// TIME_LIMIT windows describe the tool quota period (e.g. "Monthly Tools").
func (q QuotaLimit) DisplayName() string {
	switch q.Type {
	case typeTokens:
		if q.Unit == unitHours {
			return fmt.Sprintf("Session (%dh)", q.Number)
		}
		return q.periodLabel("Quota")
	case typeTime:
		return q.periodLabel("Tools")
	default:
		return q.Type
	}
}

// periodLabel builds a period-prefixed label such as "Monthly Tools" or "Daily Quota".
func (q QuotaLimit) periodLabel(suffix string) string {
	switch q.Unit {
	case unitDays:
		if q.Number == 1 {
			return fmt.Sprintf("Daily %s", suffix)
		}
		return fmt.Sprintf("%d Day %s", q.Number, suffix)
	case unitMonths:
		if q.Number == 1 {
			return fmt.Sprintf("Monthly %s", suffix)
		}
		return fmt.Sprintf("%d Month %s", q.Number, suffix)
	default:
		return suffix
	}
}

// Known limit types.
const (
	typeTokens = "TOKENS_LIMIT"
	typeTime   = "TIME_LIMIT"
)

// Known unit values.
const (
	unitHours  = 3
	unitMonths = 5
	unitDays   = 4 // assumed
)

// PlanName returns a human-readable plan name from the level string.
func PlanName(level string) string {
	switch level {
	case "lite":
		return "Lite"
	case "pro":
		return "Pro"
	case "max":
		return "Max"
	default:
		if level != "" {
			return level
		}
		return ""
	}
}
