"""Codex (OpenAI/ChatGPT) provider for vibeusage."""

from vibeusage.models import ProviderStatus
from vibeusage.providers.base import Provider, ProviderMetadata
from vibeusage.providers.codex.oauth import CodexOAuthStrategy


class CodexProvider(Provider):
    """Provider for Codex (OpenAI/ChatGPT) usage."""

    metadata = ProviderMetadata(
        id="codex",
        name="Codex",
        description="OpenAI's ChatGPT and Codex",
        homepage="https://chatgpt.com",
        status_url="https://status.openai.com",
        dashboard_url="https://chatgpt.com/codex/settings/usage",
    )

    def fetch_strategies(self):
        """Return ordered list of fetch strategies for Codex.

        Priority order:
        1. OAuth - stored tokens with refresh capability
        """
        return [
            CodexOAuthStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        """Fetch Codex's operational status from OpenAI status page."""
        from vibeusage.providers.codex.status import fetch_codex_status

        return await fetch_codex_status()
