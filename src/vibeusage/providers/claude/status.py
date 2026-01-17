"""Status page fetching utilities for providers."""
from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime

from vibeusage.core.http import fetch_url
from vibeusage.models import ProviderStatus
from vibeusage.models import StatusLevel


async def fetch_statuspage_status(url: str) -> ProviderStatus:
    """Fetch status from a Statuspage.io status endpoint.

    Args:
        url: Statuspage.io API status URL

    Returns:
        ProviderStatus with current status
    """
    content = await fetch_url(url)
    if not content:
        return ProviderStatus.unknown()

    try:
        data = json.loads(content)
    except json.JSONDecodeError:
        return ProviderStatus.unknown()

    # Statuspage.io indicator mapping
    # "none" = operational, "minor" = degraded, "major" = partial_outage, "critical" = major_outage
    indicator = data.get("status", {}).get("indicator")

    level = _indicator_to_level(indicator)

    # Get description
    description = data.get("status", {}).get("description")

    return ProviderStatus(
        level=level,
        description=description,
        updated_at=datetime.now(UTC),
    )


def _indicator_to_level(indicator: str | None) -> StatusLevel:
    """Convert Statuspage.io indicator to StatusLevel."""
    mapping = {
        "none": StatusLevel.OPERATIONAL,
        "minor": StatusLevel.DEGRADED,
        "major": StatusLevel.PARTIAL_OUTAGE,
        "critical": StatusLevel.MAJOR_OUTAGE,
    }

    return mapping.get(indicator, StatusLevel.UNKNOWN)
