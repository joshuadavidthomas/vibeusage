package cmd

import (
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show health status for all providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		statuses := fetchAllStatuses()
		durationMs := time.Since(start).Milliseconds()

		if jsonOutput {
			display.OutputStatusJSON(outWriter, statuses)
			return nil
		}

		displayStatusTable(statuses, durationMs)
		return nil
	},
}

func fetchAllStatuses() map[string]models.ProviderStatus {
	statuses := make(map[string]models.ProviderStatus)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for id, p := range provider.All() {
		wg.Add(1)
		go func(pid string, prov provider.Provider) {
			defer wg.Done()
			status := prov.FetchStatus()
			mu.Lock()
			statuses[pid] = status
			mu.Unlock()
		}(id, p)
	}

	wg.Wait()
	return statuses
}

func displayStatusTable(statuses map[string]models.ProviderStatus, durationMs int64) {
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if quiet {
		for _, pid := range ids {
			s := statuses[pid]
			out("%s: %s %s\n", pid, display.StatusSymbol(s.Level, noColor), string(s.Level))
		}
		return
	}

	var rows [][]string
	for _, pid := range ids {
		s := statuses[pid]
		desc := s.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		rows = append(rows, []string{
			pid,
			display.StatusSymbol(s.Level, noColor),
			desc,
			display.FormatStatusUpdated(s.UpdatedAt),
		})
	}

	outln(display.NewTableWithOptions(
		[]string{"Provider", "Status", "Description", "Updated"},
		rows,
		display.TableOptions{Title: "Provider Status", NoColor: noColor, Width: display.TerminalWidth()},
	))

	if durationMs > 0 {
		logger.Debug("status fetch complete", "duration_ms", durationMs)
	}
}
