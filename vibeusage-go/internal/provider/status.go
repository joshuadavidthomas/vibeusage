package provider

import (
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// FetchStatuspageStatus fetches status from a Statuspage.io endpoint.
func FetchStatuspageStatus(url string) models.ProviderStatus {
	client := httpclient.NewWithTimeout(10 * time.Second)
	var data struct {
		Status struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		} `json:"status"`
	}
	resp, err := client.GetJSON(url, &data)
	if err != nil || resp.JSONErr != nil {
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
