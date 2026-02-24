//go:build !windows

package updater

import (
	"fmt"
	"os"
	"path/filepath"
)

func replaceBinary(targetPath string, binaryBody []byte) (bool, error) {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, ".vibeusage-update-*")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(binaryBody); err != nil {
		_ = tmpFile.Close()
		return false, fmt.Errorf("failed to write update binary: %w", err)
	}

	mode := os.FileMode(0o755)
	if info, err := os.Stat(targetPath); err == nil {
		mode = info.Mode().Perm()
	}
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return false, fmt.Errorf("failed to set executable permissions on update binary: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return false, fmt.Errorf("failed to sync update binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return false, fmt.Errorf("failed to finalize update binary: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return false, fmt.Errorf("failed to replace executable %s: %w", targetPath, err)
	}
	return false, nil
}
