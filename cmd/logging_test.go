package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestVerboseOutput_MultipleSnapshots_LogsDuration(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logger
	logger = logging.NewLogger(&logBuf)
	logging.Configure(logger, logging.Flags{Verbose: true})
	defer func() { logger = oldLogger }()

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
	oldLogger := logger
	logger = logging.NewLogger(&logBuf)
	logging.Configure(logger, logging.Flags{Verbose: true})
	defer func() { logger = oldLogger }()

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
	oldLogger := logger
	logger = logging.NewLogger(&logBuf)
	logging.Configure(logger, logging.Flags{})
	defer func() { logger = oldLogger }()

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

func TestVerboseOutput_StatusTable_SuppressedInQuiet(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := logger
	logger = logging.NewLogger(&logBuf)
	logging.Configure(logger, logging.Flags{Quiet: true})
	defer func() { logger = oldLogger }()

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

func TestVerboseOutput_NotOnStdout(t *testing.T) {
	// Verbose logging should go to the logger (stderr), NOT to outWriter (stdout).
	// This ensures piped output is clean.
	var logBuf bytes.Buffer
	oldLogger := logger
	logger = logging.NewLogger(&logBuf)
	logging.Configure(logger, logging.Flags{Verbose: true})
	defer func() { logger = oldLogger }()

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
	// Stdout should NOT contain timing/diagnostic info
	if strings.Contains(stdoutOutput, "Total fetch time") {
		t.Errorf("verbose timing info should not appear on stdout, got %q", stdoutOutput)
	}
	if strings.Contains(stdoutOutput, "500ms") {
		t.Errorf("verbose duration should not appear on stdout, got %q", stdoutOutput)
	}
}
