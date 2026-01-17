"""Provider registry for vibeusage."""

from typing import Protocol

# Provider registry
_PROVIDERS: dict[str, type] = {}


def register_provider(cls: type) -> type:
    """Decorator to register a provider class.

    Usage:
        @register_provider
        class ClaudeProvider(Provider):
            ...
    """
    if not hasattr(cls, "metadata"):
        raise ValueError(f"Provider {cls.__name__} must define metadata ClassVar")

    provider_id = cls.metadata.id
    _PROVIDERS[provider_id] = cls
    return cls


def get_provider(provider_id: str) -> type | None:
    """Get a provider class by ID.

    Returns:
        Provider class or None if not found
    """
    return _PROVIDERS.get(provider_id)


def get_all_providers() -> dict[str, type]:
    """Get all registered providers.

    Returns:
        Dict of provider_id to provider class
    """
    return dict(_PROVIDERS)


def list_provider_ids() -> list[str]:
    """List all registered provider IDs.

    Returns:
        List of provider IDs
    """
    return list(_PROVIDERS.keys())


def create_provider(provider_id: str):
    """Create an instance of a provider.

    Args:
        provider_id: Provider identifier

    Returns:
        Provider instance

    Raises:
        ValueError: If provider not found
    """
    provider_cls = get_provider(provider_id)
    if provider_cls is None:
        raise ValueError(f"Unknown provider: {provider_id}")
    return provider_cls()


# Import and register providers
from vibeusage.providers.base import Provider, ProviderMetadata
from vibeusage.providers.claude import ClaudeProvider
from vibeusage.providers.codex import CodexProvider

# Register providers
register_provider(ClaudeProvider)
register_provider(CodexProvider)

__all__ = [
    "Provider",
    "ProviderMetadata",
    "register_provider",
    "get_provider",
    "get_all_providers",
    "list_provider_ids",
    "create_provider",
]
