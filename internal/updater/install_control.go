package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// ManagedInstallMarkerFilename is written by the official install scripts.
	// Self-update is only enabled when this marker is present (or when the
	// install matches legacy ~/.local/bin behavior).
	ManagedInstallMarkerFilename = ".vibeusage-managed-by"
	managedInstallMarkerValue    = "install-script"
)

// AssertSelfUpdateSupported returns an error when self-update should be
// disabled for the current installation method.
func AssertSelfUpdateSupported(binaryPath string) error {
	if strings.TrimSpace(os.Getenv("VIBEUSAGE_ALLOW_UNMANAGED_UPDATE")) == "1" {
		return nil
	}

	targetPath, err := resolveBinaryPath(binaryPath)
	if err != nil {
		return err
	}

	if isManagedInstall(targetPath) || isLegacyManagedPath(targetPath) {
		return nil
	}

	if isHomebrewInstallPath(targetPath) {
		return fmt.Errorf("self-update is not supported for Homebrew installs; use `brew upgrade %s`", projectName)
	}

	return fmt.Errorf("self-update is only supported for installs managed by the official install scripts; rerun install.sh/install.ps1 or use your package manager to upgrade")
}

func isManagedInstall(binaryPath string) bool {
	markerPath := filepath.Join(filepath.Dir(binaryPath), ManagedInstallMarkerFilename)
	markerBody, err := os.ReadFile(markerPath)
	if err != nil {
		return false
	}

	markerValue := strings.TrimSpace(string(markerBody))
	return markerValue == "" || markerValue == managedInstallMarkerValue
}

func isLegacyManagedPath(binaryPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}

	expected := filepath.Join(home, ".local", "bin", filepath.Base(binaryPath))
	return samePath(expected, binaryPath)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func isHomebrewInstallPath(binaryPath string) bool {
	path := filepath.ToSlash(filepath.Clean(binaryPath))
	return strings.Contains(path, "/Cellar/"+projectName+"/")
}
