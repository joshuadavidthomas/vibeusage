package display

import (
	"encoding/json"
	"io"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// OutputJSON writes pretty-printed JSON to the given writer.
func OutputJSON(w io.Writer, data any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

// SnapshotToJSON converts a snapshot to a JSON-friendly map.
func SnapshotToJSON(outcome fetch.FetchOutcome) map[string]any {
	if !outcome.Success || outcome.Snapshot == nil {
		return map[string]any{
			"error": map[string]any{
				"message":  outcome.Error,
				"provider": outcome.ProviderID,
			},
		}
	}

	snap := outcome.Snapshot
	data := map[string]any{
		"provider": snap.Provider,
		"source":   outcome.Source,
		"cached":   outcome.Cached,
	}

	if snap.Identity != nil {
		data["identity"] = map[string]any{
			"email":        snap.Identity.Email,
			"organization": snap.Identity.Organization,
			"plan":         snap.Identity.Plan,
		}
	}

	var periods []map[string]any
	for _, p := range snap.Periods {
		pd := map[string]any{
			"name":        p.Name,
			"utilization": p.Utilization,
			"remaining":   p.Remaining(),
			"period_type": string(p.PeriodType),
		}
		if p.ResetsAt != nil {
			pd["resets_at"] = p.ResetsAt.Format(time.RFC3339)
		}
		if p.Model != "" {
			pd["model"] = p.Model
		}
		periods = append(periods, pd)
	}
	data["periods"] = periods

	if snap.Overage != nil && snap.Overage.IsEnabled {
		data["overage"] = map[string]any{
			"used":      snap.Overage.Used,
			"limit":     snap.Overage.Limit,
			"remaining": snap.Overage.Remaining(),
			"currency":  snap.Overage.Currency,
		}
	}

	return data
}

// OutputMultiProviderJSON outputs all outcomes as JSON.
func OutputMultiProviderJSON(w io.Writer, outcomes map[string]fetch.FetchOutcome) {
	data := map[string]any{
		"providers":  map[string]any{},
		"errors":     map[string]any{},
		"fetched_at": time.Now().Format(time.RFC3339),
	}

	providers := data["providers"].(map[string]any)
	errors := data["errors"].(map[string]any)

	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			providers[pid] = SnapshotToJSON(outcome)
		} else {
			errMsg := outcome.Error
			if errMsg == "" {
				errMsg = "Unknown error"
			}
			errors[pid] = errMsg
		}
	}

	OutputJSON(w, data)
}

// OutputStatusJSON outputs provider statuses as JSON.
func OutputStatusJSON(w io.Writer, statuses map[string]models.ProviderStatus) {
	data := make(map[string]any)
	for pid, status := range statuses {
		s := map[string]any{
			"level":       string(status.Level),
			"description": status.Description,
		}
		if status.UpdatedAt != nil {
			s["updated_at"] = status.UpdatedAt.Format(time.RFC3339)
		}
		data[pid] = s
	}
	OutputJSON(w, data)
}
