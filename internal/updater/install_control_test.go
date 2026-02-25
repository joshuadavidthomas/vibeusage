package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssertSelfUpdateSupported_AllowsManagedInstallMarker(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "vibeusage")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	markerPath := filepath.Join(dir, ManagedInstallMarkerFilename)
	if err := os.WriteFile(markerPath, []byte("install-script\n"), 0o644); err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	if err := AssertSelfUpdateSupported(binary); err != nil {
		t.Fatalf("AssertSelfUpdateSupported returned error: %v", err)
	}
}

func TestAssertSelfUpdateSupported_AllowsLegacyLocalBinPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	binaryName := "vibeusage"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(home, ".local", "bin", binaryName)

	if err := AssertSelfUpdateSupported(binary); err != nil {
		t.Fatalf("AssertSelfUpdateSupported returned error: %v", err)
	}
}

func TestAssertSelfUpdateSupported_HomebrewInstall(t *testing.T) {
	binary := "/opt/homebrew/Cellar/vibeusage/0.1.2/bin/vibeusage"
	err := AssertSelfUpdateSupported(binary)
	if err == nil {
		t.Fatal("expected error for homebrew install")
	}
	if !strings.Contains(err.Error(), "brew upgrade vibeusage") {
		t.Fatalf("error = %q, want brew guidance", err.Error())
	}
}

func TestAssertSelfUpdateSupported_UnmanagedInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	binary := filepath.Join(t.TempDir(), "bin", "vibeusage")
	err := AssertSelfUpdateSupported(binary)
	if err == nil {
		t.Fatal("expected error for unmanaged install")
	}
	if !strings.Contains(err.Error(), "official install scripts") {
		t.Fatalf("error = %q, want unmanaged install guidance", err.Error())
	}
}

func TestAssertSelfUpdateSupported_UnmanagedOverride(t *testing.T) {
	t.Setenv("VIBEUSAGE_ALLOW_UNMANAGED_UPDATE", "1")
	binary := filepath.Join(t.TempDir(), "bin", "vibeusage")
	if err := AssertSelfUpdateSupported(binary); err != nil {
		t.Fatalf("AssertSelfUpdateSupported returned error with override: %v", err)
	}
}
