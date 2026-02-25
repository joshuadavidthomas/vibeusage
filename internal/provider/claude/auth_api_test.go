package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func TestSaveClaudeCredential_SessionKey(t *testing.T) {
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	config.Override(t, config.DefaultConfig())

	sessionKey := fakeClaudeSessionKey("test")
	if err := saveClaudeCredential(sessionKey); err != nil {
		t.Fatalf("saveClaudeCredential error: %v", err)
	}

	data, err := config.ReadCredential(config.CredentialPath("claude", "session"))
	if err != nil {
		t.Fatalf("ReadCredential error: %v", err)
	}
	if data == nil {
		t.Fatal("expected session credential file")
	}

	var stored map[string]string
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if stored["session_key"] != sessionKey {
		t.Errorf("session_key = %q, want %q", stored["session_key"], sessionKey)
	}
}

func TestSaveClaudeCredential_APIKey(t *testing.T) {
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	config.Override(t, config.DefaultConfig())

	apiKey := fakeClaudeAPIKey("test")
	if err := saveClaudeCredential(apiKey); err != nil {
		t.Fatalf("saveClaudeCredential error: %v", err)
	}

	data, err := config.ReadCredential(config.CredentialPath("claude", "apikey"))
	if err != nil {
		t.Fatalf("ReadCredential error: %v", err)
	}
	if data == nil {
		t.Fatal("expected apikey credential file")
	}

	var stored map[string]string
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if stored["api_key"] != apiKey {
		t.Errorf("api_key = %q, want %q", stored["api_key"], apiKey)
	}
}

func TestAPIKeyStrategy_LoadAPIKey_Env(t *testing.T) {
	apiKey := fakeClaudeAPIKey("env")
	t.Setenv("ANTHROPIC_API_KEY", apiKey)

	s := APIKeyStrategy{}
	if got := s.loadAPIKey(); got != apiKey {
		t.Errorf("loadAPIKey() = %q, want %q", got, apiKey)
	}
}

func TestAPIKeyStrategy_LoadAPIKey_File(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	testenv.ApplyVibeusage(t.Setenv, t.TempDir())
	config.Override(t, config.DefaultConfig())

	path := config.CredentialPath("claude", "apikey")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	apiKey := fakeClaudeAPIKey("file")
	content, _ := json.Marshal(map[string]string{"api_key": apiKey})
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	s := APIKeyStrategy{}
	if got := s.loadAPIKey(); got != apiKey {
		t.Errorf("loadAPIKey() = %q, want %q", got, apiKey)
	}
}

func TestAPIKeyStrategy_Fetch_ReturnsFallbackMessage(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", fakeClaudeAPIKey("env"))

	s := APIKeyStrategy{}
	res, err := s.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if res.Success {
		t.Fatal("expected unsuccessful fetch")
	}
	if !res.ShouldFallback {
		t.Fatal("expected should fallback")
	}
	if !strings.Contains(strings.ToLower(res.Error), "requires claude oauth/session") {
		t.Errorf("error = %q, want oauth/session guidance", res.Error)
	}
}

func fakeClaudeSessionKey(suffix string) string {
	return "sk-ant-" + "sid01-" + suffix
}

func fakeClaudeAPIKey(suffix string) string {
	return "sk-ant-" + "api03-" + suffix
}
