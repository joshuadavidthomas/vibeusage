"""Copilot (GitHub) provider for vibeusage."""

from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.providers.base import Provider
from vibeusage.providers.base import ProviderMetadata
from vibeusage.providers.copilot.device_flow import CopilotDeviceFlowStrategy


class CopilotProvider(Provider):
    """Provider for GitHub Copilot usage."""

    metadata = ProviderMetadata(
        id="copilot",
        name="Copilot",
        description="GitHub's AI pair programmer",
        homepage="https://github.com/features/copilot",
        status_url="https://www.githubstatus.com",
        dashboard_url="https://github.com/settings/copilot",
    )

    def fetch_strategies(self):
        """Return ordered list of fetch strategies for Copilot.

        Priority order:
        1. Device Flow - GitHub OAuth device flow
        """
        return [
            CopilotDeviceFlowStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        """Fetch Copilot's operational status from GitHub status page."""
        from vibeusage.providers.copilot.status import fetch_copilot_status

        return await fetch_copilot_status()
