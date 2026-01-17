"""Platform-specific paths for vibeusage configuration and cache."""

from __future__ import annotations

import os
from pathlib import Path

from platformdirs import user_cache_dir
from platformdirs import user_config_dir
from platformdirs import user_state_path

PACKAGE_NAME = "vibeusage"


def _get_env_path(env_var: str, fallback: Path) -> Path:
    """Get path from environment variable or fallback."""
    if env_value := os.environ.get(env_var):
        return Path(env_value)
    return fallback


def config_dir() -> Path:
    """Get user config directory.

    Respects VIBEUSAGE_CONFIG_DIR environment variable.
    """
    base_dir = Path(user_config_dir(PACKAGE_NAME))
    return _get_env_path("VIBEUSAGE_CONFIG_DIR", base_dir)


def cache_dir() -> Path:
    """Get user cache directory.

    Respects VIBEUSAGE_CACHE_DIR environment variable.
    """
    base_dir = Path(user_cache_dir(PACKAGE_NAME))
    return _get_env_path("VIBEUSAGE_CACHE_DIR", base_dir)


def state_dir() -> Path:
    """Get user state directory for runtime data."""
    return Path(user_state_path(PACKAGE_NAME))


def credentials_dir() -> Path:
    """Get credentials subdirectory."""
    return config_dir() / "credentials"


def snapshots_dir() -> Path:
    """Get cached snapshots directory."""
    return cache_dir() / "snapshots"


def org_ids_dir() -> Path:
    """Get cached org IDs directory."""
    return cache_dir() / "org-ids"


def gate_dir() -> Path:
    """Get failure gate state directory."""
    return state_dir() / "gates"


def config_file() -> Path:
    """Get main config.toml path."""
    return config_dir() / "config.toml"


def ensure_directories() -> None:
    """Ensure all required directories exist."""
    for directory in (
        config_dir(),
        cache_dir(),
        state_dir(),
        credentials_dir(),
        snapshots_dir(),
        org_ids_dir(),
        gate_dir(),
    ):
        directory.mkdir(parents=True, exist_ok=True)
