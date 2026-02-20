package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// FetchStatuspageStatus fetches status from a Statuspage.io endpoint.
func FetchStatuspageStatus(url string) models.ProviderStatus {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	var data struct {
		Status struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	level := indicatorToLevel(data.Status.Indicator)
	now := time.Now().UTC()
	return models.ProviderStatus{
		Level:       level,
		Description: data.Status.Description,
		UpdatedAt:   &now,
	}
}

func indicatorToLevel(indicator string) models.StatusLevel {
	switch strings.ToLower(indicator) {
	case "none":
		return models.StatusOperational
	case "minor":
		return models.StatusDegraded
	case "major":
		return models.StatusPartialOutage
	case "critical":
		return models.StatusMajorOutage
	default:
		return models.StatusUnknown
	}
}
