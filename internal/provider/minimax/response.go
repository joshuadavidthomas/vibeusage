package minimax

import (
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// CodingPlanResponse represents the response from the Minimax coding plan remains endpoint.
// On success, error info is nested in base_resp. On auth failures, status_code
// and status_msg appear at the top level instead.
type CodingPlanResponse struct {
	ModelRemains []ModelRemain `json:"model_remains"`
	BaseResp     BaseResp      `json:"base_resp"`
	// Top-level error fields (flat error responses, e.g. invalid API key).
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// ModelRemain represents a single model's quota within the current 5-hour window.
// This comes from a "remains" endpoint, so CurrentIntervalUsageCount is the
// number of prompts REMAINING (not consumed). Utilization = (total - remaining) / total.
type ModelRemain struct {
	StartTime                 int64  `json:"start_time"`                   // Unix millis
	EndTime                   int64  `json:"end_time"`                     // Unix millis
	RemainsTime               int64  `json:"remains_time"`                 // millis remaining in window
	CurrentIntervalTotalCount int    `json:"current_interval_total_count"` // total prompt quota for the window
	CurrentIntervalUsageCount int    `json:"current_interval_usage_count"` // prompts remaining (not consumed)
	ModelName                 string `json:"model_name"`
}

// BaseResp contains the API status code and message.
type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// Utilization returns the consumed percentage (0-100) for this model.
// CurrentIntervalUsageCount is the REMAINING quota, so:
//
//	consumed = total - remaining
//	utilization = consumed / total * 100
func (m ModelRemain) Utilization() int {
	if m.CurrentIntervalTotalCount <= 0 {
		return 0
	}
	remaining := m.CurrentIntervalUsageCount
	if remaining > m.CurrentIntervalTotalCount {
		remaining = m.CurrentIntervalTotalCount
	}
	if remaining < 0 {
		remaining = 0
	}
	consumed := m.CurrentIntervalTotalCount - remaining
	return (consumed * 100) / m.CurrentIntervalTotalCount
}

// ResetTime converts end_time (Unix millis) to a time.Time pointer.
func (m ModelRemain) ResetTime() *time.Time {
	if m.EndTime == 0 {
		return nil
	}
	t := time.Unix(m.EndTime/1000, (m.EndTime%1000)*int64(time.Millisecond))
	return &t
}

// Remaining returns the number of prompts still available in this window.
// CurrentIntervalUsageCount is already the remaining count (this is a "remains" endpoint).
func (m ModelRemain) Remaining() int {
	if m.CurrentIntervalUsageCount < 0 {
		return 0
	}
	return m.CurrentIntervalUsageCount
}

// ToUsagePeriod converts this model's quota to a UsagePeriod.
func (m ModelRemain) ToUsagePeriod() models.UsagePeriod {
	return models.UsagePeriod{
		Name:        m.ModelName,
		Utilization: m.Utilization(),
		PeriodType:  models.PeriodSession,
		ResetsAt:    m.ResetTime(),
		Model:       m.ModelName,
	}
}

// InferPlan attempts to guess the plan tier from the total prompt count.
// Known values: 500=Starter, 1500=Plus. Others are speculative.
func InferPlan(totalCount int) string {
	switch {
	case totalCount <= 500:
		return "Starter"
	case totalCount <= 1500:
		return "Plus"
	case totalCount <= 5000:
		return "Max"
	default:
		return ""
	}
}
