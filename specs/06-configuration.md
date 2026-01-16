# Spec 06: Configuration Management

**Status**: Draft
**Dependencies**: 01-architecture, 03-authentication, 05-cli-interface
**Dependents**: 07-error-handling

## Overview

This specification defines configuration management for vibeusage, including user preferences, credential storage, and cache management. Configuration uses TOML for human-readable settings and JSON for credentials and cached data.

## Design Goals

1. **XDG-Compliant Paths**: Use `platformdirs` for cross-platform config/cache locations
2. **Sensible Defaults**: Work out of box with minimal configuration
3. **Credential Security**: File permissions protect sensitive data
4. **Reuse Provider Credentials**: Detect and use credentials from provider CLIs where possible
5. **Graceful Degradation**: Missing config returns defaults, missing credentials fail cleanly

## Dependencies

```toml
# pyproject.toml
dependencies = [
    "msgspec>=0.18",
    "platformdirs>=4.0",
    "tomli>=2.0;python_version<'3.11'",  # TOML parsing (stdlib in 3.11+)
    "tomli-w>=1.0",                        # TOML writing
]
```

---

## Directory Structure

### Config Directory

User settings and credentials:

```
# Linux
~/.config/vibeusage/

# macOS
~/Library/Application Support/vibeusage/

# Windows
%APPDATA%/vibeusage/
```

**Layout**:

```
~/.config/vibeusage/
├── config.toml                    # User preferences
└── credentials/
    ├── claude/
    │   ├── oauth.json             # OAuth tokens (from Claude CLI or vibeusage)
    │   └── session-key            # Manual session key
    ├── codex/
    │   └── oauth.json
    ├── copilot/
    │   └── oauth.json             # GitHub device flow token
    ├── cursor/
    │   └── session.json           # Stored browser session
    ├── gemini/
    │   └── oauth.json
    ├── augment/
    │   └── session.json
    ├── factory/
    │   └── session.json
    └── zai/
        └── api-key                # API key file
```

### Cache Directory

Temporary data that can be regenerated:

```
# Linux
~/.cache/vibeusage/

# macOS
~/Library/Caches/vibeusage/

# Windows
%LOCALAPPDATA%/vibeusage/Cache/
```

**Layout**:

```
~/.cache/vibeusage/
├── snapshots/                     # Cached usage data for offline display
│   ├── claude.json
│   ├── codex.json
│   └── cursor.json
└── org-ids/                       # Cached organization IDs
    ├── claude
    └── codex
```

### Path Resolution

```python
from pathlib import Path
import platformdirs

APP_NAME = "vibeusage"

def config_dir() -> Path:
    """Return user config directory."""
    return Path(platformdirs.user_config_dir(APP_NAME))

def cache_dir() -> Path:
    """Return user cache directory."""
    return Path(platformdirs.user_cache_dir(APP_NAME))

def credentials_dir() -> Path:
    """Return credentials subdirectory."""
    return config_dir() / "credentials"

def snapshots_dir() -> Path:
    """Return snapshots cache subdirectory."""
    return cache_dir() / "snapshots"

def org_ids_dir() -> Path:
    """Return org-ids cache subdirectory."""
    return cache_dir() / "org-ids"

def config_file() -> Path:
    """Return main config file path."""
    return config_dir() / "config.toml"
```

---

## Configuration File

### Format

The main configuration file uses TOML:

```toml
# ~/.config/vibeusage/config.toml

# Enabled providers (default: all available)
# Comment out or set to empty list to enable all
enabled_providers = ["claude", "codex", "copilot", "cursor"]

[display]
# Show remaining percentage (default) or used percentage
show_remaining = true

# Use pace-based coloring instead of fixed thresholds
pace_colors = true

# Reset time format: "countdown" or "absolute"
reset_format = "countdown"

[fetch]
# Default fetch timeout in seconds
timeout = 30

# Maximum concurrent provider fetches
max_concurrent = 5

# Stale data threshold in minutes (show warning if older)
stale_threshold = 10

[credentials]
# Use system keyring for sensitive credentials (requires keyring package)
use_keyring = false

# Automatically detect and use provider CLI credentials
reuse_provider_credentials = true

[providers.claude]
# Override default auth source: "auto", "oauth", "web", "cli"
auth_source = "auto"

[providers.codex]
auth_source = "auto"

[providers.cursor]
# Browser to prioritize for cookie import
preferred_browser = "auto"  # or "chrome", "firefox", "safari", etc.
```

