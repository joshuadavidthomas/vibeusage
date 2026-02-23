package provider

import (
	"context"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// FetchStatuspageStatus fetches status from a Statuspage.io endpoint.
func FetchStatuspageStatus(ctx context.Context, url string) models.ProviderStatus {
	client := httpclient.NewWithTimeout(10 * time.Second)
	var data struct {
		Status struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		} `json:"status"`
	}
	resp, err := client.GetJSONCtx(ctx, url, &data)
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

const googleIncidentURL = "https://www.google.com/appsstatus/dashboard/incidents.json"

// FetchGoogleAppsStatus checks the Google Apps Status Dashboard for active
// incidents matching any of the given keywords. Used by providers that run
// on Google infrastructure (Gemini, Antigravity).
func FetchGoogleAppsStatus(ctx context.Context, keywords []string) models.ProviderStatus {
	return fetchGoogleAppsStatusFromURL(ctx, googleIncidentURL, keywords)
}

// fetchGoogleAppsStatusFromURL is the testable core of FetchGoogleAppsStatus,
// accepting the incident URL as a parameter.
func fetchGoogleAppsStatusFromURL(ctx context.Context, incidentURL string, keywords []string) models.ProviderStatus {
	client := httpclient.NewWithTimeout(10 * time.Second)
	var incidents []googleIncident
	resp, err := client.GetJSONCtx(ctx, incidentURL, &incidents)
	if err != nil || resp.JSONErr != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	for _, incident := range incidents {
		if incident.EndTime != "" {
			continue // resolved
		}
		titleLower := strings.ToLower(incident.Title)

		for _, keyword := range keywords {
			if strings.Contains(titleLower, keyword) {
				level := googleSeverityToLevel(incident.Severity)
				now := time.Now().UTC()
				return models.ProviderStatus{
					Level:       level,
					Description: incident.Title,
					UpdatedAt:   &now,
				}
			}
		}
	}

	now := time.Now().UTC()
	return models.ProviderStatus{
		Level:       models.StatusOperational,
		Description: "All systems operational",
		UpdatedAt:   &now,
	}
}

// googleIncident represents a single incident from the Google Apps Status API.
type googleIncident struct {
	Title    string `json:"title,omitempty"`
	Severity string `json:"severity,omitempty"`
	EndTime  string `json:"end_time,omitempty"`
}

func googleSeverityToLevel(severity string) models.StatusLevel {
	switch strings.ToLower(severity) {
	case "low", "medium":
		return models.StatusDegraded
	case "high":
		return models.StatusPartialOutage
	case "critical", "severe":
		return models.StatusMajorOutage
	default:
		return models.StatusDegraded
	}
}
