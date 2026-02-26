package provider

import (
	"context"
	"encoding/xml"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// statuspageAPIPath is the standard Statuspage.io API path for current status.
const statuspageAPIPath = "/api/v2/status.json"

// FetchStatuspageStatus fetches status from a Statuspage.io base URL.
// Pass the base status page URL (e.g., "https://status.anthropic.com"),
// and the function will append the standard API path.
func FetchStatuspageStatus(ctx context.Context, baseURL string) models.ProviderStatus {
	url := strings.TrimSuffix(baseURL, "/") + statuspageAPIPath
	return fetchStatuspageStatusFromURL(ctx, url)
}

// fetchStatuspageStatusFromURL is the testable core of FetchStatuspageStatus.
func fetchStatuspageStatusFromURL(ctx context.Context, url string) models.ProviderStatus {
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

// onlineOrNotRSSPath is the standard OnlineOrNot RSS feed path.
const onlineOrNotRSSPath = "/incidents.rss"

// rssItem represents a single item in an RSS feed.
type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// rssChannel represents the channel element in an RSS feed.
type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssFeed represents the root element of an RSS feed.
type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

// FetchOnlineOrNotStatus fetches status from an OnlineOrNot base URL.
// Pass the base status page URL (e.g., "https://status.openrouter.ai"),
// and the function will append the standard RSS feed path.
func FetchOnlineOrNotStatus(ctx context.Context, baseURL string) models.ProviderStatus {
	rssURL := strings.TrimSuffix(baseURL, "/") + onlineOrNotRSSPath
	return fetchOnlineOrNotStatusFromURL(ctx, rssURL)
}

// fetchOnlineOrNotStatusFromURL is the testable core of FetchOnlineOrNotStatus.
func fetchOnlineOrNotStatusFromURL(ctx context.Context, rssURL string) models.ProviderStatus {
	client := httpclient.NewWithTimeout(10 * time.Second)

	resp, err := client.DoCtx(ctx, "GET", rssURL, nil)
	if err != nil || resp.StatusCode != 200 {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	var feed rssFeed
	if err := xml.Unmarshal(resp.Body, &feed); err != nil {
		return models.ProviderStatus{Level: models.StatusUnknown}
	}

	return parseOnlineOrNotFeed(feed)
}

// parseOnlineOrNotFeed parses an OnlineOrNot RSS feed and returns a ProviderStatus.
// Exported for testing.
func parseOnlineOrNotFeed(feed rssFeed) models.ProviderStatus {
	now := time.Now().UTC()

	// If no items, assume operational
	if len(feed.Channel.Items) == 0 {
		return models.ProviderStatus{
			Level:       models.StatusOperational,
			Description: "All systems operational",
			UpdatedAt:   &now,
		}
	}

	// Check the most recent incidents (within last 24 hours)
	// If any are unresolved, report degraded status
	const lookbackWindow = 24 * time.Hour
	cutoff := now.Add(-lookbackWindow)

	var unresolved []rssItem
	for _, item := range feed.Channel.Items {
		pubDate, _ := parseRSSDate(item.PubDate)
		if pubDate.Before(cutoff) {
			continue // Skip old incidents
		}

		// Check if resolved - RSS descriptions contain "RESOLVED" when done
		descLower := strings.ToLower(item.Description)
		if !strings.Contains(descLower, "resolved") {
			unresolved = append(unresolved, item)
		}
	}

	if len(unresolved) > 0 {
		return models.ProviderStatus{
			Level:       models.StatusDegraded,
			Description: unresolved[0].Title,
			UpdatedAt:   &now,
		}
	}

	return models.ProviderStatus{
		Level:       models.StatusOperational,
		Description: "All systems operational",
		UpdatedAt:   &now,
	}
}

// parseRSSDate parses RSS pubDate format (RFC1123).
func parseRSSDate(s string) (time.Time, error) {
	return time.Parse(time.RFC1123, s)
}