### Config Model

Config uses msgspec Structs for in-memory representation. TOML is still used for file format (human-readable).

```python
from pathlib import Path
from typing import Literal
import tomllib  # Python 3.11+ or tomli
import tomli_w

import msgspec


class DisplayConfig(msgspec.Struct):
    """Display preferences."""

    show_remaining: bool = True
    pace_colors: bool = True
    reset_format: Literal["countdown", "absolute"] = "countdown"


class FetchConfig(msgspec.Struct):
    """Fetch behavior settings."""

    timeout: int = 30
    max_concurrent: int = 5
    stale_threshold: int = 10


class CredentialsConfig(msgspec.Struct):
    """Credential storage settings."""

    use_keyring: bool = False
    reuse_provider_credentials: bool = True


class ProviderConfig(msgspec.Struct):
    """Per-provider settings."""

    auth_source: Literal["auto", "oauth", "web", "cli"] = "auto"
    preferred_browser: str = "auto"
    enabled: bool = True


class Config(msgspec.Struct):
    """Complete vibeusage configuration."""

    enabled_providers: list[str] | None = None  # None = all available
    display: DisplayConfig = msgspec.field(default_factory=DisplayConfig)
    fetch: FetchConfig = msgspec.field(default_factory=FetchConfig)
    credentials: CredentialsConfig = msgspec.field(default_factory=CredentialsConfig)
    providers: dict[str, ProviderConfig] = msgspec.field(default_factory=dict)

    def get_provider_config(self, provider_id: str) -> ProviderConfig:
        """Get config for a provider, with defaults."""
        return self.providers.get(provider_id, ProviderConfig())

    def is_provider_enabled(self, provider_id: str) -> bool:
        """Check if a provider is enabled."""
        if self.enabled_providers is None:
            return True  # All enabled by default
        return provider_id in self.enabled_providers

    @classmethod
    def load(cls, path: Path | None = None) -> "Config":
        """Load config from file, returning defaults if not found."""
        path = path or config_file()

        if not path.exists():
            return cls()

        try:
            with open(path, "rb") as f:
                data = tomllib.load(f)
            return msgspec.convert(data, cls)
        except Exception:
            # Invalid config - return defaults
            return cls()

    def save(self, path: Path | None = None) -> None:
        """Save config to file."""
        path = path or config_file()
        path.parent.mkdir(parents=True, exist_ok=True)

        # Convert struct to dict for TOML serialization
        data = msgspec.to_builtins(self)

        with open(path, "wb") as f:
            tomli_w.dump(data, f)


# Global config singleton
_config: Config | None = None

def get_config() -> Config:
    """Get or load global config."""
    global _config
    if _config is None:
        _config = Config.load()
    return _config

def reload_config() -> Config:
    """Force reload config from disk."""
    global _config
    _config = Config.load()
    return _config
```

**Note**: `msgspec.convert()` handles conversion from the TOML dict to the Config struct with validation. `msgspec.to_builtins()` converts structs back to dicts for TOML serialization.

---

## Credential Management

### Credential Paths

Each provider has dedicated credential storage:

```python
from pathlib import Path

def credential_path(provider_id: str, credential_type: str) -> Path:
    """
    Get path for a provider credential.

    Args:
        provider_id: Provider identifier (e.g., "claude", "codex")
        credential_type: Type of credential ("oauth", "session-key", "api-key", etc.)

    Returns:
        Path to credential file
    """
    return credentials_dir() / provider_id / credential_type
```

### Provider CLI Credential Locations

vibeusage can reuse credentials from provider CLIs:

