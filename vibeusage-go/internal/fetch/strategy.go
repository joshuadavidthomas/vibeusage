package fetch

import (
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// FetchResult represents the outcome of a single strategy attempt.
type FetchResult struct {
	Success        bool
	Snapshot       *models.UsageSnapshot
	Error          string
	ShouldFallback bool
}

func ResultOK(snapshot models.UsageSnapshot) FetchResult {
	return FetchResult{Success: true, Snapshot: &snapshot, ShouldFallback: false}
}

func ResultFail(err string) FetchResult {
	return FetchResult{Success: false, Error: err, ShouldFallback: true}
}

func ResultFatal(err string) FetchResult {
	return FetchResult{Success: false, Error: err, ShouldFallback: false}
}

// FetchAttempt records a single attempt at a strategy.
type FetchAttempt struct {
	Strategy   string `json:"strategy"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	DurationMs int    `json:"duration_ms"`
}

// FetchOutcome is the complete result of fetching from a provider.
type FetchOutcome struct {
	ProviderID    string         `json:"provider_id"`
	Success       bool           `json:"success"`
	Snapshot      *models.UsageSnapshot `json:"snapshot,omitempty"`
	Source        string         `json:"source,omitempty"`
	Attempts      []FetchAttempt `json:"attempts"`
	Error         string         `json:"error,omitempty"`
	Cached        bool           `json:"cached"`
	Gated         bool           `json:"gated"`
	Fatal         bool           `json:"fatal"`
	GateRemaining string         `json:"gate_remaining,omitempty"`
}

// Strategy is the interface all fetch strategies must implement.
type Strategy interface {
	Name() string
	IsAvailable() bool
	Fetch() (FetchResult, error)
}
