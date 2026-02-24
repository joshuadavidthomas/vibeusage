//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func replaceBinary(targetPath string, binaryBody []byte) (bool, error) {
	stagedPath := targetPath + ".new"
	if err := os.WriteFile(stagedPath, binaryBody, 0o755); err != nil {
		return false, fmt.Errorf("failed to stage update binary: %w", err)
	}

	script, err := os.CreateTemp("", "vibeusage-update-*.cmd")
	if err != nil {
		return false, fmt.Errorf("failed to create update script: %w", err)
	}
	scriptPath := script.Name()

	scriptContent := fmt.Sprintf("@echo off\r\nsetlocal\r\nset \"TARGET=%s\"\r\nset \"NEW=%s\"\r\nfor /L %%%%I in (1,1,45) do (\r\n  move /Y \"%%NEW%%\" \"%%TARGET%%\" >nul 2>&1\r\n  if not errorlevel 1 goto done\r\n  timeout /t 1 /nobreak >nul\r\n)\r\n:done\r\ndel \"%%~f0\" >nul 2>&1\r\n", targetPath, stagedPath)
	if _, err := script.WriteString(scriptContent); err != nil {
		_ = script.Close()
		return false, fmt.Errorf("failed to write update script: %w", err)
	}
	if err := script.Close(); err != nil {
		return false, fmt.Errorf("failed to finalize update script: %w", err)
	}

	if err := os.Chmod(scriptPath, 0o700); err != nil {
		return false, fmt.Errorf("failed to chmod update script: %w", err)
	}

	cmd := exec.Command("cmd", "/C", scriptPath)
	cmd.Dir = filepath.Dir(targetPath)
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("failed to launch update script: %w", err)
	}

	return true, nil
}