```python
from pathlib import Path
import os

# Provider CLI credential locations
PROVIDER_CREDENTIAL_PATHS: dict[str, dict[str, list[Path]]] = {
    "claude": {
        "oauth": [
            Path.home() / ".claude" / ".credentials.json",
            Path(os.environ.get("CLAUDE_CONFIG_DIR", "")) / ".credentials.json",
        ],
    },
    "codex": {
        "oauth": [
            Path.home() / ".codex" / "auth.json",
            Path(os.environ.get("CODEX_HOME", "")) / "auth.json",
        ],
    },
    "gemini": {
        "oauth": [
            Path.home() / ".gemini" / "oauth_creds.json",
        ],
    },
    "vertexai": {
        "oauth": [
            Path.home() / ".config" / "gcloud" / "application_default_credentials.json",
        ],
    },
}


def find_provider_credential(provider_id: str, credential_type: str) -> Path | None:
    """
    Find credential file, checking vibeusage storage first, then provider CLIs.

    Returns:
        Path to credential file if found, None otherwise
    """
    config = get_config()

    # Check vibeusage credential storage first
    vibeusage_path = credential_path(provider_id, credential_type)
    if vibeusage_path.exists():
        return vibeusage_path

    # If reuse enabled, check provider CLI locations
    if config.credentials.reuse_provider_credentials:
        provider_paths = PROVIDER_CREDENTIAL_PATHS.get(provider_id, {})
        for path in provider_paths.get(credential_type, []):
            if path and path.exists():
                return path

    return None
```

### Secure File Operations

```python
import os
import stat

def write_credential(path: Path, content: str, mode: int = 0o600) -> None:
    """
    Write credential file with secure permissions.

    Args:
        path: Destination path
        content: Credential content
        mode: File permissions (default: owner read/write only)
    """
    path.parent.mkdir(parents=True, exist_ok=True)

    # Write to temp file first, then rename (atomic on POSIX)
    temp_path = path.with_suffix(".tmp")
    try:
        with open(temp_path, "w") as f:
            f.write(content)
        os.chmod(temp_path, mode)
        temp_path.rename(path)
    finally:
        # Clean up temp file if rename failed
        if temp_path.exists():
            temp_path.unlink()


def read_credential(path: Path) -> str | None:
    """
    Read credential file, returning None if not found or unreadable.
    """
    try:
        if path.exists():
            return path.read_text().strip()
    except (OSError, IOError):
        pass
    return None


def delete_credential(path: Path) -> bool:
    """
    Delete credential file.

    Returns:
        True if deleted, False if not found
    """
    if path.exists():
        path.unlink()
        return True
    return False


def check_credential_permissions(path: Path) -> list[str]:
    """
    Check if credential file has secure permissions.

    Returns:
        List of warnings (empty if secure)
    """
    warnings = []

    if not path.exists():
        return warnings

    mode = path.stat().st_mode

    # Check for group/other read permissions
    if mode & stat.S_IRGRP:
        warnings.append(f"{path}: group readable")
    if mode & stat.S_IROTH:
        warnings.append(f"{path}: world readable")
    if mode & stat.S_IWGRP:
        warnings.append(f"{path}: group writable")
    if mode & stat.S_IWOTH:
        warnings.append(f"{path}: world writable")

    return warnings
```

### Keyring Integration (Optional)

```python
def use_keyring() -> bool:
    """Check if keyring is enabled and available."""
    config = get_config()
    if not config.credentials.use_keyring:
        return False

    try:
        import keyring
        return True
    except ImportError:
        return False


SERVICE_NAME = "vibeusage"

def keyring_key(provider_id: str, credential_type: str) -> str:
    """Generate keyring key for a credential."""
    return f"{provider_id}:{credential_type}"


def store_in_keyring(provider_id: str, credential_type: str, value: str) -> bool:
    """
    Store credential in system keyring.

    Returns:
        True if stored, False if keyring unavailable
    """
    if not use_keyring():
        return False

    import keyring
    key = keyring_key(provider_id, credential_type)
    keyring.set_password(SERVICE_NAME, key, value)
    return True


def get_from_keyring(provider_id: str, credential_type: str) -> str | None:
    """
    Retrieve credential from system keyring.

    Returns:
        Credential value or None if not found/unavailable
    """
    if not use_keyring():
        return None

    import keyring
    key = keyring_key(provider_id, credential_type)
    return keyring.get_password(SERVICE_NAME, key)


def delete_from_keyring(provider_id: str, credential_type: str) -> bool:
    """
    Delete credential from system keyring.

    Returns:
        True if deleted, False if not found/unavailable
    """
    if not use_keyring():
        return False

    import keyring
    key = keyring_key(provider_id, credential_type)
    try:
        keyring.delete_password(SERVICE_NAME, key)
        return True
    except keyring.errors.PasswordDeleteError:
        return False
```

---

## Cache Management

### Snapshot Cache

