package oauth

import (
	"context"
	"io"
	"os/exec"
	"time"
)

// CLIRefreshConfig holds the parameters for a CLI-based token refresh.
type CLIRefreshConfig struct {
	// BinaryName is the CLI executable to look up in PATH (e.g. "claude", "codex").
	BinaryName string
	// Args are the command-line arguments that trigger a lightweight
	// operation whose side effect is refreshing credentials on disk.
	Args []string
	// LoadCredentials is called repeatedly to check whether the CLI has
	// written fresh credentials. Return nil if no valid credentials are found.
	LoadCredentials func() *Credentials
}

// RefreshViaCLI attempts to refresh OAuth credentials by spawning a CLI tool
// that refreshes tokens as a side effect on startup. It polls for fresh
// credentials every 25ms and kills the process as soon as they appear (or
// after a 2-second timeout).
func RefreshViaCLI(ctx context.Context, cfg CLIRefreshConfig) *Credentials {
	binPath, err := exec.LookPath(cfg.BinaryName)
	if err != nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tctx, binPath, cfg.Args...)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		if creds := cfg.LoadCredentials(); creds != nil && !creds.NeedsRefresh() {
			killProcess(cmd)
			return creds
		}

		select {
		case <-done:
			creds := cfg.LoadCredentials()
			if creds == nil || creds.NeedsRefresh() {
				return nil
			}
			return creds
		case <-tctx.Done():
			killProcess(cmd)
			creds := cfg.LoadCredentials()
			if creds == nil || creds.NeedsRefresh() {
				return nil
			}
			return creds
		case <-ticker.C:
		}
	}
}

func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
