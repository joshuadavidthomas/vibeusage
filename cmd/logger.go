package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

// logger is the application-wide logger instance, explicitly initialized.
// It writes to stderr so it doesn't interfere with piped stdout.
var logger *log.Logger = logging.NewLogger(os.Stderr)
