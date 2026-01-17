"""Credential file management for vibeusage."""

from __future__ import annotations

import os
import stat
from pathlib import Path

from vibeusage.config.paths import credentials_dir

# Provider CLI credential locations
PROVIDER_CREDENTIAL_PATHS: dict[str, str | list[str]] = {
    "claude": "~/.claude/.credentials.json",
    "codex": "~/.codex/auth.json",
    "gemini": "~/.gemini/oauth_creds.json",
    "copilot": "~/.config/github-copilot/hosts.json",
    "cursor": "~/.cursor/mcp-state.json",
}


def credential_path(provider_id: str, credential_type: str) -> Path:
    """Get the path for a provider's credential file."""
    return credentials_dir() / provider_id / f"{credential_type}.json"


def _expand_path(path: str) -> Path:
    """Expand user path and return Path object."""
    return Path(path).expanduser()


def find_provider_credential(provider_id: str) -> tuple[bool, str | None, Path | None]:
    """Check for provider credentials in vibeusage and provider CLIs.

    Returns:
        (found, source, path) - source is 'vibeusage', 'provider_cli', or None
    """
    from .settings import get_config

    config = get_config()
    config.get_provider_config(provider_id)

    # Check vibeusage storage first
    vibeusage_paths = [
        credential_path(provider_id, "oauth"),
        credential_path(provider_id, "session"),
        credential_path(provider_id, "apikey"),
    ]

    for cred_path in vibeusage_paths:
        if cred_path.exists():
            return True, "vibeusage", cred_path

    # Check provider CLI credentials if reuse is enabled
    if (
        config.credentials.reuse_provider_credentials
        and provider_id in PROVIDER_CREDENTIAL_PATHS
    ):
        provider_paths = PROVIDER_CREDENTIAL_PATHS[provider_id]
        if isinstance(provider_paths, str):
            provider_paths = [provider_paths]

        for provider_path in provider_paths:
            path = _expand_path(provider_path)
            if path.exists():
                return True, "provider_cli", path

    # Check environment variables for API keys
    env_var_names = {
        "claude": "ANTHROPIC_API_KEY",
        "codex": "OPENAI_API_KEY",
        "gemini": "GEMINI_API_KEY",
        "copilot": "GITHUB_TOKEN",
        "cursor": "CURSOR_API_KEY",
    }

    if env_var := env_var_names.get(provider_id):
        if os.environ.get(env_var):
            return True, "env", None

    return False, None, None


def write_credential(path: Path, content: bytes) -> None:
    """Securely write credential to file with 0o600 permissions."""
    path.parent.mkdir(parents=True, exist_ok=True)

    # Write to temp file first, then rename for atomicity
    temp_path = path.with_suffix(".tmp")
    temp_path.write_bytes(content)

    # Set restrictive permissions
    temp_path.chmod(stat.S_IRUSR | stat.S_IWUSR)  # 0o600

    # Atomic rename
    temp_path.replace(path)


def read_credential(path: Path) -> bytes | None:
    """Read credential file if it exists and has secure permissions."""
    if not path.exists():
        return None

    # Check permissions
    mode = path.stat().st_mode
    if mode & (stat.S_IRGRP | stat.S_IWGRP | stat.S_IROTH | stat.S_IWOTH):
        # File is readable/writable by group or others - unsafe
        return None

    return path.read_bytes()


def delete_credential(path: Path) -> bool:
    """Delete credential file.

    Returns:
        True if deleted, False if didn't exist
    """
    if not path.exists():
        return False

    path.unlink()
    return True


def check_credential_permissions(path: Path) -> bool:
    """Verify credential file has secure permissions (0o600 or stricter)."""
    if not path.exists():
        return True  # No file is secure

    mode = path.stat().st_mode
    # Check that only owner has read/write
    return not (mode & (stat.S_IRGRP | stat.S_IWGRP | stat.S_IROTH | stat.S_IWOTH))


def check_provider_credentials(provider_id: str) -> tuple[bool, str | None]:
    """Check if provider has valid credentials.

    Returns:
        (has_credentials, source) - source is 'vibeusage', 'provider_cli', 'env', or None
    """
    found, source, _ = find_provider_credential(provider_id)
    return found, source


def get_all_credential_status() -> dict[str, dict]:
    """Get credential status for all known providers."""
    status = {}
    for provider_id in PROVIDER_CREDENTIAL_PATHS:
        has_creds, source = check_provider_credentials(provider_id)
        status[provider_id] = {
            "has_credentials": has_creds,
            "source": source,
        }
    return status
