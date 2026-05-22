package gemini

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func writeGeminiCLICreds(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"access_token":"cli-tok","refresh_token":"cli-ref","expires_at":"2099-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "oauth_creds.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLoadCredentials_DeletesOrphanSlotWhenCanonicalFilePresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())

	writeGeminiCLICreds(t, home)
	if err := config.WriteCredential("gemini", "oauth", []byte(`{"access_token":"stale"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	s := OAuthStrategy{}
	creds := s.loadCredentials()
	if creds == nil {
		t.Fatal("loadCredentials() = nil, want canonical creds")
	}
	if creds.AccessToken != "cli-tok" {
		t.Errorf("access_token = %q, want cli-tok", creds.AccessToken)
	}
	if config.HasCredential("gemini", "oauth") {
		t.Error("orphan gemini/oauth slot should have been deleted")
	}
}

func TestLoadCredentials_NoCanonicalSource_PreservesOrphan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())

	if err := config.WriteCredential("gemini", "oauth", []byte(`{"access_token":"stale"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	s := OAuthStrategy{}
	if creds := s.loadCredentials(); creds != nil {
		t.Errorf("loadCredentials() = %+v, want nil (no canonical source)", creds)
	}
	if !config.HasCredential("gemini", "oauth") {
		t.Error("orphan should not be deleted when no canonical source is found")
	}
}

func TestFetch_FailsClosedWhenRefreshNeeded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())

	// Write Gemini CLI creds with an expired access token so NeedsRefresh()
	// triggers the refresh branch.
	dir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"access_token":"stale","refresh_token":"ref","expires_at":"2020-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "oauth_creds.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := OAuthStrategy{}
	res, err := s.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Success {
		t.Fatal("Fetch should fail when Gemini creds need refresh (vibeusage must not refresh externally-owned chains)")
	}
	if !strings.Contains(res.Error, "Gemini CLI") {
		t.Errorf("Fetch error = %q, want guidance to use the Gemini CLI", res.Error)
	}
}
