package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

func TestTryRefreshViaCLI_UsesSingleHaikuAttempt(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config")
	t.Setenv("VIBEUSAGE_CONFIG_DIR", base)
	t.Setenv("VIBEUSAGE_DATA_DIR", base)

	expired := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	writeOAuthCred(t, "expired", "refresh", expired)

	binDir := filepath.Join(dir, "bin")
	logPath := filepath.Join(dir, "calls.log")
	createFakeClaude(t, binDir, "#!/usr/bin/env sh\nset -eu\necho \"$@\" >> \"$CLAUDE_CALL_LOG\"\ncred=\"$VIBEUSAGE_CONFIG_DIR/credentials/claude/oauth.json\"\nif [ \"${1:-}\" = \"-p\" ]; then\n  mkdir -p \"$(dirname \"$cred\")\"\n  cat > \"$cred\" <<'JSON'\n{\"access_token\":\"fresh-print\",\"refresh_token\":\"refresh\",\"expires_at\":\"2099-01-01T00:00:00Z\"}\nJSON\nfi\n")
	t.Setenv("CLAUDE_CALL_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	s := OAuthStrategy{}
	got := s.tryRefreshViaCLI(context.Background())
	if got == nil {
		t.Fatal("tryRefreshViaCLI() = nil, want refreshed credentials")
	}
	if got.AccessToken != "fresh-print" {
		t.Errorf("access_token = %q, want %q", got.AccessToken, "fresh-print")
	}

	calls := readTrimmed(t, logPath)
	if len(calls) != 1 {
		t.Fatalf("expected 1 CLI call, got %d (%v)", len(calls), calls)
	}
	if !strings.Contains(calls[0], "--model haiku") {
		t.Errorf("call = %q, want haiku model", calls[0])
	}
}

func TestTryRefreshViaCLI_ReturnsQuicklyAfterRefreshEvenIfCLIHangs(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config")
	t.Setenv("VIBEUSAGE_CONFIG_DIR", base)
	t.Setenv("VIBEUSAGE_DATA_DIR", base)

	expired := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	writeOAuthCred(t, "expired", "refresh", expired)

	binDir := filepath.Join(dir, "bin")
	logPath := filepath.Join(dir, "calls.log")
	createFakeClaude(t, binDir, "#!/usr/bin/env sh\nset -eu\necho \"$@\" >> \"$CLAUDE_CALL_LOG\"\ncred=\"$VIBEUSAGE_CONFIG_DIR/credentials/claude/oauth.json\"\nif [ \"${1:-}\" = \"-p\" ]; then\n  mkdir -p \"$(dirname \"$cred\")\"\n  cat > \"$cred\" <<'JSON'\n{\"access_token\":\"fresh-print\",\"refresh_token\":\"refresh\",\"expires_at\":\"2099-01-01T00:00:00Z\"}\nJSON\n  sleep 10\nfi\n")
	t.Setenv("CLAUDE_CALL_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	s := OAuthStrategy{}
	start := time.Now()
	got := s.tryRefreshViaCLI(context.Background())
	elapsed := time.Since(start)
	if got == nil {
		t.Fatal("tryRefreshViaCLI() = nil, want refreshed credentials")
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("tryRefreshViaCLI() took %v, want < 2s", elapsed)
	}

	calls := readTrimmed(t, logPath)
	if len(calls) != 1 {
		t.Fatalf("expected 1 CLI call, got %d (%v)", len(calls), calls)
	}
}

func TestTryRefreshViaCLI_ReturnsNilWhenStillExpired(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config")
	t.Setenv("VIBEUSAGE_CONFIG_DIR", base)
	t.Setenv("VIBEUSAGE_DATA_DIR", base)

	expired := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	writeOAuthCred(t, "expired", "refresh", expired)

	binDir := filepath.Join(dir, "bin")
	logPath := filepath.Join(dir, "calls.log")
	createFakeClaude(t, binDir, "#!/usr/bin/env sh\nset -eu\necho \"$@\" >> \"$CLAUDE_CALL_LOG\"\nexit 0\n")
	t.Setenv("CLAUDE_CALL_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	s := OAuthStrategy{}
	got := s.tryRefreshViaCLI(context.Background())
	if got != nil {
		t.Fatalf("tryRefreshViaCLI() = %+v, want nil", got)
	}

	calls := readTrimmed(t, logPath)
	if len(calls) != 1 {
		t.Fatalf("expected 1 CLI call, got %d (%v)", len(calls), calls)
	}
	if !strings.Contains(calls[0], "--model haiku") {
		t.Errorf("call = %q, want haiku model", calls[0])
	}
}

func writeOAuthCred(t *testing.T, accessToken, refreshToken, expiresAt string) {
	t.Helper()
	path := config.CredentialPath("claude", "oauth")
	content := []byte("{\"access_token\":\"" + accessToken + "\",\"refresh_token\":\"" + refreshToken + "\",\"expires_at\":\"" + expiresAt + "\"}")
	if err := config.WriteCredential(path, content); err != nil {
		t.Fatalf("WriteCredential(%s) error: %v", path, err)
	}
}

func createFakeClaude(t *testing.T, binDir, script string) {
	t.Helper()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error: %v", binDir, err)
	}
	path := filepath.Join(binDir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error: %v", path, err)
	}
}

func readTrimmed(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	out := strings.TrimSpace(string(data))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}
