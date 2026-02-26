package claude

import (
	"encoding/json"
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

func fakeClaudeSessionKey(suffix string) string {
	return "sk-ant-" + "sid01-" + suffix
}
