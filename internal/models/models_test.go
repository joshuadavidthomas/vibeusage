package models

import (
	"testing"
	"time"
)

func TestPeriodTypeHours(t *testing.T) {
	tests := []struct {
		name string
		pt   PeriodType
		want float64
	}{
		{"session", PeriodSession, 5.0},
		{"daily", PeriodDaily, 24.0},
		{"weekly", PeriodWeekly, 168.0},
		{"monthly", PeriodMonthly, 720.0},
		{"unknown defaults to daily", PeriodType("unknown"), 24.0},
		{"empty defaults to daily", PeriodType(""), 24.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pt.Hours()
			if got != tt.want {
				t.Errorf("PeriodType(%q).Hours() = %v, want %v", tt.pt, got, tt.want)
			}
		})
	}
}

func TestUsagePeriodRemaining(t *testing.T) {
	tests := []struct {
		name        string
		utilization int
		want        int
	}{
		{"zero usage", 0, 100},
		{"half usage", 50, 50},
		{"full usage", 100, 0},
		{"over 100", 120, -20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := UsagePeriod{Utilization: tt.utilization}
			if got := p.Remaining(); got != tt.want {
				t.Errorf("Remaining() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatResetCountdown(t *testing.T) {
	dur := func(d time.Duration) *time.Duration { return &d }

	tests := []struct {
		name string
		d    *time.Duration
		want string
	}{
		{"nil duration", nil, ""},
		{"zero", dur(0), "now"},
		{"negative", dur(-5 * time.Minute), "now"},
		{"30 minutes", dur(30 * time.Minute), "30m"},
		{"1 minute", dur(1 * time.Minute), "1m"},
		{"0 minutes (59s)", dur(59 * time.Second), "0m"},
		{"2 hours 15 min", dur(2*time.Hour + 15*time.Minute), "2h 15m"},
		{"1 hour 0 min", dur(1 * time.Hour), "1h 0m"},
		{"1 day 3 hours", dur(27 * time.Hour), "1d 3h"},
		{"2 days 0 hours", dur(48 * time.Hour), "2d 0h"},
		{"7 days 12 hours", dur(180 * time.Hour), "7d 12h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResetCountdown(tt.d)
			if got != tt.want {
				t.Errorf("FormatResetCountdown(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestOverageUsageRemaining(t *testing.T) {
	tests := []struct {
		name string
		o    OverageUsage
		want float64
	}{
		{"normal", OverageUsage{Used: 30, Limit: 100}, 70},
		{"zero used", OverageUsage{Used: 0, Limit: 100}, 100},
		{"fully used", OverageUsage{Used: 100, Limit: 100}, 0},
		{"over limit clamps to zero", OverageUsage{Used: 150, Limit: 100}, 0},
		{"zero limit zero used", OverageUsage{Used: 0, Limit: 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.o.Remaining()
			if got != tt.want {
				t.Errorf("Remaining() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverageUsageUtilizationPct(t *testing.T) {
	tests := []struct {
		name string
		o    OverageUsage
		want int
	}{
		{"50 percent", OverageUsage{Used: 50, Limit: 100}, 50},
		{"zero used", OverageUsage{Used: 0, Limit: 100}, 0},
		{"100 percent", OverageUsage{Used: 100, Limit: 100}, 100},
		{"over limit clamps to 100", OverageUsage{Used: 200, Limit: 100}, 100},
		{"zero limit zero used", OverageUsage{Used: 0, Limit: 0}, 0},
		{"zero limit with usage", OverageUsage{Used: 10, Limit: 0}, 100},
		{"negative limit with usage", OverageUsage{Used: 10, Limit: -5}, 100},
		{"fractional", OverageUsage{Used: 1, Limit: 3}, 33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.o.UtilizationPct()
			if got != tt.want {
				t.Errorf("UtilizationPct() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestElapsedRatio(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		period  UsagePeriod
		wantNil bool
		wantMin float64
		wantMax float64
	}{
		{
			name:    "nil reset returns nil",
			period:  UsagePeriod{PeriodType: PeriodDaily},
			wantNil: true,
		},
		{
			name: "reset far in the future means early in period",
			period: UsagePeriod{
				PeriodType: PeriodDaily,
				ResetsAt:   timePtr(now.Add(23 * time.Hour)),
			},
			wantMin: 0.0,
			wantMax: 0.1,
		},
		{
			name: "reset soon means late in period",
			period: UsagePeriod{
				PeriodType: PeriodDaily,
				ResetsAt:   timePtr(now.Add(1 * time.Hour)),
			},
			wantMin: 0.9,
			wantMax: 1.0,
		},
		{
			name: "reset at halfway",
			period: UsagePeriod{
				PeriodType: PeriodDaily,
				ResetsAt:   timePtr(now.Add(12 * time.Hour)),
			},
			wantMin: 0.45,
			wantMax: 0.55,
		},
		{
			name: "reset in the past clamps to 1.0",
			period: UsagePeriod{
				PeriodType: PeriodDaily,
				ResetsAt:   timePtr(now.Add(-1 * time.Hour)),
			},
			wantMin: 1.0,
			wantMax: 1.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.period.ElapsedRatio()
			if tt.wantNil {
				if got != nil {
					t.Errorf("ElapsedRatio() = %v, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("ElapsedRatio() = nil, want non-nil")
			}
			if *got < tt.wantMin || *got > tt.wantMax {
				t.Errorf("ElapsedRatio() = %v, want between %v and %v", *got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPaceRatio(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		period  UsagePeriod
		wantNil bool
		wantMin float64
		wantMax float64
	}{
		{
			name:    "nil reset returns nil",
			period:  UsagePeriod{PeriodType: PeriodDaily, Utilization: 50},
			wantNil: true,
		},
		{
			name: "too early in period returns nil",
			period: UsagePeriod{
				PeriodType:  PeriodDaily,
				Utilization: 5,
				ResetsAt:    timePtr(now.Add(23 * time.Hour)),
			},
			wantNil: true,
		},
		{
			name: "on pace ratio near 1.0",
			period: UsagePeriod{
				PeriodType:  PeriodDaily,
				Utilization: 50,
				ResetsAt:    timePtr(now.Add(12 * time.Hour)),
			},
			wantMin: 0.9,
			wantMax: 1.1,
		},
		{
			name: "ahead of pace",
			period: UsagePeriod{
				PeriodType:  PeriodDaily,
				Utilization: 90,
				ResetsAt:    timePtr(now.Add(12 * time.Hour)),
			},
			wantMin: 1.7,
			wantMax: 2.0,
		},
		{
			name: "behind pace",
			period: UsagePeriod{
				PeriodType:  PeriodDaily,
				Utilization: 25,
				ResetsAt:    timePtr(now.Add(12 * time.Hour)),
			},
			wantMin: 0.4,
			wantMax: 0.6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.period.PaceRatio()
			if tt.wantNil {
				if got != nil {
					t.Errorf("PaceRatio() = %v, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("PaceRatio() = nil, want non-nil")
			}
			if *got < tt.wantMin || *got > tt.wantMax {
				t.Errorf("PaceRatio() = %v, want between %v and %v", *got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTimeUntilReset(t *testing.T) {
	now := time.Now()
	future := now.Add(2 * time.Hour)
	past := now.Add(-1 * time.Hour)

	tests := []struct {
		name    string
		period  UsagePeriod
		wantNil bool
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "nil reset returns nil",
			period:  UsagePeriod{},
			wantNil: true,
		},
		{
			name:    "future reset returns positive duration",
			period:  UsagePeriod{ResetsAt: &future},
			wantMin: 1*time.Hour + 59*time.Minute,
			wantMax: 2*time.Hour + 1*time.Minute,
		},
		{
			name:    "past reset returns zero",
			period:  UsagePeriod{ResetsAt: &past},
			wantMin: 0,
			wantMax: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.period.TimeUntilReset()
			if tt.wantNil {
				if got != nil {
					t.Errorf("TimeUntilReset() = %v, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("TimeUntilReset() = nil, want non-nil")
			}
			if *got < tt.wantMin || *got > tt.wantMax {
				t.Errorf("TimeUntilReset() = %v, want between %v and %v", *got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPrimaryPeriod(t *testing.T) {
	tests := []struct {
		name    string
		snap    UsageSnapshot
		wantNil bool
		wantPT  PeriodType
	}{
		{
			name:    "empty periods returns nil",
			snap:    UsageSnapshot{},
			wantNil: true,
		},
		{
			name: "single period returned",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "monthly", PeriodType: PeriodMonthly},
				},
			},
			wantPT: PeriodMonthly,
		},
		{
			name: "session preferred over daily",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "daily", PeriodType: PeriodDaily},
					{Name: "session", PeriodType: PeriodSession},
				},
			},
			wantPT: PeriodSession,
		},
		{
			name: "daily preferred over weekly",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "weekly", PeriodType: PeriodWeekly},
					{Name: "daily", PeriodType: PeriodDaily},
				},
			},
			wantPT: PeriodDaily,
		},
		{
			name: "session preferred over all others",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "monthly", PeriodType: PeriodMonthly},
					{Name: "weekly", PeriodType: PeriodWeekly},
					{Name: "session", PeriodType: PeriodSession},
					{Name: "daily", PeriodType: PeriodDaily},
				},
			},
			wantPT: PeriodSession,
		},
		{
			name: "unknown period type treated as lowest priority",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "custom", PeriodType: PeriodType("custom")},
					{Name: "monthly", PeriodType: PeriodMonthly},
				},
			},
			wantPT: PeriodMonthly,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snap.PrimaryPeriod()
			if tt.wantNil {
				if got != nil {
					t.Errorf("PrimaryPeriod() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("PrimaryPeriod() = nil, want non-nil")
			}
			if got.PeriodType != tt.wantPT {
				t.Errorf("PrimaryPeriod().PeriodType = %q, want %q", got.PeriodType, tt.wantPT)
			}
		})
	}
}

func TestBottleneckPeriod(t *testing.T) {
	tests := []struct {
		name     string
		snap     UsageSnapshot
		wantNil  bool
		wantPT   PeriodType
		wantUtil int
	}{
		{
			name:    "empty periods returns nil",
			snap:    UsageSnapshot{},
			wantNil: true,
		},
		{
			name: "single period returned",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "monthly", PeriodType: PeriodMonthly, Utilization: 30},
				},
			},
			wantPT:   PeriodMonthly,
			wantUtil: 30,
		},
		{
			name: "picks highest utilization (weekly > session)",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "session", PeriodType: PeriodSession, Utilization: 2},
					{Name: "weekly", PeriodType: PeriodWeekly, Utilization: 62},
				},
			},
			wantPT:   PeriodWeekly,
			wantUtil: 62,
		},
		{
			name: "session is bottleneck when most used",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "session", PeriodType: PeriodSession, Utilization: 90},
					{Name: "weekly", PeriodType: PeriodWeekly, Utilization: 20},
				},
			},
			wantPT:   PeriodSession,
			wantUtil: 90,
		},
		{
			name: "picks most constrained among many periods",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "session", PeriodType: PeriodSession, Utilization: 10},
					{Name: "daily", PeriodType: PeriodDaily, Utilization: 50},
					{Name: "weekly", PeriodType: PeriodWeekly, Utilization: 80},
					{Name: "monthly", PeriodType: PeriodMonthly, Utilization: 30},
				},
			},
			wantPT:   PeriodWeekly,
			wantUtil: 80,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snap.BottleneckPeriod()
			if tt.wantNil {
				if got != nil {
					t.Errorf("BottleneckPeriod() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("BottleneckPeriod() = nil, want non-nil")
			}
			if got.PeriodType != tt.wantPT {
				t.Errorf("BottleneckPeriod().PeriodType = %q, want %q", got.PeriodType, tt.wantPT)
			}
			if got.Utilization != tt.wantUtil {
				t.Errorf("BottleneckPeriod().Utilization = %d, want %d", got.Utilization, tt.wantUtil)
			}
		})
	}
}

func TestModelPeriods(t *testing.T) {
	tests := []struct {
		name string
		snap UsageSnapshot
		want int
	}{
		{
			name: "no periods",
			snap: UsageSnapshot{},
			want: 0,
		},
		{
			name: "no model periods",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "daily"},
					{Name: "monthly"},
				},
			},
			want: 0,
		},
		{
			name: "all model periods",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "gpt-4", Model: "gpt-4"},
					{Name: "gpt-3.5", Model: "gpt-3.5-turbo"},
				},
			},
			want: 2,
		},
		{
			name: "mixed",
			snap: UsageSnapshot{
				Periods: []UsagePeriod{
					{Name: "daily"},
					{Name: "gpt-4", Model: "gpt-4"},
					{Name: "monthly"},
				},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snap.ModelPeriods()
			if len(got) != tt.want {
				t.Errorf("ModelPeriods() returned %d periods, want %d", len(got), tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	tests := []struct {
		name          string
		fetchedAt     time.Time
		maxAgeMinutes int
		want          bool
	}{
		{
			name:          "fresh",
			fetchedAt:     time.Now(),
			maxAgeMinutes: 5,
			want:          false,
		},
		{
			name:          "stale",
			fetchedAt:     time.Now().Add(-10 * time.Minute),
			maxAgeMinutes: 5,
			want:          true,
		},
		{
			name:          "just under boundary is not stale",
			fetchedAt:     time.Now().Add(-4*time.Minute - 50*time.Second),
			maxAgeMinutes: 5,
			want:          false,
		},
		{
			name:          "very old",
			fetchedAt:     time.Now().Add(-24 * time.Hour),
			maxAgeMinutes: 60,
			want:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UsageSnapshot{FetchedAt: tt.fetchedAt}
			got := s.IsStale(tt.maxAgeMinutes)
			if got != tt.want {
				t.Errorf("IsStale(%d) = %v, want %v", tt.maxAgeMinutes, got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestPaceToColor(t *testing.T) {
	pf := func(v float64) *float64 { return &v }

	tests := []struct {
		name        string
		paceRatio   *float64
		utilization int
		want        string
	}{
		// nil pace ratio - fall back to utilization thresholds
		{"nil pace, low util", nil, 20, "green"},
		{"nil pace, mid util", nil, 50, "yellow"},
		{"nil pace, high util", nil, 79, "yellow"},
		{"nil pace, very high util", nil, 80, "red"},
		{"nil pace, full util", nil, 100, "red"},

		// with pace ratio
		{"pace 0.5 green", pf(0.5), 25, "green"},
		{"pace 1.0 green", pf(1.0), 50, "green"},
		{"pace 1.15 green boundary", pf(1.15), 60, "green"},
		{"pace 1.16 yellow", pf(1.16), 60, "yellow"},
		{"pace 1.30 yellow boundary", pf(1.30), 70, "yellow"},
		{"pace 1.31 red", pf(1.31), 70, "red"},
		{"pace 2.0 red", pf(2.0), 80, "red"},

		// exhausted quota — always red regardless of pace
		// at 100% you're blocked; "on pace" is irrelevant
		{"pace 1.0 util 100 red", pf(1.0), 100, "red"},
		{"pace 1.05 util 100 red", pf(1.05), 100, "red"},
		{"pace 0.5 util 100 red", pf(0.5), 100, "red"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaceToColor(tt.paceRatio, tt.utilization)
			if got != tt.want {
				t.Errorf("PaceToColor(%v, %d) = %q, want %q", tt.paceRatio, tt.utilization, got, tt.want)
			}
		})
	}
}
