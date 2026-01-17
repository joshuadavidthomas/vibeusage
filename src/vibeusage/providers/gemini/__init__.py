"""Gemini (Google AI) provider for vibeusage."""
from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.models import StatusLevel
from vibeusage.providers.base import Provider
from vibeusage.providers.base import ProviderMetadata
from vibeusage.providers.gemini.api_key import GeminiApiKeyStrategy
from vibeusage.providers.gemini.oauth import GeminiOAuthStrategy


class GeminiProvider(Provider):
    """Provider for Gemini (Google AI) usage."""

    metadata = ProviderMetadata(
        id="gemini",
        name="Gemini",
        description="Google Gemini AI",
        homepage="https://gemini.google.com",
        status_url=None,  # Uses Google Workspace status
        dashboard_url="https://aistudio.google.com/app/usage",
    )

    def fetch_strategies(self):
        """Return ordered list of fetch strategies for Gemini.

        Priority order:
        1. OAuth - Google Cloud Code API via OAuth tokens (provides real usage)
        2. API Key - Google AI API key (provides model info, no usage data)
        """
        return [
            GeminiOAuthStrategy(),
            GeminiApiKeyStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        """Fetch Gemini's operational status from Google Workspace incidents feed."""
        from vibeusage.providers.gemini.status import fetch_gemini_status

        return await fetch_gemini_status()
