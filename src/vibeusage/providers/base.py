"""Base provider protocol and metadata for vibeusage."""

from abc import ABC, abstractmethod
from typing import ClassVar

from msgspec import Struct
from vibeusage.models import ProviderStatus, StatusLevel, UsageSnapshot
from vibeusage.strategies.base import FetchStrategy


class ProviderMetadata(Struct, frozen=True):
    """Metadata about a provider."""

    id: str
    name: str
    description: str
    homepage: str
    status_url: str | None = None
    dashboard_url: str | None = None


class Provider(ABC):
    """Abstract base class for all providers.

    Each provider must:
    1. Define metadata as a ClassVar
    2. Implement fetch_strategies() to return ordered list of strategies
    3. Implement fetch_status() to get provider health status
    """

    # Subclasses must define this
    metadata: ClassVar[ProviderMetadata]

    @property
    def id(self) -> str:
        """Get provider ID."""
        return self.metadata.id

    @property
    def name(self) -> str:
        """Get provider name."""
        return self.metadata.name

    @abstractmethod
    def fetch_strategies(self) -> list[FetchStrategy]:
        """Return ordered list of fetch strategies to try.

        Strategies are tried in order until one succeeds.
        """

    async def fetch_status(self) -> ProviderStatus:
        """Fetch the current operational status of this provider.

        Returns:
            ProviderStatus with current status level

        Default implementation returns StatusLevel.unknown.
        Subclasses can override to query actual status pages.
        """
        return ProviderStatus.unknown()

    def is_enabled(self) -> bool:
        """Check if this provider is enabled in configuration.

        Returns:
            True if provider should be fetched
        """
        from vibeusage.config.settings import get_config

        config = get_config()
        return config.is_provider_enabled(self.id)
