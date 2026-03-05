package display

import (
	"sort"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// FormatStatusRows builds table headers and rows from a provider status map.
// Rows are sorted by provider ID. Descriptions are truncated to 30 characters.
func FormatStatusRows(statuses map[string]models.ProviderStatus, noColor bool) (headers []string, rows [][]string) {
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	headers = []string{"Provider", "Status", "Description", "Updated"}

	for _, pid := range ids {
		s := statuses[pid]
		desc := s.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		rows = append(rows, []string{
			pid,
			StatusSymbol(s.Level, noColor),
			desc,
			FormatStatusUpdated(s.UpdatedAt),
		})
	}

	return headers, rows
}
