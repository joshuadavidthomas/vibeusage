package modelmap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

const (
	modelsDevURL = "https://models.dev/api.json"
	cacheTTL     = 24 * time.Hour
)

// modelsDevProvider is the top-level entry for a provider in the models.dev API.
type modelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Models map[string]modelsDevModel `json:"models"`
}

// modelsDevModel is a single model entry from models.dev.
type modelsDevModel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Family string `json:"family"`
}

// loadModelsDevData returns the parsed models.dev data, using the cache when
// fresh and fetching from the network otherwise. Returns nil on any failure.
func loadModelsDevData() map[string]modelsDevProvider {
	path := config.ModelsFile()

	if data := readCacheIfFresh(path); data != nil {
		return data
	}

	raw, err := fetchModelsDevAPI()
	if err != nil {
		// Network failed — serve stale cache if it exists.
		if data := readCache(path); data != nil {
			return data
		}
		return nil
	}

	_ = writeCache(path, raw)
	return parseModelsDevJSON(raw)
}

func readCacheIfFresh(path string) map[string]modelsDevProvider {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil
	}
	return readCache(path)
}

func readCache(path string) map[string]modelsDevProvider {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseModelsDevJSON(raw)
}

func writeCache(path string, data []byte) error {
	if err := os.MkdirAll(config.CacheDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fetchModelsDevAPI() ([]byte, error) {
	client := httpclient.NewWithTimeout(15 * time.Second)
	resp, err := client.DoCtx(context.Background(), "GET", modelsDevURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching models.dev: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching models.dev: HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func parseModelsDevJSON(raw []byte) map[string]modelsDevProvider {
	var data map[string]modelsDevProvider
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}
	return data
}
