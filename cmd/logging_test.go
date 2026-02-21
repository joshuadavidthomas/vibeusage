package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestVerboseOutput_MultipleSnapshots_LogsDuration(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{Verbose: true})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Source:     "oauth",
			Snapshot: &models.UsageSnapshot{
				Provider: "claude",
				Periods: []models.UsagePeriod{
					{Name: "daily", Utilization: 50},
				},
			},
		},
	}

	displayMultipleSnapshots(outcomes, 342)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "342") {
		t.Errorf("expected log output to contain duration '342', got %q", logOutput)
	}
}

func TestVerboseOutput_MultipleSnapshots_LogsErrors(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{Verbose: true})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot: &models.UsageSnapshot{
				Provider: "claude",
				Periods:  []models.UsagePeriod{{Name: "daily", Utilization: 50}},
			},
		},
		"cursor": {
			ProviderID: "cursor",
			Success:    false,
			Error:      "auth token expired",
		},
	}

	displayMultipleSnapshots(outcomes, 100)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "cursor") {
		t.Errorf("expected log output to contain provider 'cursor', got %q", logOutput)
	}
	if !strings.Contains(logOutput, "auth token expired") {
		t.Errorf("expected log output to contain error message, got %q", logOutput)
	}
}

func TestVerboseOutput_MultipleSnapshots_SuppressedWhenNotVerbose(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot: &models.UsageSnapshot{
				Provider: "claude",
				Periods:  []models.UsagePeriod{{Name: "daily", Utilization: 50}},
			},
		},
	}

	displayMultipleSnapshots(outcomes, 500)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "500") {
		t.Errorf("expected no duration in log when not verbose, got %q", logOutput)
	}
}

func TestVerboseOutput_StatusTable_LogsDuration(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{Verbose: true})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	statuses := map[string]models.ProviderStatus{
		"claude": {Level: models.StatusOperational, Description: "OK"},
	}

	displayStatusTable(statuses, 250)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "250") {
		t.Errorf("expected log output to contain duration '250', got %q", logOutput)
	}
}

func TestVerboseOutput_StatusTable_SuppressedInQuiet(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{Quiet: true})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	statuses := map[string]models.ProviderStatus{
		"claude": {Level: models.StatusOperational, Description: "OK"},
	}

	displayStatusTable(statuses, 250)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "250") {
		t.Errorf("expected no duration in log in quiet mode, got %q", logOutput)
	}
}

func TestLoggerConfiguration_CalledInRunDefaultUsage(t *testing.T) {
	// Verify that the logger level is set correctly when flags are configured.
	var logBuf bytes.Buffer
	l := logging.NewLogger(&logBuf)

	// Simulate verbose mode
	logging.Configure(l, logging.Flags{Verbose: true})
	if l.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", l.GetLevel())
	}

	// Simulate quiet mode
	logging.Configure(l, logging.Flags{Quiet: true})
	if l.GetLevel() != log.ErrorLevel {
		t.Errorf("expected ErrorLevel, got %v", l.GetLevel())
	}

	// Simulate default
	logging.Configure(l, logging.Flags{})
	if l.GetLevel() != log.WarnLevel {
		t.Errorf("expected WarnLevel, got %v", l.GetLevel())
	}
}

func TestConfigureLogger_SetsUpFromFlags(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	defer func() { logging.Logger = oldLogger }()

	oldVerbose := verbose
	oldQuiet := quiet
	oldNoColor := noColor
	oldJSON := jsonOutput
	defer func() {
		verbose = oldVerbose
		quiet = oldQuiet
		noColor = oldNoColor
		jsonOutput = oldJSON
	}()

	// Test verbose
	verbose = true
	quiet = false
	noColor = false
	jsonOutput = false
	configureLogger()
	if logging.Logger.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel for verbose, got %v", logging.Logger.GetLevel())
	}

	// Test quiet
	verbose = false
	quiet = true
	configureLogger()
	if logging.Logger.GetLevel() != log.ErrorLevel {
		t.Errorf("expected ErrorLevel for quiet, got %v", logging.Logger.GetLevel())
	}

	// Test default
	verbose = false
	quiet = false
	configureLogger()
	if logging.Logger.GetLevel() != log.WarnLevel {
		t.Errorf("expected WarnLevel for default, got %v", logging.Logger.GetLevel())
	}
}

func TestVerboseOutput_NotOnStdout(t *testing.T) {
	// Verbose logging should go to the logger (stderr), NOT to outWriter (stdout).
	// This ensures piped output is clean.
	var logBuf bytes.Buffer
	oldLogger := logging.Logger
	logging.Logger = logging.NewLogger(&logBuf)
	logging.Configure(logging.Logger, logging.Flags{Verbose: true})
	defer func() { logging.Logger = oldLogger }()

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Source:     "oauth",
			Snapshot: &models.UsageSnapshot{
				Provider: "claude",
				Periods:  []models.UsagePeriod{{Name: "daily", Utilization: 50}},
			},
		},
	}

	displayMultipleSnapshots(outcomes, 500)

	stdoutOutput := outBuf.String()
	// Stdout should NOT contain timing/diagnostic info anymore
	if strings.Contains(stdoutOutput, "Total fetch time") {
		t.Errorf("verbose timing info should not appear on stdout, got %q", stdoutOutput)
	}
	if strings.Contains(stdoutOutput, "500ms") {
		t.Errorf("verbose duration should not appear on stdout, got %q", stdoutOutput)
	}
}
