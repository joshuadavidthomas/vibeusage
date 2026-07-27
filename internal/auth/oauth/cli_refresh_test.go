package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

const (
	cliRefreshHelperEnv      = "GO_WANT_CLI_REFRESH_HELPER"
	cliRefreshScenarioEnv    = "CLI_REFRESH_HELPER_SCENARIO"
	cliRefreshCredentialsEnv = "CRED_PATH"
)

func TestMain(m *testing.M) {
	if os.Getenv(cliRefreshHelperEnv) == "1" {
		os.Exit(runCLIRefreshHelper(os.Getenv(cliRefreshScenarioEnv), os.Getenv(cliRefreshCredentialsEnv)))
	}
	os.Exit(m.Run())
}

func TestRefreshViaCLI_ReturnsFreshCredentials(t *testing.T) {
	binDir, credPath := setupFakeCLI(t, "fresh")

	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName:      "testcli",
		Args:            []string{"refresh"},
		LoadCredentials: credLoader(credPath),
	})
	_ = binDir
	if got == nil {
		t.Fatal("RefreshViaCLI() = nil, want credentials")
	}
	if got.AccessToken != "fresh" {
		t.Errorf("access_token = %q, want %q", got.AccessToken, "fresh")
	}
}

func TestRefreshViaCLI_ReturnsQuicklyWhenCLIHangs(t *testing.T) {
	binDir, credPath := setupFakeCLI(t, "write-then-hang")

	start := time.Now()
	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName:      "testcli",
		Args:            []string{"refresh"},
		LoadCredentials: credLoader(credPath),
	})
	elapsed := time.Since(start)
	_ = binDir

	if got == nil {
		t.Fatal("RefreshViaCLI() = nil, want credentials")
	}
	if elapsed >= 2*time.Second {
		t.Errorf("took %v, want < 2s", elapsed)
	}
}

func TestRefreshViaCLI_ReturnsNilWhenCredentialsStayExpired(t *testing.T) {
	binDir, _ := setupFakeCLI(t, "success")

	expired := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName: "testcli",
		Args:       []string{"refresh"},
		LoadCredentials: func() *Credentials {
			return &Credentials{
				AccessToken:  "stale",
				RefreshToken: "ref",
				ExpiresAt:    expired,
			}
		},
	})
	_ = binDir

	if got != nil {
		t.Errorf("RefreshViaCLI() = %+v, want nil", got)
	}
}

func TestRefreshViaCLI_ReturnsNilWhenBinaryNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName: "nonexistent-cli",
		Args:       []string{"refresh"},
		LoadCredentials: func() *Credentials {
			return nil
		},
	})

	if got != nil {
		t.Errorf("RefreshViaCLI() = %+v, want nil", got)
	}
}

func TestRefreshViaCLI_RespectsContextCancellation(t *testing.T) {
	binDir, _ := setupFakeCLI(t, "hang")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	got := RefreshViaCLI(ctx, CLIRefreshConfig{
		BinaryName: "testcli",
		Args:       []string{"refresh"},
		LoadCredentials: func() *Credentials {
			return nil
		},
	})
	elapsed := time.Since(start)
	_ = binDir

	if got != nil {
		t.Errorf("RefreshViaCLI() = %+v, want nil", got)
	}
	if elapsed >= 1*time.Second {
		t.Errorf("took %v, want fast return on cancelled context", elapsed)
	}
}

func TestRefreshViaCLI_DoesNotShortCircuitWhenCredsHaveNoExpiry(t *testing.T) {
	// Regression: providers like Codex store creds without expires_at, so
	// NeedsRefresh() returns false even for stale tokens. RefreshViaCLI must
	// not accept the pre-refresh creds on the first poll just because they
	// look "non-expiring".
	binDir, credPath := setupFakeCLI(t, "delayed-no-expiry")

	// Pre-write the existing creds file with the *old* token (no expires_at).
	initial := `{"access_token":"stale","refresh_token":"oldref"}`
	if err := os.WriteFile(credPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName:      "testcli",
		Args:            []string{"refresh"},
		LoadCredentials: credLoader(credPath),
	})
	_ = binDir

	if got == nil {
		t.Fatal("RefreshViaCLI() = nil, want refreshed creds")
	}
	if got.AccessToken != "refreshed" {
		t.Errorf("access_token = %q, want %q (must wait for CLI to write new token)", got.AccessToken, "refreshed")
	}
}

func TestRefreshViaCLI_RejectsChangedButExpiredToken(t *testing.T) {
	// A changed access token still has to satisfy !NeedsRefresh(). If the
	// CLI rotates to a token that's already expired, treat it as a failed
	// refresh — we don't want the caller to immediately re-enter the
	// refresh path.
	binDir, credPath := setupFakeCLI(t, "delayed-expired")

	initial := `{"access_token":"stale","refresh_token":"r","expires_at":"2020-01-01T00:00:00Z"}`
	if err := os.WriteFile(credPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName:      "testcli",
		Args:            []string{"refresh"},
		LoadCredentials: credLoader(credPath),
	})
	_ = binDir

	if got != nil {
		t.Errorf("RefreshViaCLI() = %+v, want nil for a token that was rotated to an already-expired value", got)
	}
}