Cache usage snapshots for offline display and fast startup. Uses msgspec for fast JSON serialization.

```python
import msgspec
from datetime import datetime
from pathlib import Path
from vibeusage.models import UsageSnapshot

def snapshot_path(provider_id: str) -> Path:
    """Get path for cached snapshot."""
    return snapshots_dir() / f"{provider_id}.json"


def cache_snapshot(snapshot: UsageSnapshot) -> None:
    """
    Cache a usage snapshot using msgspec.
    """
    path = snapshot_path(snapshot.provider)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(msgspec.json.encode(snapshot))


def load_cached_snapshot(provider_id: str) -> UsageSnapshot | None:
    """
    Load cached snapshot for a provider.

    Returns:
        Cached snapshot or None if not found/invalid
    """
    path = snapshot_path(provider_id)
    if not path.exists():
        return None

    try:
        return msgspec.json.decode(path.read_bytes(), type=UsageSnapshot)
    except (FileNotFoundError, msgspec.ValidationError):
        return None


def is_snapshot_fresh(snapshot: UsageSnapshot, max_age_minutes: int | None = None) -> bool:
    """
    Check if snapshot is fresh enough to use.

    Args:
        snapshot: The snapshot to check
        max_age_minutes: Maximum age in minutes (uses config default if None)

    Returns:
        True if snapshot is fresh
    """
    if max_age_minutes is None:
        max_age_minutes = get_config().fetch.stale_threshold

    return not snapshot.is_stale(max_age_minutes)
```

### Organization ID Cache

Cache organization IDs to avoid repeated API calls:

```python
def org_id_path(provider_id: str) -> Path:
    """Get path for cached org ID."""
    return org_ids_dir() / provider_id


def cache_org_id(provider_id: str, org_id: str) -> None:
    """Cache an organization ID."""
    path = org_id_path(provider_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(org_id)


def load_cached_org_id(provider_id: str) -> str | None:
    """Load cached organization ID."""
    path = org_id_path(provider_id)
    if path.exists():
        return path.read_text().strip()
    return None


def clear_org_id_cache(provider_id: str | None = None) -> None:
    """
    Clear organization ID cache.

    Args:
        provider_id: Clear for specific provider, or all if None
    """
    if provider_id:
        path = org_id_path(provider_id)
        if path.exists():
            path.unlink()
    else:
        cache = org_ids_dir()
        if cache.exists():
            for path in cache.iterdir():
                path.unlink()
```

### Cache Clearing

```python
def clear_all_cache() -> None:
    """Clear all cached data (snapshots, org IDs)."""
    import shutil

    cache = cache_dir()
    if cache.exists():
        shutil.rmtree(cache)


def clear_provider_cache(provider_id: str) -> None:
    """Clear cached data for a specific provider."""
    # Clear snapshot
    snapshot = snapshot_path(provider_id)
    if snapshot.exists():
        snapshot.unlink()

    # Clear org ID
    clear_org_id_cache(provider_id)
```

---

## CLI Commands

### Config Subcommands

