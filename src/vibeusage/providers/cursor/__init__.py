"""Cursor provider for vibeusage."""

from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.providers.base import Provider
from vibeusage.providers.base import ProviderMetadata
from vibeusage.providers.cursor.web import CursorBrowserCookieStrategy
from vibeusage.providers.cursor.web import CursorWebStrategy


class CursorProvider(Provider):
    """Provider for Cursor IDE usage."""

    metadata = ProviderMetadata(
        id="cursor",
        name="Cursor",
        description="AI-powered code editor",
        homepage="https://cursor.com",
        status_url="https://status.cursor.com",
        dashboard_url="https://cursor.com/settings/usage",
    )

    def fetch_strategies(self):
        """Return ordered list of fetch strategies for Cursor.

        Priority order:
        1. Web - stored session token from credential storage
        2. Browser - extract session token from browser cookies
        """
        return [
            CursorWebStrategy(),
            CursorBrowserCookieStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        """Fetch Cursor's operational status."""
        from vibeusage.providers.cursor.status import fetch_cursor_status

        return await fetch_cursor_status()
