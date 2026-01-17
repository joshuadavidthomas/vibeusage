"""Status page fetching for Gemini provider."""

import json
from datetime import datetime, timezone

from vibeusage.core.http import fetch_url
from vibeusage.models import ProviderStatus, StatusLevel


async def fetch_gemini_status() -> ProviderStatus:
    """Fetch Gemini's operational status from Google Workspace incidents feed.

    Google doesn't provide a dedicated status endpoint for Gemini.
    This checks the Google Workspace incidents feed and filters for
    Gemini-related issues.

    Returns:
        ProviderStatus with current status
    """
    url = "https://www.google.com/appsstatus/dashboard/incidents.json"

    content = await fetch_url(url)
    if not content:
        return ProviderStatus.unknown()

    try:
        data = json.loads(content)
    except json.JSONDecodeError:
        return ProviderStatus.unknown()

    # Google Apps Status Dashboard format
    # Check for incidents affecting Gemini or AI-related services
    incidents = data if isinstance(data, list) else data.get("incidents", [])

    # Look for active Gemini-related incidents
    gemini_incidents = []
    for incident in incidents:
        # Check if incident is still active (no end_time or end_time is recent)
        end_time = incident.get("end_time")
        if end_time:
            # Incident has ended
            continue

        # Check if incident affects Gemini or AI services
        title = incident.get("title", "").lower()
        services = incident.get("affected_services", [])
        service_names = " ".join(
            [s.get("name", "").lower() for s in services if isinstance(s, dict)]
        )

        # Keywords that indicate Gemini might be affected
        gemini_keywords = [
            "gemini",
            "ai studio",
            "aistudio",
            "generative ai",
            "vertex ai",
            "cloud code",
        ]

        if any(
            keyword in title or keyword in service_names for keyword in gemini_keywords
        ):
            gemini_incidents.append(incident)

    # Determine status level based on incidents
    if not gemini_incidents:
        return ProviderStatus(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=datetime.now(timezone.utc),
        )

    # Check severity of most recent incident
    latest_incident = gemini_incidents[0]
    severity = latest_incident.get("severity", "unknown").lower()

    level = _severity_to_level(severity)

    description = latest_incident.get("title", "Service issue reported")

    return ProviderStatus(
        level=level,
        description=description,
        updated_at=datetime.now(timezone.utc),
    )


def _severity_to_level(severity: str) -> StatusLevel:
    """Convert Google incident severity to StatusLevel."""
    mapping = {
        "low": StatusLevel.DEGRADED,
        "medium": StatusLevel.DEGRADED,
        "high": StatusLevel.PARTIAL_OUTAGE,
        "critical": StatusLevel.MAJOR_OUTAGE,
        "severe": StatusLevel.MAJOR_OUTAGE,
    }

    return mapping.get(severity, StatusLevel.DEGRADED)