```python
import typer
from rich.console import Console
from rich.table import Table
from rich.panel import Panel

config_app = typer.Typer(help="Manage configuration")
console = Console()

@config_app.command("show")
def config_show() -> None:
    """Show current configuration."""
    config = get_config()

    # Display settings
    table = Table(title="Configuration", show_header=True)
    table.add_column("Setting", style="bold")
    table.add_column("Value")

    # Enabled providers
    if config.enabled_providers is None:
        providers = "all"
    else:
        providers = ", ".join(config.enabled_providers) or "(none)"
    table.add_row("Enabled providers", providers)

    # Display settings
    table.add_row("Show remaining", str(config.display.show_remaining))
    table.add_row("Pace colors", str(config.display.pace_colors))
    table.add_row("Reset format", config.display.reset_format)

    # Fetch settings
    table.add_row("Fetch timeout", f"{config.fetch.timeout}s")
    table.add_row("Max concurrent", str(config.fetch.max_concurrent))
    table.add_row("Stale threshold", f"{config.fetch.stale_threshold}m")

    # Credential settings
    table.add_row("Use keyring", str(config.credentials.use_keyring))
    table.add_row("Reuse provider creds", str(config.credentials.reuse_provider_credentials))

    console.print(table)


@config_app.command("path")
def config_path() -> None:
    """Show configuration and cache paths."""
    table = Table(title="Paths", show_header=True)
    table.add_column("Type", style="bold")
    table.add_column("Path")
    table.add_column("Exists")

    paths = [
        ("Config file", config_file()),
        ("Credentials", credentials_dir()),
        ("Cache", cache_dir()),
        ("Snapshots", snapshots_dir()),
    ]

    for name, path in paths:
        exists = "yes" if path.exists() else "no"
        table.add_row(name, str(path), exists)

    console.print(table)


@config_app.command("reset")
def config_reset(
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
) -> None:
    """Reset configuration to defaults."""
    if not force:
        if not typer.confirm("Reset configuration to defaults?"):
            raise typer.Abort()

    path = config_file()
    if path.exists():
        path.unlink()

    # Also clear the singleton
    global _config
    _config = None

    console.print("[green]Configuration reset to defaults[/green]")


@config_app.command("edit")
def config_edit() -> None:
    """Open configuration file in editor."""
    import os
    import subprocess

    path = config_file()

    # Create default config if doesn't exist
    if not path.exists():
        Config().save()
        console.print(f"[dim]Created default config at {path}[/dim]")

    # Find editor
    editor = os.environ.get("EDITOR", os.environ.get("VISUAL", "nano"))

    try:
        subprocess.run([editor, str(path)])
        reload_config()
        console.print("[green]Configuration reloaded[/green]")
    except FileNotFoundError:
        console.print(f"[red]Editor '{editor}' not found[/red]")
        console.print(f"[dim]Edit manually: {path}[/dim]")
        raise typer.Exit(1)
```

### Key Management Commands

Per-provider key management:

```python
key_app = typer.Typer(help="Manage provider credentials")

@key_app.callback(invoke_without_command=True)
def key_status(ctx: typer.Context) -> None:
    """Show credential status for all providers."""
    if ctx.invoked_subcommand is not None:
        return

    from vibeusage.providers import list_provider_ids

    table = Table(title="Credential Status", show_header=True)
    table.add_column("Provider", style="bold")
    table.add_column("Status")
    table.add_column("Source")

    for provider_id in list_provider_ids():
        status, source = check_provider_credentials(provider_id)

        if status == "configured":
            status_str = "[green]configured[/green]"
        elif status == "missing":
            status_str = "[yellow]not configured[/yellow]"
        else:
            status_str = "[red]error[/red]"

        table.add_row(provider_id.title(), status_str, source or "")

    console.print(table)
    console.print()
    console.print("[dim]Run 'vibeusage auth <provider>' to configure credentials[/dim]")


def check_provider_credentials(provider_id: str) -> tuple[str, str | None]:
    """
    Check credential status for a provider.

    Returns:
        (status, source) where status is "configured", "missing", or "error"
    """
    # Check for OAuth credentials
    oauth_path = find_provider_credential(provider_id, "oauth.json")
    if oauth_path:
        return ("configured", f"oauth ({oauth_path.parent.name})")

    # Check for session key
    session_path = find_provider_credential(provider_id, "session-key")
    if session_path:
        return ("configured", "session-key")

    # Check for API key
    api_path = find_provider_credential(provider_id, "api-key")
    if api_path:
        return ("configured", "api-key")

    # Check keyring
    if get_from_keyring(provider_id, "session-key"):
        return ("configured", "keyring")

    return ("missing", None)


# Provider-specific key commands
def create_key_command(provider_id: str, key_type: str, key_prefix: str | None = None):
    """Factory for provider-specific key commands."""

    provider_app = typer.Typer(help=f"Manage {provider_id.title()} credentials")

    @provider_app.callback(invoke_without_command=True)
    def show(ctx: typer.Context) -> None:
        """Show credential status."""
        if ctx.invoked_subcommand is not None:
            return

        status, source = check_provider_credentials(provider_id)

        if status == "configured":
            console.print(f"[green]{provider_id.title()} credentials configured[/green]")
            console.print(f"[dim]Source: {source}[/dim]")

            # Show masked key if session-key
            path = credential_path(provider_id, key_type)
            if path.exists():
                key = path.read_text().strip()
                masked = f"{key[:16]}...{key[-8:]}" if len(key) > 24 else "***"
                console.print(f"[dim]Key: {masked}[/dim]")
        else:
            console.print(f"[yellow]{provider_id.title()} credentials not configured[/yellow]")
            console.print(f"[dim]Run 'vibeusage key {provider_id} set' to configure[/dim]")

    @provider_app.command("set")
    def set_key(
        value: str = typer.Argument(None, help="Credential value (or enter interactively)"),
    ) -> None:
        """Set the credential."""
        if value is None:
            console.print(f"[bold]Set {provider_id.title()} Credential[/bold]")
            console.print()
            value = typer.prompt(f"Enter {key_type}", hide_input=True)

        # Validate format if prefix specified
        if key_prefix and not value.startswith(key_prefix):
            console.print(f"[yellow]Warning: doesn't start with '{key_prefix}'[/yellow]")
            if not typer.confirm("Save anyway?"):
                raise typer.Abort()

        # Save credential
        path = credential_path(provider_id, key_type)
        write_credential(path, value)

        # Also store in keyring if enabled
        store_in_keyring(provider_id, key_type, value)

        # Clear cached data for this provider
        clear_provider_cache(provider_id)

        console.print(f"[green]Credential saved to {path}[/green]")

    @provider_app.command("delete")
    def delete_key(
        force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    ) -> None:
        """Delete the credential."""
        if not force:
            if not typer.confirm(f"Delete {provider_id.title()} credential?"):
                raise typer.Abort()

        # Delete from file
        path = credential_path(provider_id, key_type)
        deleted_file = delete_credential(path)

        # Delete from keyring
        deleted_keyring = delete_from_keyring(provider_id, key_type)

        if deleted_file or deleted_keyring:
            console.print(f"[green]{provider_id.title()} credential deleted[/green]")
        else:
            console.print(f"[dim]No credential found to delete[/dim]")

    return provider_app


# Register provider-specific key commands
key_app.add_typer(
    create_key_command("claude", "session-key", "sk-ant-sid01-"),
    name="claude",
)
key_app.add_typer(
    create_key_command("codex", "session-key"),
    name="codex",
)
key_app.add_typer(
    create_key_command("cursor", "session-key"),
    name="cursor",
)
key_app.add_typer(
    create_key_command("zai", "api-key"),
    name="zai",
)
```

