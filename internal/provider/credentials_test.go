package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

// withIsolatedRegistry replaces the global registry for the duration of
// the test and restores it on cleanup.
func withIsolatedRegistry(t *testing.T) {
	t.Helper()
	orig := registry
	registry = map[string]Provider{}
	t.Cleanup(func() { registry = orig })
}

// withTempCredentialDir points vibeusage credential storage at a temp
// directory so tests don't touch real files.
func withTempCredentialDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	testenv.ApplyVibeusage(t.Setenv, dir)
	config.Override(t, config.DefaultConfig())
}

func TestFindCredential_VibeusageStorage(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{
		id:    "testprov",
		creds: CredentialInfo{EnvVars: []string{"TEST_KEY"}},
	})

	credPath := config.CredentialPath("testprov", "oauth")
	if err := config.WriteCredential(credPath, []byte(`{"token":"x"}`)); err != nil {
		t.Fatalf("WriteCredential: %v", err)
	}

	found, source, path := FindCredential("testprov")
	if !found {
		t.Fatal("expected credential to be found")
	}
	if source != "vibeusage" {
		t.Errorf("source = %q, want vibeusage", source)
	}
	if path != credPath {
		t.Errorf("path = %q, want %q", path, credPath)
	}
}

func TestFindCredential_EnvVar(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{
		id:    "testprov",
		creds: CredentialInfo{EnvVars: []string{"MY_TEST_KEY"}},
	})

	t.Setenv("MY_TEST_KEY", "secret")

	found, source, _ := FindCredential("testprov")
	if !found {
		t.Fatal("expected credential to be found via env var")
	}
	if source != "env" {
		t.Errorf("source = %q, want env", source)
	}
}

func TestFindCredential_UnknownProvider(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	found, source, path := FindCredential("nonexistent")
	if found {
		t.Error("expected no credential for unknown provider")
	}
	if source != "" {
		t.Errorf("source = %q, want empty", source)
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
}

func TestFindCredential_VibeusageTakesPrecedenceOverEnv(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{
		id:    "testprov",
		creds: CredentialInfo{EnvVars: []string{"MY_TEST_KEY"}},
	})

	t.Setenv("MY_TEST_KEY", "secret")
	credPath := config.CredentialPath("testprov", "session")
	_ = config.WriteCredential(credPath, []byte(`{"key":"val"}`))

	_, source, _ := FindCredential("testprov")
	if source != "vibeusage" {
		t.Errorf("vibeusage storage should take precedence, got source = %q", source)
	}
}

func TestFindCredential_CLIPaths(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	// Create a fake CLI credential file
	cliDir := t.TempDir()
	cliFile := filepath.Join(cliDir, "creds.json")
	if err := os.WriteFile(cliFile, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	Register(&stubProvider{
		id:    "testprov",
		creds: CredentialInfo{CLIPaths: []string{cliFile}},
	})

	config.Override(t, config.DefaultConfig())

	found, source, _ := FindCredential("testprov")
	if !found {
		t.Fatal("expected credential to be found via CLI path")
	}
	if source != "provider_cli" {
		t.Errorf("source = %q, want provider_cli", source)
	}
}

func TestCheckCredentials(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{id: "prov1"})
	Register(&stubProvider{
		id:    "prov2",
		creds: CredentialInfo{EnvVars: []string{"PROV2_KEY"}},
	})

	// prov1 has no credentials
	hasCreds, _ := CheckCredentials("prov1")
	if hasCreds {
		t.Error("prov1 should not have credentials")
	}

	// prov2 via env
	t.Setenv("PROV2_KEY", "secret")
	hasCreds, source := CheckCredentials("prov2")
	if !hasCreds {
		t.Error("prov2 should have credentials")
	}
	if source != "env" {
		t.Errorf("source = %q, want env", source)
	}
}

func TestIsFirstRun_NoCreds(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{id: "prov1"})
	Register(&stubProvider{id: "prov2"})

	if !IsFirstRun() {
		t.Error("IsFirstRun should be true when no credentials exist")
	}
}

func TestIsFirstRun_WithCreds(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{id: "prov1"})

	_ = config.WriteCredential(config.CredentialPath("prov1", "apikey"), []byte(`{}`))

	if IsFirstRun() {
		t.Error("IsFirstRun should be false when credentials exist")
	}
}

func TestCountConfigured_None(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{id: "prov1"})

	if got := CountConfigured(); got != 0 {
		t.Errorf("CountConfigured() = %d, want 0", got)
	}
}

func TestCountConfigured_Some(t *testing.T) {
	withIsolatedRegistry(t)
	withTempCredentialDir(t)

	Register(&stubProvider{id: "prov1"})
	Register(&stubProvider{id: "prov2"})
	Register(&stubProvider{id: "prov3"})

	_ = config.WriteCredential(config.CredentialPath("prov1", "oauth"), []byte(`{}`))
	_ = config.WriteCredential(config.CredentialPath("prov3", "session"), []byte(`{}`))

	if got := CountConfigured(); got != 2 {
		t.Errorf("CountConfigured() = %d, want 2", got)
	}
}
