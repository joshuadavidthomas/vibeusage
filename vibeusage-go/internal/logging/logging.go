package logging

import (
	"io"
	"os"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
)

// Logger is the application-wide logger instance.
// It writes to stderr so it doesn't interfere with piped stdout.
var Logger *log.Logger

func init() {
	Logger = NewLogger(os.Stderr)
}

// Flags holds the CLI flags that affect logging behavior.
type Flags struct {
	Verbose bool
	Quiet   bool
	NoColor bool
	JSON    bool
}

// NewLogger creates a new logger writing to the given writer.
// The default level is WarnLevel (suppresses debug/info).
func NewLogger(w io.Writer) *log.Logger {
	return log.NewWithOptions(w, log.Options{
		Level: log.WarnLevel,
	})
}

// Configure adjusts the logger based on CLI flags.
// Quiet takes precedence over verbose when both are set.
func Configure(l *log.Logger, f Flags) {
	switch {
	case f.Quiet:
		l.SetLevel(log.ErrorLevel)
	case f.Verbose:
		l.SetLevel(log.DebugLevel)
	default:
		l.SetLevel(log.WarnLevel)
	}

	if f.NoColor {
		l.SetColorProfile(termenv.Ascii)
	}

	if f.JSON {
		l.SetFormatter(log.JSONFormatter)
	}
}
