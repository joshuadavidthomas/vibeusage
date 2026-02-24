package cli

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

// newConfiguredLogger creates a new logger configured based on CLI flags.
func newConfiguredLogger() *log.Logger {
	l := logging.NewLogger(os.Stderr)
	logging.Configure(l, logging.Flags{
		Verbose: verbose,
		Quiet:   quiet,
		NoColor: noColor,
		JSON:    jsonOutput,
	})
	return l
}