func TestRefreshViaCLI_ReturnsNilWhenCLIDoesNotChangeToken(t *testing.T) {
	// If the CLI exits without rotating the token, refresh has effectively
	// failed — we must return nil rather than the unchanged creds.
	binDir, credPath := setupFakeCLI(t, "success")

	initial := `{"access_token":"unchanged","refresh_token":"ref"}`
	if err := os.WriteFile(credPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName:      "testcli",
		Args:            []string{"refresh"},
		LoadCredentials: credLoader(credPath),
	})
	_ = binDir

	if got != nil {
		t.Errorf("RefreshViaCLI() = %+v, want nil when token did not change", got)
	}
}

func TestRefreshViaCLI_PollsMultipleTimes(t *testing.T) {
	binDir, credPath := setupFakeCLI(t, "polling")

	var calls atomic.Int32
	got := RefreshViaCLI(context.Background(), CLIRefreshConfig{
		BinaryName: "testcli",
		Args:       []string{"refresh"},
		LoadCredentials: func() *Credentials {
			calls.Add(1)
			return credLoader(credPath)()
		},
	})
	_ = binDir

	if got == nil {
		t.Fatal("RefreshViaCLI() = nil, want credentials")
	}
	if got.AccessToken != "delayed" {
		t.Errorf("access_token = %q, want %q", got.AccessToken, "delayed")
	}
	if n := calls.Load(); n < 2 {
		t.Errorf("LoadCredentials called %d times, want >= 2 (proving polling)", n)
	}
}

// setupFakeCLI copies the current test binary into PATH as "testcli". TestMain
// detects the helper environment and runs the requested CLI behavior in Go.
func setupFakeCLI(t *testing.T, scenario string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(dir, "creds.json")

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	binaryName := "testcli"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binPath := filepath.Join(binDir, binaryName)
	copyTestBinary(t, testBinary, binPath)

	// A killed executable can remain locked for a moment on Windows. Remove it
	// before TempDir's cleanup runs so the test does not race process teardown.
	t.Cleanup(func() {
		deadline := time.Now().Add(2 * time.Second)
		for {
			err := os.Remove(binPath)
			if err == nil || os.IsNotExist(err) {
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("remove helper executable: %v", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Setenv(cliRefreshHelperEnv, "1")
	t.Setenv(cliRefreshScenarioEnv, scenario)
	t.Setenv(cliRefreshCredentialsEnv, credPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return binDir, credPath
}

func copyTestBinary(t *testing.T, sourcePath, targetPath string) {
	t.Helper()
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("open test binary: %v", err)
	}
	defer func() { _ = source.Close() }()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create helper executable: %v", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatalf("copy helper executable: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close helper executable: %v", err)
	}
	if err := os.Chmod(targetPath, 0o755); err != nil {
		t.Fatalf("chmod helper executable: %v", err)
	}
}

func runCLIRefreshHelper(scenario, credPath string) int {
	writeCredentials := func(body string) int {
		if credPath == "" {
			_, _ = fmt.Fprintln(os.Stderr, "credential path is empty")
			return 2
		}
		if err := os.MkdirAll(filepath.Dir(credPath), 0o755); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "create credential directory: %v\n", err)
			return 2
		}
		if err := os.WriteFile(credPath, []byte(body), 0o600); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "write credentials: %v\n", err)
			return 2
		}
		return 0
	}

	switch scenario {
	case "fresh":
		return writeCredentials(`{"access_token":"fresh","refresh_token":"ref","expires_at":"2099-01-01T00:00:00Z"}`)
	case "write-then-hang":
		if code := writeCredentials(`{"access_token":"fresh","refresh_token":"ref","expires_at":"2099-01-01T00:00:00Z"}`); code != 0 {
			return code
		}
		time.Sleep(30 * time.Second)
		return 0
	case "success":
		return 0
	case "hang":
		time.Sleep(30 * time.Second)
		return 0
	case "delayed-no-expiry":
		time.Sleep(50 * time.Millisecond)
		return writeCredentials(`{"access_token":"refreshed","refresh_token":"newref"}`)
	case "delayed-expired":
		time.Sleep(50 * time.Millisecond)
		return writeCredentials(`{"access_token":"changed","refresh_token":"r","expires_at":"2020-01-01T00:00:00Z"}`)
	case "polling":
		time.Sleep(100 * time.Millisecond)
		return writeCredentials(`{"access_token":"delayed","refresh_token":"ref","expires_at":"2099-01-01T00:00:00Z"}`)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown CLI refresh helper scenario %q\n", scenario)
		return 2
	}
}

// credLoader returns a LoadCredentials function that reads from a JSON file.
func credLoader(path string) func() *Credentials {
	return func() *Credentials {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var creds Credentials
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil
		}
		if creds.AccessToken == "" {
			return nil
		}
		return &creds
	}
}
