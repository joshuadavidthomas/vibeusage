package display

import (
	"encoding/json"
	"io"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// OutputJSON writes pretty-printed JSON to the given writer.
func OutputJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// SnapshotToJSON converts a fetch outcome to a JSON-serializable value.
// Returns the UsageSnapshot directly for successes (with disabled overage
// stripped), or a SnapshotErrorJSON for failures.
func SnapshotToJSON(outcome fetch.FetchOutcome) any {
	if !outcome.Success || outcome.Snapshot == nil {
		return SnapshotErrorJSON{
			Error: ErrorDetailJSON{
				Message:  outcome.Error,
				Provider: outcome.ProviderID,
			},
		}
	}
	snap := *outcome.Snapshot
	if snap.Overage != nil && !snap.Overage.IsEnabled {
		snap.Overage = nil
	}
	return snap
}

// OutputMultiProviderJSON outputs all outcomes as JSON.
func OutputMultiProviderJSON(w io.Writer, outcomes map[string]fetch.FetchOutcome) error {
	data := multiProviderJSON{
		Providers: make(map[string]models.UsageSnapshot),
		Errors:    make(map[string]string),
		FetchedAt: time.Now().Format(time.RFC3339),
	}

	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			snap := *outcome.Snapshot
			if snap.Overage != nil && !snap.Overage.IsEnabled {
				snap.Overage = nil
			}
			data.Providers[pid] = snap
		} else {
			errMsg := outcome.Error
			if errMsg == "" {
				errMsg = "Unknown error"
			}
			data.Errors[pid] = errMsg
		}
	}

	return OutputJSON(w, data)
}

// OutputStatusJSON outputs provider statuses as JSON.
func OutputStatusJSON(w io.Writer, statuses map[string]models.ProviderStatus) error {
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
	return OutputJSON(w, data)
}
