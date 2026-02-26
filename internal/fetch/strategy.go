package fetch

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// Cache abstracts snapshot persistence so ExecutePipeline doesn't depend
// on the filesystem or config package directly.
type Cache interface {
	Save(snapshot models.UsageSnapshot) error
	Load(providerID string) *models.UsageSnapshot
}

// PipelineConfig holds the parameters that ExecutePipeline needs,
// replacing the previous hidden dependency on config.Get().
type PipelineConfig struct {
	Timeout time.Duration
	Cache   Cache
}

// OrchestratorConfig holds parameters for FetchAllProviders and
// FetchEnabledProviders, replacing config.Get() calls.
type OrchestratorConfig struct {
	MaxConcurrent int
	Pipeline      PipelineConfig
}

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

// FetchOutcome is the complete result of fetching from a provider.
type FetchOutcome struct {
	ProviderID string                `json:"provider_id"`
	Success    bool                  `json:"success"`
	Snapshot   *models.UsageSnapshot `json:"snapshot,omitempty"`
	Source     string                `json:"source,omitempty"`
	Error      string                `json:"error,omitempty"`
	Cached     bool                  `json:"cached"`
}

// Strategy is the interface all fetch strategies must implement.
type Strategy interface {
	IsAvailable() bool
	Fetch(ctx context.Context) (FetchResult, error)
}

// StrategyName returns a short identifier for a strategy derived from its
// type name (e.g. *claude.OAuthStrategy → "oauth").
func StrategyName(s Strategy) string {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.Name()
	name = strings.TrimSuffix(name, "Strategy")
	if name == "" {
		return fmt.Sprintf("%T", s)
	}
	return strings.ToLower(name)
}