### Cache Commands

```python
cache_app = typer.Typer(help="Manage cached data")

@cache_app.command("clear")
def cache_clear(
    provider: str = typer.Argument(None, help="Provider to clear (or all if not specified)"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
) -> None:
    """Clear cached data."""
    if provider:
        if not force:
            if not typer.confirm(f"Clear cache for {provider}?"):
                raise typer.Abort()
        clear_provider_cache(provider)
        console.print(f"[green]Cache cleared for {provider}[/green]")
    else:
        if not force:
            if not typer.confirm("Clear all cached data?"):
                raise typer.Abort()
        clear_all_cache()
        console.print("[green]All cache cleared[/green]")


@cache_app.command("show")
def cache_show() -> None:
    """Show cached data status."""
    from vibeusage.providers import list_provider_ids

    table = Table(title="Cache Status", show_header=True)
    table.add_column("Provider", style="bold")
    table.add_column("Snapshot")
    table.add_column("Age")
    table.add_column("Org ID")

    for provider_id in list_provider_ids():
        # Check snapshot
        snapshot = load_cached_snapshot(provider_id)
        if snapshot:
            age_minutes = int((datetime.now(snapshot.fetched_at.tzinfo) - snapshot.fetched_at).total_seconds() / 60)
            snapshot_status = "[green]cached[/green]"
            age_str = f"{age_minutes}m ago"
        else:
            snapshot_status = "[dim]none[/dim]"
            age_str = ""

        # Check org ID
        org_id = load_cached_org_id(provider_id)
        org_status = "[green]cached[/green]" if org_id else "[dim]none[/dim]"

        table.add_row(provider_id.title(), snapshot_status, age_str, org_status)

    console.print(table)
```

---

## Environment Variables

Configuration can be overridden via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `VIBEUSAGE_CONFIG_DIR` | Override config directory | `/custom/config` |
| `VIBEUSAGE_CACHE_DIR` | Override cache directory | `/custom/cache` |
| `VIBEUSAGE_ENABLED_PROVIDERS` | Comma-separated provider list | `claude,codex` |
| `VIBEUSAGE_NO_COLOR` | Disable colored output | `1` |

