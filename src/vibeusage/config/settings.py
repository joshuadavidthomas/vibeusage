"""Configuration structures and loading for vibeusage."""

import os
from pathlib import Path
from typing import Literal

import msgspec


# Default values
DEFAULT_TIMEOUT = 30.0
DEFAULT_MAX_CONCURRENT = 5
DEFAULT_STALE_THRESHOLD_MINUTES = 60


# Display configuration
class DisplayConfig(msgspec.Struct, omit_defaults=True):
    """Display settings."""

    show_remaining: bool = True
    pace_colors: bool = True
    reset_format: Literal["countdown", "absolute"] = "countdown"


# Fetch configuration
class FetchConfig(msgspec.Struct, omit_defaults=True):
    """Fetch behavior settings."""

    timeout: float = DEFAULT_TIMEOUT
    max_concurrent: int = DEFAULT_MAX_CONCURRENT
    stale_threshold_minutes: int = DEFAULT_STALE_THRESHOLD_MINUTES


# Credentials configuration
class CredentialsConfig(msgspec.Struct, omit_defaults=True):
    """Credential management settings."""

    use_keyring: bool = False
    reuse_provider_credentials: bool = True


# Per-provider configuration
class ProviderConfig(msgspec.Struct, omit_defaults=True):
    """Configuration for a specific provider."""

    auth_source: Literal["auto", "oauth", "web", "cli", "apikey", "manual"] = "auto"
    preferred_browser: (
        Literal["safari", "chrome", "firefox", "arc", "brave", "edge", "chromium"]
        | None
    ) = None
    enabled: bool = True


# Main configuration
class Config(msgspec.Struct, omit_defaults=True):
    """Main configuration structure."""

    enabled_providers: list[str] = []
    display: DisplayConfig = msgspec.field(default_factory=DisplayConfig)
    fetch: FetchConfig = msgspec.field(default_factory=FetchConfig)
    credentials: CredentialsConfig = msgspec.field(default_factory=CredentialsConfig)
    providers: dict[str, ProviderConfig] = msgspec.field(default_factory=dict)

    def get_provider_config(self, provider_id: str) -> ProviderConfig:
        """Get config for a provider, with defaults."""
        return self.providers.get(provider_id, ProviderConfig())

    def is_provider_enabled(self, provider_id: str) -> bool:
        """Check if a provider is enabled.

        A provider is enabled if:
        1. It's not explicitly disabled in providers config
        2. It's either in enabled_providers list OR enabled_providers is empty (all enabled)
        """
        provider_cfg = self.get_provider_config(provider_id)
        if not provider_cfg.enabled:
            return False
        if not self.enabled_providers:
            return True
        return provider_id in self.enabled_providers


def _load_from_toml(path: Path) -> dict:
    """Load configuration from TOML file."""
    try:
        import tomli
    except ImportError:
        import tomllib as tomli

    if not path.exists():
        return {}

    with path.open("rb") as f:
        return tomli.load(f)


def _save_to_toml(data: dict, path: Path) -> None:
    """Save configuration to TOML file."""
    import tomli_w

    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("wb") as f:
        tomli_w.dump(data, f)


def convert_config(data: dict) -> Config:
    """Convert raw dict to Config struct."""
    converter = msgspec.convert
    return converter(data, type=Config)


def _apply_env_overrides(config: Config) -> Config:
    """Apply environment variable overrides to config.

    VIBEUSAGE_ENABLED_PROVIDERS: Comma-separated list of providers
    VIBEUSAGE_NO_COLOR: Disable colored output
    """
    if "VIBEUSAGE_ENABLED_PROVIDERS" in os.environ:
        providers_str = os.environ["VIBEUSAGE_ENABLED_PROVIDERS"]
        enabled = [p.strip() for p in providers_str.split(",") if p.strip()]
        config = msgspec.structs.replace(config, enabled_providers=enabled)

    if "VIBEUSAGE_NO_COLOR" in os.environ:
        display = msgspec.structs.replace(config.display, pace_colors=False)
        config = msgspec.structs.replace(config, display=display)

    return config


# Config state storage
_config: Config | None = None


def get_config() -> Config:
    """Get the current configuration (singleton)."""
    global _config
    if _config is None:
        _config = load_config()
    return _config


def reload_config() -> Config:
    """Reload configuration from disk."""
    global _config
    _config = load_config()
    return _config


def load_config(path: Path | None = None) -> Config:
    """Load configuration from file with defaults."""
    from .paths import config_file

    config_path = path or config_file()

    raw_data = _load_from_toml(config_path)
    if not raw_data:
        # Return default config
        config = Config()
    else:
        config = convert_config(raw_data)

    # Apply environment variable overrides
    config = _apply_env_overrides(config)

    return config


def save_config(config: Config, path: Path | None = None) -> None:
    """Save configuration to file."""
    from .paths import config_file

    config_path = path or config_file()

    # Convert to dict for TOML serialization
    data = msgspec.to_builtins(config)

    # Remove None values
    def clean_none(d: dict) -> dict:
        return {
            k: clean_none(v) if isinstance(v, dict) else v
            for k, v in d.items()
            if v is not None
        }

    data = clean_none(data)

    _save_to_toml(data, config_path)

    # Update singleton
    global _config
    _config = config
