"""Status fetching for Copilot (GitHub) provider."""
from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.providers.claude.status import fetch_statuspage_status


async def fetch_copilot_status() -> ProviderStatus:
    """Fetch Copilot's operational status from GitHub status page.

    GitHub (which hosts Copilot) uses Statuspage.io at www.githubstatus.com.
    """
    return await fetch_statuspage_status(
        "https://www.githubstatus.com/api/v2/status.json"
    )
