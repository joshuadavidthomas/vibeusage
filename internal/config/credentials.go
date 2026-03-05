package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CredentialsFile returns the path to the consolidated credentials file.
func CredentialsFile() string {
	return filepath.Join(DataDir(), "credentials.json")
}

// credentialsMu protects reads and writes to the credentials file.
var credentialsMu sync.Mutex

// credentialsStore is the in-memory representation of the credentials file:
// map[providerID]map[credType]json.RawMessage
type credentialsStore map[string]map[string]json.RawMessage

func loadCredentialsStore() (credentialsStore, error) {
	path := CredentialsFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(credentialsStore), nil
		}
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}
	var store credentialsStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing credentials file: %w", err)
	}
	if store == nil {
		store = make(credentialsStore)
	}
	return store, nil
}

func saveCredentialsStore(store credentialsStore) error {
	path := CredentialsFile()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

func expandPath(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// FindProviderCredential checks for credentials in vibeusage storage,
// provider CLI paths, and environment variables.
// cliPaths are external CLI credential file paths (may contain ~ for home).
// envVars are environment variable names to check.
// Returns (found, source, providerID+credType or path).
func FindProviderCredential(providerID string, cliPaths []string, envVars []string) (bool, string, string) {
	// Check vibeusage storage first
	for _, credType := range []string{"oauth", "session", "apikey", "api_key"} {
		data, _ := ReadCredential(providerID, credType)
		if data != nil {
			return true, "vibeusage", providerID + "/" + credType
		}
	}

	// Check provider CLI credentials
	for _, raw := range cliPaths {
		p := expandPath(raw)
		if fileExists(p) {
			return true, "provider_cli", p
		}
	}

	// Check environment variables
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			return true, "env", ""
		}
	}

	return false, "", ""
}

// WriteCredential writes credential content for a provider and credential type
// into the consolidated credentials file.
func WriteCredential(providerID, credType string, content []byte) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	store, err := loadCredentialsStore()
	if err != nil {
		return err
	}

	if store[providerID] == nil {
		store[providerID] = make(map[string]json.RawMessage)
	}
	store[providerID][credType] = json.RawMessage(content)

	return saveCredentialsStore(store)
}

// ReadCredential reads credential content for a provider and credential type
// from the consolidated credentials file. Returns (nil, nil) if not found.
func ReadCredential(providerID, credType string) ([]byte, error) {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	store, err := loadCredentialsStore()
	if err != nil {
		return nil, err
	}

	providerStore, ok := store[providerID]
	if !ok {
		return nil, nil
	}

	data, ok := providerStore[credType]
	if !ok {
		return nil, nil
	}

	return []byte(data), nil
}

// DeleteCredential removes a credential for a provider and credential type.
// Returns true if the credential existed and was removed.
func DeleteCredential(providerID, credType string) bool {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	store, err := loadCredentialsStore()
	if err != nil {
		return false
	}

	providerStore, ok := store[providerID]
	if !ok {
		return false
	}

	if _, ok := providerStore[credType]; !ok {
		return false
	}

	delete(providerStore, credType)
	if len(providerStore) == 0 {
		delete(store, providerID)
	}

	return saveCredentialsStore(store) == nil
}

// DeleteProviderCredentials removes all credentials for a provider.
// Returns true if any credentials were removed.
func DeleteProviderCredentials(providerID string) bool {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	store, err := loadCredentialsStore()
	if err != nil {
		return false
	}

	if _, ok := store[providerID]; !ok {
		return false
	}

	delete(store, providerID)
	return saveCredentialsStore(store) == nil
}

// HasCredential reports whether a credential exists for a provider and credential type.
func HasCredential(providerID, credType string) bool {
	data, _ := ReadCredential(providerID, credType)
	return data != nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
