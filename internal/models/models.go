package models

import (
	"math"
	"strconv"
	"time"
)

type PeriodType string

const (
	PeriodSession PeriodType = "session"
	PeriodDaily   PeriodType = "daily"
	PeriodWeekly  PeriodType = "weekly"
	PeriodMonthly PeriodType = "monthly"
)

func (p PeriodType) Hours() float64 {
	switch p {
	case PeriodSession:
		return 5.0
	case PeriodDaily:
		return 24.0
	case PeriodWeekly:
		return 7.0 * 24.0
	case PeriodMonthly:
		return 30.0 * 24.0
	default:
		return 24.0
	}
}

type UsagePeriod struct {
	Name        string     `json:"name"`
	Utilization int        `json:"utilization"`
	PeriodType  PeriodType `json:"period_type"`
	ResetsAt    *time.Time `json:"resets_at,omitempty"`
	Model       string     `json:"model,omitempty"`
}

func (p UsagePeriod) Remaining() int {
	return 100 - p.Utilization
}

func (p UsagePeriod) ElapsedRatio() *float64 {
	if p.ResetsAt == nil {
		return nil
	}
	now := time.Now()
	totalHours := p.PeriodType.Hours()
	startTime := p.ResetsAt.Add(-time.Duration(totalHours * float64(time.Hour)))
	elapsed := now.Sub(startTime).Hours()
	ratio := math.Max(0.0, math.Min(elapsed/totalHours, 1.0))
	return &ratio
}

func (p UsagePeriod) PaceRatio() *float64 {
	elapsed := p.ElapsedRatio()
	if elapsed == nil || *elapsed < 0.10 {
		return nil
	}
	expected := *elapsed * 100.0
	ratio := float64(p.Utilization) / expected
	return &ratio
}

func (p UsagePeriod) TimeUntilReset() *time.Duration {
	if p.ResetsAt == nil {
		return nil
	}
	d := time.Until(*p.ResetsAt)
	if d < 0 {
		d = 0
	}
	return &d
}

type OverageUsage struct {
	Used      float64 `json:"used"`
	Limit     float64 `json:"limit"`
	Currency  string  `json:"currency"`
	IsEnabled bool    `json:"is_enabled"`
}

func (o OverageUsage) Remaining() float64 {
	r := o.Limit - o.Used
	if r < 0 {
		return 0
	}
	return r
}

func (o OverageUsage) UtilizationPct() int {
	if o.Limit <= 0 {
		if o.Used > 0 {
			return 100
		}
		return 0
	}
	pct := int((o.Used / o.Limit) * 100)
	if pct > 100 {
		return 100
	}
	return pct
}

type ProviderIdentity struct {
	Email        string `json:"email,omitempty"`
	Organization string `json:"organization,omitempty"`
	Plan         string `json:"plan,omitempty"`
}

type StatusLevel string

const (
	StatusOperational   StatusLevel = "operational"
	StatusDegraded      StatusLevel = "degraded"
	StatusPartialOutage StatusLevel = "partial_outage"
	StatusMajorOutage   StatusLevel = "major_outage"
	StatusUnknown       StatusLevel = "unknown"
)

type ProviderStatus struct {
	Level       StatusLevel `json:"level"`
	Description string      `json:"description,omitempty"`
	UpdatedAt   *time.Time  `json:"updated_at,omitempty"`
}

type UsageSnapshot struct {
	Provider  string            `json:"provider"`
	FetchedAt time.Time         `json:"fetched_at"`
	Periods   []UsagePeriod     `json:"periods"`
	Overage   *OverageUsage     `json:"overage,omitempty"`
	Identity  *ProviderIdentity `json:"identity,omitempty"`
	Status    *ProviderStatus   `json:"status,omitempty"`
	Source    string            `json:"source,omitempty"`
}

func (s UsageSnapshot) PrimaryPeriod() *UsagePeriod {
	if len(s.Periods) == 0 {
		return nil
	}
	priority := map[PeriodType]int{
		PeriodSession: 0,
		PeriodDaily:   1,
		PeriodWeekly:  2,
		PeriodMonthly: 3,
	}
	best := 0
	bestPri := 99
	for i, p := range s.Periods {
		pri, ok := priority[p.PeriodType]
		if !ok {
			pri = 99
		}
		if pri < bestPri {
			bestPri = pri
			best = i
		}
	}
	return &s.Periods[best]
}

// BottleneckPeriod returns the period with the least remaining headroom
// (highest utilization). For routing, this is the constraining limit — if
// session is at 2% but weekly is at 62%, effective headroom is 38%.
func (s UsageSnapshot) BottleneckPeriod() *UsagePeriod {
	if len(s.Periods) == 0 {
		return nil
	}
	best := 0
	for i, p := range s.Periods {
		if p.Utilization > s.Periods[best].Utilization {
			best = i
		}
	}
	return &s.Periods[best]
}

func (s UsageSnapshot) ModelPeriods() []UsagePeriod {
	var result []UsagePeriod
	for _, p := range s.Periods {
		if p.Model != "" {
			result = append(result, p)
		}
	}
	return result
}

func (s UsageSnapshot) IsStale(maxAgeMinutes int) bool {
	return time.Since(s.FetchedAt).Minutes() > float64(maxAgeMinutes)
}

func FormatResetCountdown(d *time.Duration) string {
	if d == nil {
		return ""
	}
	total := int(d.Seconds())
	if total <= 0 {
		return "now"
	}
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	if days > 0 {
		return formatDH(days, hours)
	}
	if hours > 0 {
		return formatHM(hours, minutes)
	}
	return formatM(minutes)
}

func formatDH(d, h int) string { return strconv.Itoa(d) + "d " + strconv.Itoa(h) + "h" }
func formatHM(h, m int) string { return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m" }
func formatM(m int) string     { return strconv.Itoa(m) + "m" }

func PaceToColor(paceRatio *float64, utilization int) string {
	// Exhausted quota is always red — you're blocked regardless of pace.
	if utilization >= 100 {
		return "red"
	}
	if paceRatio == nil {
		if utilization < 50 {
			return "green"
		}
		if utilization < 80 {
			return "yellow"
		}
		return "red"
	}
	// Near-exhaustion floor: ≥90% utilization is always at least yellow.
	// Pace ratio captures burn rate, not how much budget remains. At 90%+
	// you have ≤10% left regardless of pace, which warrants a caution signal.
	// Pace can still escalate near-exhaustion to red, but not rescue it to green.
	if utilization >= 90 {
		if *paceRatio > 1.15 {
			return "red"
		}
		return "yellow"
	}
	if *paceRatio <= 1.15 {
		return "green"
	}
	if *paceRatio <= 1.30 {
		return "yellow"
	}
	return "red"
}