```python
import os

def config_dir() -> Path:
    """Return user config directory, respecting env override."""
    if override := os.environ.get("VIBEUSAGE_CONFIG_DIR"):
        return Path(override)
    return Path(platformdirs.user_config_dir(APP_NAME))


def cache_dir() -> Path:
    """Return user cache directory, respecting env override."""
    if override := os.environ.get("VIBEUSAGE_CACHE_DIR"):
        return Path(override)
    return Path(platformdirs.user_cache_dir(APP_NAME))
```

---

## Default Configuration

When no config file exists, these defaults apply:

```python
DEFAULT_CONFIG = Config(
    enabled_providers=None,  # All available
    display=DisplayConfig(
        show_remaining=True,
        pace_colors=True,
        reset_format="countdown",
    ),
    fetch=FetchConfig(
        timeout=30,
        max_concurrent=5,
        stale_threshold=10,
    ),
    credentials=CredentialsConfig(
        use_keyring=False,
        reuse_provider_credentials=True,
    ),
    providers={},  # Use defaults for all providers
)
```

---

## Migration

### From ccusage (Example Script)

Users migrating from the ccusage example can move their session key:

```bash
# Old location
~/.config/ccusage/session-key

# New location
~/.config/vibeusage/credentials/claude/session-key
```

Auto-migration:

```python
def migrate_legacy_config() -> None:
    """Migrate configuration from legacy locations."""
    # Check for ccusage session key
    legacy_key = Path.home() / ".config" / "ccusage" / "session-key"
    new_key = credential_path("claude", "session-key")

    if legacy_key.exists() and not new_key.exists():
        new_key.parent.mkdir(parents=True, exist_ok=True)
        import shutil
        shutil.copy2(legacy_key, new_key)
        new_key.chmod(0o600)
```

---

## Security Considerations

### File Permissions

All credential files are created with mode `0o600` (owner read/write only). The credential directory should be `0o700`.

### Sensitive Data

- Session keys and OAuth tokens are sensitive
- Never log credential values
- Mask credentials in CLI output (show first/last few characters only)
- Clear credentials from memory after use where possible

### Keyring vs Files

| Aspect | Files | Keyring |
|--------|-------|---------|
| Cross-platform | Yes | Depends on keyring backend |
| Backup/sync | Easy | Requires keyring-specific backup |
| Visibility | Hidden files | Truly hidden |
| Dependency | None | `keyring` package |

Recommendation: Use files by default for simplicity, keyring as opt-in for users with higher security requirements.

---

## Implementation Checklist

- [ ] `vibeusage/config/__init__.py` - Module exports
- [ ] `vibeusage/config/paths.py` - Path resolution functions
- [ ] `vibeusage/config/settings.py` - Config dataclass and load/save
- [ ] `vibeusage/config/credentials.py` - Credential read/write/find
- [ ] `vibeusage/config/cache.py` - Snapshot and org ID caching
- [ ] `vibeusage/config/keyring.py` - Optional keyring integration
- [ ] `vibeusage/cli/commands/config.py` - Config CLI commands
- [ ] `vibeusage/cli/commands/key.py` - Key management CLI commands
- [ ] `vibeusage/cli/commands/cache.py` - Cache CLI commands

---

## Open Questions

1. **Config file format**: TOML is human-readable but requires a dependency. Should we use JSON or INI instead for fewer dependencies?

2. **Multi-config support**: Should we support multiple config profiles (e.g., `vibeusage --profile work`)?

3. **Config validation**: Should we validate config on load and warn about invalid values, or silently fall back to defaults?

4. **Credential expiry tracking**: Should we track when credentials were last used/refreshed to help users diagnose auth issues?

5. **Auto-migration**: How aggressively should we auto-migrate from legacy/provider CLI locations?

## Implementation Notes

- Use `tomllib` (Python 3.11+) or `tomli` for reading, `tomli-w` for writing TOML
- Use `msgspec.Struct` for config models - provides validation and clean defaults
- Use `msgspec.convert()` to convert TOML dicts to config structs
- Use `msgspec.to_builtins()` to convert structs back to dicts for TOML writing
- Use `msgspec.json.encode()` / `msgspec.json.decode()` for cached snapshots
- Environment variable overrides should take precedence over file config
- The `get_config()` singleton pattern allows lazy loading and easy testing via `reload_config()`
