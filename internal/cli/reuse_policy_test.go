package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func TestAvailableProviderIDs_IgnoresExternalCredentialsWhenReuseDisabled(t *testing.T) {
	dir := t.TempDir()
	testenv.ApplyVibeusage(t.Setenv, dir)

	home := filepath.Join(dir, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	for _, p := range provider.All() {
		for _, envVar := range p.CredentialSources().EnvVars {
			t.Setenv(envVar, "")
		}
	}

	cfg := config.DefaultConfig()
	cfg.Credentials.ReuseProviderCredentials = false
	if err := config.Save(cfg, ""); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	reloadConfig()

	for _, p := range provider.All() {
		for _, raw := range p.CredentialSources().CLIPaths {
			path := expandHomePath(raw, home)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll(%s): %v", path, err)
			}
			if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
				t.Fatalf("WriteFile(%s): %v", path, err)
			}
		}
	}

	// Antigravity has an additional external source outside CredentialSources.
	vscdb := filepath.Join(home, ".config", "Antigravity", "User", "globalStorage", "state.vscdb")
	if err := os.MkdirAll(filepath.Dir(vscdb), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", vscdb, err)
	}
	if err := os.WriteFile(vscdb, []byte("not-a-real-db"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", vscdb, err)
	}

	pm := buildProviderMap()
	got := availableProviderIDs(pm, config.Get())
	if len(got) != 0 {
		t.Fatalf("availableProviderIDs() = %v, want none when reuse_provider_credentials=false and only external creds exist", got)
	}
}

func expandHomePath(path string, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
