package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

// resetPathFlags resets configPathCmd flags to defaults and registers
// cleanup to restore them after the test, preventing inter-test leakage.
func resetPathFlags(t *testing.T) {
	t.Helper()
	_ = configPathCmd.Flags().Set("cache", "false")
	_ = configPathCmd.Flags().Set("credentials", "false")
	t.Cleanup(func() {
		_ = configPathCmd.Flags().Set("cache", "false")
		_ = configPathCmd.Flags().Set("credentials", "false")
	})
}

func collectCommandPaths(cmd *cobra.Command, prefix []string) [][]string {
	paths := [][]string{append([]string{}, prefix...)}
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
			continue
		}
		next := append(append([]string{}, prefix...), sub.Name())
		paths = append(paths, collectCommandPaths(sub, next)...)
	}
	return paths
}

// Root command tests

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	expected := []string{"auth", "status", "config", "update", "usage", "route"}
	for _, name := range expected {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rootCmd missing expected subcommand %q", name)
		}
	}
}

func TestRootCmd_HasPersistentFlags(t *testing.T) {
	flags := []string{"json", "no-color", "verbose", "quiet"}
	for _, name := range flags {
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("rootCmd missing persistent flag %q", name)
		}
	}
}

func TestAllCommands_FlagMergingDoesNotPanic(t *testing.T) {
	paths := collectCommandPaths(rootCmd, nil)
	for _, path := range paths {
		path := append([]string{}, path...)
		name := "root"
		if len(path) > 0 {
			name = strings.Join(path, " ")
		}

		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("flag merge panicked for %q: %v", name, r)
				}
			}()

			var cmd *cobra.Command
			if len(path) == 0 {
				cmd = rootCmd
			} else {
				found, _, err := rootCmd.Find(path)
				if err != nil {
					t.Fatalf("failed to resolve command %q: %v", name, err)
				}
				cmd = found
			}

			cmd.InitDefaultHelpFlag()
			_ = cmd.Flags()
		})
	}
}

func TestRootCmd_VersionFlag(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	// Set up temp dir to avoid first-run check
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	rootCmd.SetArgs([]string{"--version"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("root --version error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "vibeusage") {
		t.Errorf("expected 'vibeusage' in version output, got: %q", output)
	}
	if !strings.Contains(output, version) {
		t.Errorf("expected version %q in output, got: %q", version, output)
	}
}

// Config show tests

func TestConfigShow_DefaultOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Config:") {
		t.Errorf("expected 'Config:' prefix, got: %q", output)
	}
	// Should contain TOML output
	if !strings.Contains(output, "timeout") {
		t.Errorf("expected config fields in output, got: %q", output)
	}
}

func TestConfigShow_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	// Quiet mode should only show the path
	if !strings.Contains(output, "config.toml") {
		t.Errorf("quiet mode should show config path, got: %q", output)
	}
}

func TestConfigShow_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show --json error: %v", err)
	}

	var parsed display.ConfigShowJSON
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("config show --json output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if parsed.Path == "" {
		t.Error("JSON output missing 'path'")
	}
}

// Config path tests

func TestConfigPath_DefaultOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path error: %v", err)
	}

	output := buf.String()
	for _, expected := range []string{"Config dir:", "Config file:", "Cache dir:", "Credentials:"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in output, got: %q", expected, output)
		}
	}
}

func TestConfigPath_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != tmpDir {
		t.Errorf("quiet mode should output just the dir, got: %q", output)
	}
}

func TestConfigPath_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path --json error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	for _, key := range []string{"config_dir", "config_file", "cache_dir", "credentials_dir"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON output missing %q key", key)
		}
	}
}

func TestConfigPath_CacheFlag(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)
	_ = configPathCmd.Flags().Set("cache", "true")

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path --cache error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != tmpDir {
		t.Errorf("expected cache dir %q, got %q", tmpDir, output)
	}
}

// Context tests (ExecuteContext)

func TestExecuteContext_PropagatesContext(t *testing.T) {
	// Verify that ExecuteContext sets the context on rootCmd,
	// which commands can access via cmd.Context().
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We can't easily run the full command without providers,
	// but we can verify the API exists and doesn't panic.
	// Pass a cancelled context so any fetch would abort quickly.
	cancel()

	// ExecuteContext should accept the context and not panic.
	// It will likely error because there's no config, which is fine.
	_ = ExecuteContext(ctx)
}

// Spinner/outcome conversion tests

func TestOutcomeToCompletion_Success(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Source:     "oauth",
	}

	got := outcomeToCompletion(o)

	want := display.CompletionInfo{
		ProviderID: "claude",
		Success:    true,
	}

	if got != want {
		t.Errorf("outcomeToCompletion() = %+v, want %+v", got, want)
	}
}

