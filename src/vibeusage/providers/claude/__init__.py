"""Claude (Anthropic) provider for vibeusage."""

from __future__ import annotations

from vibeusage.models import ProviderStatus
from vibeusage.providers.base import Provider
from vibeusage.providers.base import ProviderMetadata
from vibeusage.providers.claude.cli import ClaudeCLIStrategy
from vibeusage.providers.claude.oauth import ClaudeOAuthStrategy
from vibeusage.providers.claude.web import ClaudeBrowserCookieStrategy
from vibeusage.providers.claude.web import ClaudeWebStrategy


class ClaudeProvider(Provider):
    """Provider for Claude (Anthropic) usage."""

    metadata = ProviderMetadata(
        id="claude",
        name="Claude",
        description="Anthropic's Claude AI assistant",
        homepage="https://claude.ai",
        status_url="https://status.anthropic.com",
        dashboard_url="https://claude.ai/settings/usage",
    )

    def fetch_strategies(self):
        """Return ordered list of fetch strategies for Claude.

        Priority order:
        1. OAuth - stored tokens with refresh capability
        2. Web - stored session key from credential storage
        3. Browser - extract session key from browser cookies
        4. CLI - delegate to claude CLI tool
        """
        return [
            ClaudeOAuthStrategy(),
            ClaudeWebStrategy(),
            ClaudeBrowserCookieStrategy(),
            ClaudeCLIStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        """Fetch Claude's operational status from Statuspage.io."""
        from vibeusage.providers.claude.status import fetch_statuspage_status

        return await fetch_statuspage_status(
            "https://8kmfgt14pf99.statuspage.io/api/v2/status.json"
        )
