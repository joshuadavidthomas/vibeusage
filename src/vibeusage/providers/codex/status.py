"""Status fetching for Codex (OpenAI) provider."""

from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.providers.claude.status import fetch_statuspage_status


async def fetch_codex_status() -> ProviderStatus:
    """Fetch Codex's operational status from OpenAI status page.

    OpenAI uses Statuspage.io at status.openai.com.
    """
    return await fetch_statuspage_status("https://status.openai.com/api/v2/status.json")
