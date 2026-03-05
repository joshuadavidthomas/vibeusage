package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateCredentials_MigratesExistingFiles(t *testing.T) {
	dir := setupTempDir(t)

	// Create old-style credential files
	oldDir := filepath.Join(dir, "data", "credentials")
	claudeDir := filepath.Join(oldDir, "claude")
	cursorDir := filepath.Join(oldDir, "cursor")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.MkdirAll(cursorDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "oauth.json"), []byte(`{"access_token":"abc"}`), 0o600)
	_ = os.WriteFile(filepath.Join(cursorDir, "session.json"), []byte(`{"session_token":"xyz"}`), 0o600)

	if err := MigrateCredentials(); err != nil {
		t.Fatalf("MigrateCredentials() error: %v", err)
	}

	// Verify credentials were migrated
	data, err := ReadCredential("claude", "oauth")
	if err != nil {
		t.Fatalf("ReadCredential(claude, oauth) error: %v", err)
	}
	var claudeCreds map[string]string
	if err := json.Unmarshal(data, &claudeCreds); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if claudeCreds["access_token"] != "abc" {
		t.Errorf("claude access_token = %q, want %q", claudeCreds["access_token"], "abc")
	}

	data, err = ReadCredential("cursor", "session")
	if err != nil {
		t.Fatalf("ReadCredential(cursor, session) error: %v", err)
	}
	var cursorCreds map[string]string
	if err := json.Unmarshal(data, &cursorCreds); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cursorCreds["session_token"] != "xyz" {
		t.Errorf("cursor session_token = %q, want %q", cursorCreds["session_token"], "xyz")
	}

	// Old directory should be cleaned up
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old credentials directory should have been removed")
	}
}

func TestMigrateCredentials_NoopWhenNoOldDir(t *testing.T) {
	setupTempDir(t)

	if err := MigrateCredentials(); err != nil {
		t.Fatalf("MigrateCredentials() error: %v", err)
	}

	// No credentials file should be created
	if _, err := os.Stat(CredentialsFile()); !os.IsNotExist(err) {
		t.Error("credentials file should not be created when nothing to migrate")
	}
}

func TestMigrateCredentials_DoesNotOverwriteExisting(t *testing.T) {
	dir := setupTempDir(t)

	// Write a credential to the new store first
	_ = WriteCredential("claude", "oauth", []byte(`{"access_token":"new"}`))

	// Create old-style credential with different value
	oldDir := filepath.Join(dir, "data", "credentials")
	claudeDir := filepath.Join(oldDir, "claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "oauth.json"), []byte(`{"access_token":"old"}`), 0o600)

	if err := MigrateCredentials(); err != nil {
		t.Fatalf("MigrateCredentials() error: %v", err)
	}

	// Should keep the new value, not overwrite with old
	data, _ := ReadCredential("claude", "oauth")
	var creds map[string]string
	_ = json.Unmarshal(data, &creds)
	if creds["access_token"] != "new" {
		t.Errorf("access_token = %q, want %q (should not overwrite)", creds["access_token"], "new")
	}
}

func TestMigrateCredentials_SkipsInvalidJSON(t *testing.T) {
	dir := setupTempDir(t)

	// Create old-style credential with invalid JSON
	oldDir := filepath.Join(dir, "data", "credentials")
	badDir := filepath.Join(oldDir, "badprov")
	_ = os.MkdirAll(badDir, 0o755)
	_ = os.WriteFile(filepath.Join(badDir, "oauth.json"), []byte(`not json`), 0o600)

	if err := MigrateCredentials(); err != nil {
		t.Fatalf("MigrateCredentials() error: %v", err)
	}

	// Invalid credential should not be migrated
	data, _ := ReadCredential("badprov", "oauth")
	if data != nil {
		t.Error("invalid JSON should not be migrated")
	}
}

func TestMigrateCredentials_Idempotent(t *testing.T) {
	dir := setupTempDir(t)

	// Create old-style credential
	oldDir := filepath.Join(dir, "data", "credentials")
	claudeDir := filepath.Join(oldDir, "claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "oauth.json"), []byte(`{"token":"x"}`), 0o600)

	// Run migration twice
	if err := MigrateCredentials(); err != nil {
		t.Fatalf("first MigrateCredentials() error: %v", err)
	}
	if err := MigrateCredentials(); err != nil {
		t.Fatalf("second MigrateCredentials() error: %v", err)
	}

	// Credential should still be there
	data, _ := ReadCredential("claude", "oauth")
	if data == nil {
		t.Error("credential should persist after double migration")
	}
}
