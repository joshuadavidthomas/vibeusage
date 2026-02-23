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
	_ = enc.Encode(data)
}

// SnapshotToJSON converts a snapshot to a typed JSON-serializable struct.
func SnapshotToJSON(outcome fetch.FetchOutcome) any {
	if !outcome.Success || outcome.Snapshot == nil {
		return SnapshotErrorJSON{
			Error: ErrorDetailJSON{
				Message:  outcome.Error,
				Provider: outcome.ProviderID,
			},
		}
	}

	snap := outcome.Snapshot
	data := SnapshotJSON{
		Provider: snap.Provider,
		Source:   outcome.Source,
		Cached:   outcome.Cached,
	}

	if snap.Identity != nil {
		data.Identity = &IdentityJSON{
			Email:        snap.Identity.Email,
			Organization: snap.Identity.Organization,
			Plan:         snap.Identity.Plan,
		}
	}

	var periods []PeriodJSON
	for _, p := range snap.Periods {
		pd := PeriodJSON{
			Name:        p.Name,
			Utilization: p.Utilization,
			Remaining:   p.Remaining(),
			PeriodType:  string(p.PeriodType),
			Model:       p.Model,
		}
		if p.ResetsAt != nil {
			pd.ResetsAt = p.ResetsAt.Format(time.RFC3339)
		}
		periods = append(periods, pd)
	}
	data.Periods = periods

	if snap.Overage != nil && snap.Overage.IsEnabled {
		data.Overage = &OverageJSON{
			Used:      snap.Overage.Used,
			Limit:     snap.Overage.Limit,
			Remaining: snap.Overage.Remaining(),
			Currency:  snap.Overage.Currency,
		}
	}

	return data
}

// OutputMultiProviderJSON outputs all outcomes as JSON.
func OutputMultiProviderJSON(w io.Writer, outcomes map[string]fetch.FetchOutcome) {
	data := MultiProviderJSON{
		Providers: make(map[string]any),
		Errors:    make(map[string]string),
		FetchedAt: time.Now().Format(time.RFC3339),
	}

	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			data.Providers[pid] = SnapshotToJSON(outcome)
		} else {
			errMsg := outcome.Error
			if errMsg == "" {
				errMsg = "Unknown error"
			}
			data.Errors[pid] = errMsg
		}
	}

	OutputJSON(w, data)
}

// OutputStatusJSON outputs provider statuses as JSON.
func OutputStatusJSON(w io.Writer, statuses map[string]models.ProviderStatus) {
	data := make(map[string]StatusEntryJSON)
	for pid, status := range statuses {
		entry := StatusEntryJSON{
			Level:       string(status.Level),
			Description: status.Description,
		}
		if status.UpdatedAt != nil {
			entry.UpdatedAt = status.UpdatedAt.Format(time.RFC3339)
		}
		data[pid] = entry
	}
	OutputJSON(w, data)
}
