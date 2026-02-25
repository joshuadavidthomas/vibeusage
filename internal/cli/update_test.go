package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/updater"
)

type fakeUpdaterService struct {
	checkResult updater.CheckResult
	checkErr    error
	applyResult updater.ApplyResult
	applyErr    error
	checkCalls  int
	applyCalls  int
}

func (f *fakeUpdaterService) Check(ctx context.Context, req updater.CheckRequest) (updater.CheckResult, error) {
	f.checkCalls++
	if f.checkErr != nil {
		return updater.CheckResult{}, f.checkErr
	}
	return f.checkResult, nil
}

func (f *fakeUpdaterService) Apply(ctx context.Context, req updater.ApplyRequest) (updater.ApplyResult, error) {
	f.applyCalls++
	if f.applyErr != nil {
		return updater.ApplyResult{}, f.applyErr
	}
	return f.applyResult, nil
}

func TestRunUpdate_CheckJSON(t *testing.T) {
	service := &fakeUpdaterService{
		checkResult: updater.CheckResult{
			CurrentVersion:  "v1.0.0",
			LatestVersion:   "v1.1.0",
			TargetVersion:   "v1.1.0",
			UpdateAvailable: true,
			AssetName:       "vibeusage_linux_amd64.tar.gz",
		},
	}

	oldFactory := updaterFactory
	updaterFactory = func() updater.Service { return service }
	defer func() { updaterFactory = oldFactory }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	oldCheckOnly := updateCheckOnly
	updateCheckOnly = true
	defer func() { updateCheckOnly = oldCheckOnly }()

	oldYes := updateYes
	updateYes = false
	defer func() { updateYes = oldYes }()

	oldVersionFlag := updateVersion
	updateVersion = ""
	defer func() { updateVersion = oldVersionFlag }()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := runUpdate(context.Background()); err != nil {
		t.Fatalf("runUpdate error: %v", err)
	}

	var got display.UpdateStatusJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, buf.String())
	}
	if !got.UpdateAvailable {
		t.Fatal("expected update_available=true")
	}
	if got.TargetVersion != "v1.1.0" {
		t.Fatalf("target_version = %q, want %q", got.TargetVersion, "v1.1.0")
	}
}

func TestRunUpdate_ApplyRequiresYesNonInteractive(t *testing.T) {
	service := &fakeUpdaterService{
		checkResult: updater.CheckResult{
			CurrentVersion:  "v1.0.0",
			LatestVersion:   "v1.1.0",
			TargetVersion:   "v1.1.0",
			UpdateAvailable: true,
		},
	}

	oldFactory := updaterFactory
	updaterFactory = func() updater.Service { return service }
	defer func() { updaterFactory = oldFactory }()

	oldSupportChecker := selfUpdateSupportChecker
	selfUpdateSupportChecker = func() error { return nil }
	defer func() { selfUpdateSupportChecker = oldSupportChecker }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	oldCheckOnly := updateCheckOnly
	updateCheckOnly = false
	defer func() { updateCheckOnly = oldCheckOnly }()

	oldYes := updateYes
	updateYes = false
	defer func() { updateYes = oldYes }()

	err := runUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error without --yes in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "rerun with --yes") {
		t.Fatalf("error = %q, want rerun with --yes guidance", err.Error())
	}
	if service.applyCalls != 0 {
		t.Fatalf("applyCalls = %d, want 0", service.applyCalls)
	}
}

func TestRunUpdate_ApplyWithYes(t *testing.T) {
	service := &fakeUpdaterService{
		checkResult: updater.CheckResult{
			CurrentVersion:  "v1.0.0",
			LatestVersion:   "v1.1.0",
			TargetVersion:   "v1.1.0",
			UpdateAvailable: true,
		},
		applyResult: updater.ApplyResult{Updated: true, Pending: false},
	}

	oldFactory := updaterFactory
	updaterFactory = func() updater.Service { return service }
	defer func() { updaterFactory = oldFactory }()

	oldSupportChecker := selfUpdateSupportChecker
	selfUpdateSupportChecker = func() error { return nil }
	defer func() { selfUpdateSupportChecker = oldSupportChecker }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	oldCheckOnly := updateCheckOnly
	updateCheckOnly = false
	defer func() { updateCheckOnly = oldCheckOnly }()

	oldYes := updateYes
	updateYes = true
	defer func() { updateYes = oldYes }()

	oldVersionFlag := updateVersion
	updateVersion = ""
	defer func() { updateVersion = oldVersionFlag }()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := runUpdate(context.Background()); err != nil {
		t.Fatalf("runUpdate error: %v", err)
	}
	if service.applyCalls != 1 {
		t.Fatalf("applyCalls = %d, want 1", service.applyCalls)
	}
	if !strings.Contains(buf.String(), "Updated vibeusage") {
		t.Fatalf("expected success output, got: %s", buf.String())
	}
}

func TestRunUpdate_CheckError(t *testing.T) {
	service := &fakeUpdaterService{checkErr: errors.New("boom")}

	oldFactory := updaterFactory
	updaterFactory = func() updater.Service { return service }
	defer func() { updaterFactory = oldFactory }()

	oldCheckOnly := updateCheckOnly
	updateCheckOnly = true
	defer func() { updateCheckOnly = oldCheckOnly }()

	err := runUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to check for updates") {
		t.Fatalf("error = %q, want wrapped check failure", err.Error())
	}
}

func TestRunUpdate_UnmanagedInstallRejected(t *testing.T) {
	service := &fakeUpdaterService{}

	oldFactory := updaterFactory
	updaterFactory = func() updater.Service { return service }
	defer func() { updaterFactory = oldFactory }()

	oldSupportChecker := selfUpdateSupportChecker
	selfUpdateSupportChecker = func() error {
		return errors.New("self-update is not supported for Homebrew installs")
	}
	defer func() { selfUpdateSupportChecker = oldSupportChecker }()

	oldCheckOnly := updateCheckOnly
	updateCheckOnly = false
	defer func() { updateCheckOnly = oldCheckOnly }()

	err := runUpdate(context.Background())
	if err == nil {
		t.Fatal("expected unmanaged-install error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("error = %q, want unsupported install guidance", err.Error())
	}
	if service.checkCalls != 0 {
		t.Fatalf("checkCalls = %d, want 0", service.checkCalls)
	}
	if service.applyCalls != 0 {
		t.Fatalf("applyCalls = %d, want 0", service.applyCalls)
	}
}
