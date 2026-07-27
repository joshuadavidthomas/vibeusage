//go:build !windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func repairCredentialPermissions(path string) error {
	if err := restrictPermissions(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("securing credentials directory: %w", err)
	}
	if err := restrictPermissions(path, 0o600); err != nil {
		return fmt.Errorf("securing credentials file: %w", err)
	}
	return nil
}

func secureCredentialDirectory(path string) error {
	return restrictPermissions(path, 0o700)
}

func secureCredentialFile(path string) error {
	return restrictPermissions(path, 0o600)
}

func restrictPermissions(path string, allowed os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	current := info.Mode().Perm()
	desired := current & allowed
	if current == desired {
		return nil
	}
	return os.Chmod(path, desired)
}
