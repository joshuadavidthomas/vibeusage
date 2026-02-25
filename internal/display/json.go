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
// Returns SnapshotErrorJSON for failures, snapshotJSON for successes.
func SnapshotToJSON(outcome fetch.FetchOutcome) any {
	if !outcome.Success || outcome.Snapshot == nil {
		return SnapshotErrorJSON{
			Error: ErrorDetailJSON{
				Message:  outcome.Error,
				Provider: outcome.ProviderID,
			},
		}
	}
	return buildSnapshotJSON(outcome)
}

func buildSnapshotJSON(outcome fetch.FetchOutcome) snapshotJSON {
	snap := outcome.Snapshot

	var overage *models.OverageUsage
	if snap.Overage != nil && snap.Overage.IsEnabled {
		overage = snap.Overage
	}

	return snapshotJSON{
		Provider: snap.Provider,
		Source:   outcome.Source,
		Cached:   outcome.Cached,
		Identity: snap.Identity,
		Periods:  snap.Periods,
		Overage:  overage,
	}
}

// OutputMultiProviderJSON outputs all outcomes as JSON.
func OutputMultiProviderJSON(w io.Writer, outcomes map[string]fetch.FetchOutcome) error {
	data := multiProviderJSON{
		Providers: make(map[string]snapshotJSON),
		Errors:    make(map[string]string),
		FetchedAt: time.Now().Format(time.RFC3339),
	}

	for pid, outcome := range outcomes {
		if outcome.Success && outcome.Snapshot != nil {
			data.Providers[pid] = buildSnapshotJSON(outcome)
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