func TestOutcomeToCompletion_Failure(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "cursor",
		Success:    false,
		Error:      "auth failed",
	}

	got := outcomeToCompletion(o)

	want := display.CompletionInfo{
		ProviderID: "cursor",
		Success:    false,
		Error:      "auth failed",
	}

	if got != want {
		t.Errorf("outcomeToCompletion() = %+v, want %+v", got, want)
	}
}

func TestOutcomeToCompletion_Fallback(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "gemini",
		Success:    true,
		Source:     "code_assist",
	}

	got := outcomeToCompletion(o)

	if !got.Success {
		t.Error("expected success=true")
	}
	if got.ProviderID != "gemini" {
		t.Errorf("expected providerID gemini, got %s", got.ProviderID)
	}
}

// Logging/verbose output tests

func TestVerboseOutput_MultipleSnapshots_LogsDuration(t *testing.T) {
	ctx, logBuf := logging.NewTestContext(logging.Flags{Verbose: true})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

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

	displayMultipleSnapshots(ctx, outcomes, 342)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "342") {
		t.Errorf("expected log output to contain duration '342', got %q", logOutput)
	}
}

func TestVerboseOutput_MultipleSnapshots_LogsErrors(t *testing.T) {
	ctx, logBuf := logging.NewTestContext(logging.Flags{Verbose: true})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

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

	displayMultipleSnapshots(ctx, outcomes, 100)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "cursor") {
		t.Errorf("expected log output to contain provider 'cursor', got %q", logOutput)
	}
	if !strings.Contains(logOutput, "auth token expired") {
		t.Errorf("expected log output to contain error message, got %q", logOutput)
	}
}

func TestVerboseOutput_MultipleSnapshots_SuppressedWhenNotVerbose(t *testing.T) {
	ctx, logBuf := logging.NewTestContext(logging.Flags{})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

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

	displayMultipleSnapshots(ctx, outcomes, 500)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "500") {
		t.Errorf("expected no duration in log when not verbose, got %q", logOutput)
	}
}

func TestVerboseOutput_StatusTable_SuppressedInQuiet(t *testing.T) {
	ctx, logBuf := logging.NewTestContext(logging.Flags{Quiet: true})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	statuses := map[string]models.ProviderStatus{
		"claude": {Level: models.StatusOperational, Description: "OK"},
	}

	displayStatusTable(ctx, statuses, 250)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "250") {
		t.Errorf("expected no duration in log in quiet mode, got %q", logOutput)
	}
}

func TestVerboseOutput_NotOnStdout(t *testing.T) {
	// Verbose logging should go to the logger (stderr), NOT to outWriter (stdout).
	// This ensures piped output is clean.
	ctx, _ := logging.NewTestContext(logging.Flags{Verbose: true})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

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

	displayMultipleSnapshots(ctx, outcomes, 500)

	stdoutOutput := outBuf.String()
	// Stdout should NOT contain timing/diagnostic info
	if strings.Contains(stdoutOutput, "Total fetch time") {
		t.Errorf("verbose timing info should not appear on stdout, got %q", stdoutOutput)
	}
	if strings.Contains(stdoutOutput, "500ms") {
		t.Errorf("verbose duration should not appear on stdout, got %q", stdoutOutput)
	}
}

func TestDisplayMultipleSnapshots_NoDataWithErrors_ShowsProviderErrors(t *testing.T) {
	ctx, _ := logging.NewTestContext(logging.Flags{})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    false,
			Error:      "Anthropic API keys are configured, but claude.ai plan usage requires Claude OAuth/session credentials.",
		},
	}

	displayMultipleSnapshots(ctx, outcomes, 0)

	got := outBuf.String()
	if !strings.Contains(got, "No usage data available") {
		t.Fatalf("expected no-data message, got %q", got)
	}
	if !strings.Contains(got, "requires Claude OAuth/session credentials") {
		t.Errorf("expected provider error to be shown, got %q", got)
	}
	if strings.Contains(got, "Configure credentials with:") {
		t.Errorf("did not expect generic configure hint when provider errors are present, got %q", got)
	}
}

func TestDisplayMultipleSnapshots_NoDataNoErrors_ShowsConfigureHint(t *testing.T) {
	ctx, _ := logging.NewTestContext(logging.Flags{})

	var outBuf bytes.Buffer
	outWriter = &outBuf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	outcomes := map[string]fetch.FetchOutcome{}
	displayMultipleSnapshots(ctx, outcomes, 0)

	got := outBuf.String()
	if !strings.Contains(got, "No usage data available") {
		t.Fatalf("expected no-data message, got %q", got)
	}
	if !strings.Contains(got, "vibeusage auth") {
		t.Errorf("expected auth hint, got %q", got)
	}
}
