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
// after a 15-second timeout).
//
// "Fresh" means the access token differs from whatever was on disk before
// the subprocess started. Some providers (notably Codex) store credentials
// without an expires_at, so NeedsRefresh() alone can't tell whether the CLI
// has actually rotated the token — checking for a change in access token
// avoids accepting the pre-refresh creds on the first poll.
func RefreshViaCLI(ctx context.Context, cfg CLIRefreshConfig) *Credentials {
	initial := cfg.LoadCredentials()
	var initialToken string
	if initial != nil {
		initialToken = initial.AccessToken
	}

	isFresh := func(c *Credentials) bool {
		if c == nil || c.AccessToken == "" || c.AccessToken == initialToken {
			return false
		}
		return !c.NeedsRefresh()
	}

	binPath, err := exec.LookPath(cfg.BinaryName)
	if err != nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
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
		if creds := cfg.LoadCredentials(); isFresh(creds) {
			killProcess(cmd)
			return creds
		}

		select {
		case <-done:
			creds := cfg.LoadCredentials()
			if !isFresh(creds) {
				return nil
			}
			return creds
		case <-tctx.Done():
			killProcess(cmd)
			creds := cfg.LoadCredentials()
			if !isFresh(creds) {
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
