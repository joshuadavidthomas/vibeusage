"""Status fetching for Cursor provider."""

from vibeusage.models import ProviderStatus
from vibeusage.providers.claude.status import fetch_statuspage_status


async def fetch_cursor_status() -> ProviderStatus:
    """Fetch Cursor's operational status from status page.

    Cursor uses Statuspage.io at status.cursor.com.
    """
    return await fetch_statuspage_status("https://status.cursor.com/api/v2/status.json")
