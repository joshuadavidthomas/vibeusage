package amp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
)

// CLISecretsStrategy loads Amp credentials from the local secrets file.
type CLISecretsStrategy struct {
	HTTPTimeout float64
}

func (s *CLISecretsStrategy) IsAvailable() bool {
	_, ok := loadCLISecretsToken()
	return ok
}

func (s *CLISecretsStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	token, ok := loadCLISecretsToken()
	if !ok {
		return fetch.ResultFail("No Amp CLI credentials found in ~/.local/share/amp/secrets.json"), nil
	}
	return fetchBalance(ctx, token, "provider_cli", s.HTTPTimeout)
}

func loadCLISecretsToken() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	path := filepath.Join(home, ".local", "share", "amp", "secrets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return "", false
	}
	token := strings.TrimSpace(secrets["apiKey@https://ampcode.com/"])
	if token == "" {
		return "", false
	}
	return token, true
}
