package oauth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefreshViaCLI_ReturnsFreshCredentials(t *testing.T) {
	binDir, credPath := setupFakeCLI(t,
		"#!/usr/bin/env sh\nmkdir -p \"$(dirname \"$CRED_PATH\")\"\n"+
			"cat > \"$CRED_PATH\" <<'JSON'\n"+
			"{\"access_token\":\"fresh\",\"refresh_token\":\"ref\",\"expires_at\":\"2099-01-01T00:00:00Z\"}\n"+
			"JSON\n")

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
	binDir, credPath := setupFakeCLI(t,
		"#!/usr/bin/env sh\nmkdir -p \"$(dirname \"$CRED_PATH\")\"\n"+
			"cat > \"$CRED_PATH\" <<'JSON'\n"+
			"{\"access_token\":\"fresh\",\"refresh_token\":\"ref\",\"expires_at\":\"2099-01-01T00:00:00Z\"}\n"+
			"JSON\nsleep 30\n")

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
	binDir, _ := setupFakeCLI(t, "#!/usr/bin/env sh\nexit 0\n")

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
	binDir, _ := setupFakeCLI(t, "#!/usr/bin/env sh\nsleep 30\n")

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

func TestRefreshViaCLI_PollsMultipleTimes(t *testing.T) {
	binDir, credPath := setupFakeCLI(t,
		"#!/usr/bin/env sh\nsleep 0.1\nmkdir -p \"$(dirname \"$CRED_PATH\")\"\n"+
			"cat > \"$CRED_PATH\" <<'JSON'\n"+
			"{\"access_token\":\"delayed\",\"refresh_token\":\"ref\",\"expires_at\":\"2099-01-01T00:00:00Z\"}\n"+
			"JSON\n")

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

// setupFakeCLI creates a fake "testcli" binary in a temp dir and prepends it
// to PATH. Returns the bin dir and a credential file path the script can write to.
func setupFakeCLI(t *testing.T, script string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(dir, "creds.json")

	bin := filepath.Join(binDir, "testcli")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRED_PATH", credPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return binDir, credPath
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
