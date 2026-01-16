# Spec 01: Core Architecture

**Status**: Draft
**Dependencies**: 02-data-models, 03-authentication
**Dependents**: 04-providers, 05-cli-interface, 06-configuration, 07-error-handling

## Overview

This specification defines the overall system architecture for vibeusage, including the provider abstraction layer, fetch pipeline, and module structure. The design is inspired by CodexBar's architecture but simplified for a Python CLI tool.

## Design Goals

1. **Provider Abstraction**: Consistent interface for all LLM providers
2. **Fallback Chains**: Multiple fetch strategies per provider with automatic fallback
3. **Async-First**: Non-blocking I/O for concurrent provider fetches
4. **Extensibility**: Easy to add new providers without modifying core code
5. **Testability**: Dependency injection and interfaces for mocking

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                           CLI Layer                              │
│    (vibeusage.cli - command parsing, output formatting)         │
└─────────────────────────────────────┬───────────────────────────┘
                                      │
┌─────────────────────────────────────▼───────────────────────────┐
│                        Orchestration Layer                       │
│    (vibeusage.core - concurrent fetch, result aggregation)      │
└─────────────────────────────────────┬───────────────────────────┘
                                      │
┌─────────────────────────────────────▼───────────────────────────┐
│                         Provider Layer                           │
│    (vibeusage.providers - provider implementations)             │
│    ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐             │
│    │ Claude  │ │ Codex   │ │ Copilot │ │ Cursor  │  ...        │
│    └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘             │
└─────────┼───────────┼───────────┼───────────┼───────────────────┘
          │           │           │           │
┌─────────▼───────────▼───────────▼───────────▼───────────────────┐
│                      Fetch Strategy Layer                        │
│    (vibeusage.strategies - OAuth, Web, CLI, etc.)               │
└─────────────────────────────────────┬───────────────────────────┘
                                      │
┌─────────────────────────────────────▼───────────────────────────┐
│                       Authentication Layer                       │
│    (vibeusage.auth - credential management)                     │
└─────────────────────────────────────────────────────────────────┘
```

## Provider Protocol

Every provider implements the `Provider` protocol, defining its identity, capabilities, and fetch behavior.

### Provider Interface

```python
from abc import ABC, abstractmethod
from typing import ClassVar

import msgspec

from vibeusage.models import UsageSnapshot, ProviderStatus
from vibeusage.auth import AuthCredentials


class ProviderMetadata(msgspec.Struct, frozen=True):
    """Static provider information."""

    id: str                     # Unique identifier (e.g., "claude", "codex")
    name: str                   # Display name (e.g., "Claude", "OpenAI Codex")
    description: str            # Short description
    homepage: str               # Provider homepage URL
    status_url: str | None = None      # Status page URL (for linking)
    dashboard_url: str | None = None   # Usage dashboard URL (for linking)


class Provider(ABC):
    """Base protocol for all LLM providers."""

    # Class-level metadata (immutable)
    metadata: ClassVar[ProviderMetadata]

    @abstractmethod
    def fetch_strategies(self) -> list["FetchStrategy"]:
        """
        Return fetch strategies in priority order.

        The orchestrator will try each strategy until one succeeds.
        """
        ...

    @abstractmethod
    async def fetch_status(self) -> ProviderStatus:
        """
        Fetch provider operational status.

        May be called independently of usage fetching.
        """
        ...

    def is_enabled(self, config: "Config") -> bool:
        """Check if this provider is enabled in configuration."""
        return self.metadata.id in config.enabled_providers
```

### Provider Registration

Providers register themselves via a registry pattern:

```python
from typing import Type

# Global provider registry
_PROVIDERS: dict[str, Type[Provider]] = {}


def register_provider(cls: Type[Provider]) -> Type[Provider]:
    """Decorator to register a provider class."""
    _PROVIDERS[cls.metadata.id] = cls
    return cls


def get_provider(provider_id: str) -> Provider:
    """Get provider instance by ID."""
    if provider_id not in _PROVIDERS:
        raise ValueError(f"Unknown provider: {provider_id}")
    return _PROVIDERS[provider_id]()


def get_all_providers() -> list[Provider]:
    """Get all registered provider instances."""
    return [cls() for cls in _PROVIDERS.values()]


def list_provider_ids() -> list[str]:
    """List all registered provider IDs."""
    return list(_PROVIDERS.keys())
```

### Example Provider Implementation

```python
from vibeusage.providers.base import Provider, ProviderMetadata, register_provider
from vibeusage.strategies import OAuthFetchStrategy, WebFetchStrategy, CLIFetchStrategy

@register_provider
class ClaudeProvider(Provider):
    """Anthropic Claude provider."""

    metadata = ProviderMetadata(
        id="claude",
        name="Claude",
        description="Anthropic's Claude AI assistant",
        homepage="https://claude.ai",
        status_url="https://status.anthropic.com",
        dashboard_url="https://claude.ai/settings/usage",
    )

    def fetch_strategies(self) -> list[FetchStrategy]:
        return [
            ClaudeOAuthFetchStrategy(),
            ClaudeWebFetchStrategy(),
            ClaudeCLIFetchStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        return await fetch_statuspage_status("https://status.anthropic.com")
```

## Fetch Strategy Pattern

Each provider can have multiple ways to fetch usage data. Strategies encapsulate authentication and API interaction.

### FetchStrategy Interface

```python
from abc import ABC, abstractmethod

import msgspec

from vibeusage.models import UsageSnapshot
from vibeusage.auth import AuthResult, AuthCredentials


class FetchResult(msgspec.Struct, frozen=True):
    """Result of a fetch attempt."""

    success: bool
    snapshot: UsageSnapshot | None = None
    error: str | None = None
    should_fallback: bool = True  # Whether to try next strategy

    @classmethod
    def ok(cls, snapshot: UsageSnapshot) -> "FetchResult":
        return cls(success=True, snapshot=snapshot, should_fallback=False)

    @classmethod
    def fail(cls, error: str, should_fallback: bool = True) -> "FetchResult":
        return cls(success=False, error=error, should_fallback=should_fallback)

    @classmethod
    def fatal(cls, error: str) -> "FetchResult":
        """Error that should not trigger fallback (e.g., rate limit)."""
        return cls(success=False, error=error, should_fallback=False)


class FetchStrategy(ABC):
    """Base class for fetch strategies."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Strategy identifier (e.g., 'oauth', 'web', 'cli')."""
        ...

    @abstractmethod
    async def is_available(self) -> bool:
        """
        Check if this strategy can be attempted.

        Returns True if credentials/requirements exist.
        Should be fast (no network calls).
        """
        ...

    @abstractmethod
    async def fetch(self) -> FetchResult:
        """
        Attempt to fetch usage data.

        Returns FetchResult with snapshot or error details.
        """
        ...
```

### Strategy Pipeline Execution

The orchestration layer executes strategies in sequence:

```python
class FetchAttempt(msgspec.Struct):
    """Record of a single fetch attempt."""

    strategy: str
    success: bool
    error: str | None = None
    duration_ms: int = 0


class FetchOutcome(msgspec.Struct):
    """Complete result of fetching from a provider."""

    provider_id: str
    success: bool
    snapshot: UsageSnapshot | None
    source: str | None  # Which strategy succeeded
    attempts: list[FetchAttempt]  # All attempts for debugging
    error: str | None = None  # Final error if all failed


async def execute_fetch_pipeline(
    provider: Provider,
    strategies: list[FetchStrategy],
) -> FetchOutcome:
    """
    Execute fetch strategies in order until one succeeds.

    Args:
        provider: The provider being fetched
        strategies: Strategies in priority order

    Returns:
        FetchOutcome with result and attempt history
    """
    attempts = []

    for strategy in strategies:
        # Check availability (fast, no network)
        if not await strategy.is_available():
            attempts.append(FetchAttempt(
                strategy=strategy.name,
                success=False,
                error="Not available",
            ))
            continue

        # Attempt fetch
        start = time.monotonic()
        try:
            result = await asyncio.wait_for(
                strategy.fetch(),
                timeout=30.0,  # Per-strategy timeout
            )
        except asyncio.TimeoutError:
            result = FetchResult.fail("Timeout", should_fallback=True)
        except Exception as e:
            result = FetchResult.fail(str(e), should_fallback=True)

        duration_ms = int((time.monotonic() - start) * 1000)

        attempts.append(FetchAttempt(
            strategy=strategy.name,
            success=result.success,
            error=result.error,
            duration_ms=duration_ms,
        ))

        if result.success:
            return FetchOutcome(
                provider_id=provider.metadata.id,
                success=True,
                snapshot=result.snapshot,
                source=strategy.name,
                attempts=attempts,
            )

        if not result.should_fallback:
            # Fatal error - don't try other strategies
            break

    # All strategies failed
    return FetchOutcome(
        provider_id=provider.metadata.id,
        success=False,
        snapshot=None,
        source=None,
        attempts=attempts,
        error=attempts[-1].error if attempts else "No strategies available",
    )
```

## Orchestration Layer

The orchestration layer coordinates fetching from multiple providers concurrently.

### Concurrent Fetching

```python
import asyncio
from typing import Callable

async def fetch_all_providers(
    providers: list[Provider],
    on_complete: Callable[[FetchOutcome], None] | None = None,
    max_concurrent: int = 5,
) -> list[FetchOutcome]:
    """
    Fetch usage from all providers concurrently.

    Args:
        providers: List of providers to fetch
        on_complete: Optional callback for each completion (for progress)
        max_concurrent: Maximum concurrent fetches

    Returns:
        List of FetchOutcome, one per provider
    """
    semaphore = asyncio.Semaphore(max_concurrent)

    async def fetch_one(provider: Provider) -> FetchOutcome:
        async with semaphore:
            strategies = provider.fetch_strategies()
            outcome = await execute_fetch_pipeline(provider, strategies)
            if on_complete:
                on_complete(outcome)
            return outcome

    tasks = [fetch_one(p) for p in providers]
    return await asyncio.gather(*tasks)


async def fetch_single_provider(provider_id: str) -> FetchOutcome:
    """Fetch usage from a single provider."""
    provider = get_provider(provider_id)
    strategies = provider.fetch_strategies()
    return await execute_fetch_pipeline(provider, strategies)
```

### Result Aggregation

```python
from datetime import datetime

class AggregatedResult(msgspec.Struct):
    """Combined results from all providers."""

    snapshots: dict[str, UsageSnapshot]  # provider_id -> snapshot
    errors: dict[str, str]               # provider_id -> error message
    fetched_at: datetime

    def successful_providers(self) -> list[str]:
        """Return IDs of providers that succeeded."""
        return list(self.snapshots.keys())

    def failed_providers(self) -> list[str]:
        """Return IDs of providers that failed."""
        return list(self.errors.keys())


def aggregate_results(outcomes: list[FetchOutcome]) -> AggregatedResult:
    """Aggregate fetch outcomes into a combined result."""
    snapshots = {}
    errors = {}

    for outcome in outcomes:
        if outcome.success and outcome.snapshot:
            snapshots[outcome.provider_id] = outcome.snapshot
        else:
            errors[outcome.provider_id] = outcome.error or "Unknown error"

    return AggregatedResult(
        snapshots=snapshots,
        errors=errors,
        fetched_at=datetime.now().astimezone(),
    )
```

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                          User Command                                │
│                     vibeusage [provider] [--json]                   │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Determine Providers                             │
│   - All enabled providers (default)                                  │
│   - Single specified provider                                        │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Concurrent Fetch Pipeline                         │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐                             │
│   │Provider1│  │Provider2│  │Provider3│  (parallel)                 │
│   └────┬────┘  └────┬────┘  └────┬────┘                             │
│        │            │            │                                   │
│   Strategy Chain    │       Strategy Chain                           │
│   OAuth→Web→CLI     │       API Key                                  │
└────────┬────────────┴────────────┬──────────────────────────────────┘
         │                         │
         ▼                         ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Normalize to UsageSnapshot                       │
│   Provider-specific API responses → Common data models               │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Aggregate Results                             │
│   Combine successful snapshots + collect errors                      │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                    ┌────────────┴────────────┐
                    ▼                         ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│      Rich Display       │     │      JSON Output        │
│   Tables, colors, bars  │     │   Structured data       │
└─────────────────────────┘     └─────────────────────────┘
```

## Module Structure

```
vibeusage/
├── __init__.py
├── __main__.py              # Entry point: python -m vibeusage
├── cli/
│   ├── __init__.py          # CLI app and commands
│   ├── atyper.py            # ATyper async wrapper for Typer
│
├── models.py                # Data models (spec 02)
│   - PeriodType, UsagePeriod, OverageUsage
│   - ProviderIdentity, ProviderStatus
│   - UsageSnapshot
│
├── auth/                    # Authentication (spec 03)
│   ├── __init__.py
│   ├── base.py              # AuthStrategy, AuthResult, AuthCredentials
│   ├── oauth.py             # OAuth2Strategy
│   ├── session.py           # ManualSessionStrategy
│   ├── cookies.py           # BrowserCookieStrategy
│   ├── apikey.py            # APIKeyStrategy
│   ├── cli.py               # CLISessionStrategy
│   ├── device_flow.py       # GitHubDeviceFlowStrategy
│   └── local_process.py     # LocalProcessStrategy
│
├── core/                    # Orchestration layer
│   ├── __init__.py
│   ├── fetch.py             # execute_fetch_pipeline
│   ├── orchestrator.py      # fetch_all_providers, fetch_single_provider
│   └── aggregate.py         # AggregatedResult
│
├── providers/               # Provider implementations (spec 04)
│   ├── __init__.py          # Provider registry, get_provider
│   ├── base.py              # Provider protocol, ProviderMetadata
│   ├── claude/
│   │   ├── __init__.py
│   │   ├── provider.py      # ClaudeProvider
│   │   ├── oauth.py         # ClaudeOAuthFetchStrategy
│   │   ├── web.py           # ClaudeWebFetchStrategy
│   │   └── cli.py           # ClaudeCLIFetchStrategy
│   ├── codex/
│   │   └── ...
│   ├── copilot/
│   │   └── ...
│   └── ...
│
├── strategies/              # Fetch strategy base classes
│   ├── __init__.py
│   ├── base.py              # FetchStrategy, FetchResult
│   └── status.py            # Status page fetching utilities
│
├── display/                 # CLI output formatting (spec 05)
│   ├── __init__.py
│   ├── rich.py              # Rich-based rendering
│   ├── json.py              # JSON output
│   └── colors.py            # Pace-based coloring
│
├── config/                  # Configuration (spec 06)
│   ├── __init__.py
│   ├── settings.py          # User preferences
│   ├── credentials.py       # Credential file management
│   └── paths.py             # Platform-specific paths
│
└── cache/                   # Caching layer
    ├── __init__.py
    ├── snapshots.py         # Usage snapshot cache
    └── org_ids.py           # Organization ID cache
```

## Async vs Sync

vibeusage uses async/await throughout for non-blocking I/O:

### Why Async

1. **Concurrent provider fetches**: Fetch from 5+ providers simultaneously
2. **HTTP client efficiency**: httpx AsyncClient connection pooling
3. **CLI subprocess handling**: Non-blocking CLI shelling with timeouts
4. **Future extensibility**: Background refresh, watch mode

### Typer Async Compatibility

Typer doesn't natively support async commands. We use an `ATyper` wrapper that automatically wraps async functions with `asyncio.run()`:

```python
from __future__ import annotations

import asyncio
import inspect
from functools import wraps
from typing import Any, Protocol, cast, override

from typer import Typer
from typer.models import CommandFunctionType


class CommandDecorator(Protocol):
    """A function that decorates a Typer command."""

    def __call__(self, __func: CommandFunctionType, /) -> CommandFunctionType: ...


class ATyper(Typer):
    """Extended Typer class that supports async functions in commands and callbacks."""

    @override
    def callback(self, **kwargs: Any) -> CommandDecorator:
        """Override callback to support async functions."""
        decorator = super().callback(**kwargs)
        return self._async_wrap_decorator(decorator)

    @override
    def command(self, name: str | None = None, **kwargs: Any) -> CommandDecorator:
        """Override command to support async functions."""
        decorator = super().command(name, **kwargs)
        return self._async_wrap_decorator(decorator)

    def _async_wrap_decorator(self, decorator: CommandDecorator) -> CommandDecorator:
        """Wrap a decorator to make it async-aware."""

        def wrapper(func: CommandFunctionType) -> CommandFunctionType:
            return async_me_maybe(decorator, func)

        return cast(CommandDecorator, wrapper)


def async_me_maybe(
    decorator: CommandDecorator,
    func: CommandFunctionType,
) -> CommandFunctionType:
    """Wrap async functions with asyncio.run."""
    if inspect.iscoroutinefunction(func):

        @wraps(func)
        def runner(*args: object, **kwargs: object) -> object:
            result: object = asyncio.run(func(*args, **kwargs))
            return result

        return decorator(cast(CommandFunctionType, runner))
    else:
        return decorator(func)
```

### Async Commands

With `ATyper`, async commands work naturally:

```python
from vibeusage.cli.atyper import ATyper

app = ATyper()

# Async commands just work
@app.command()
async def show(provider: str | None = None):
    """Show usage for providers."""
    if provider:
        outcome = await fetch_single_provider(provider)
    else:
        outcomes = await fetch_all_providers(get_enabled_providers())
    # ... display results
```

### HTTP Client Management

```python
import httpx
from contextlib import asynccontextmanager

# Shared client with connection pooling
_client: httpx.AsyncClient | None = None


@asynccontextmanager
async def get_http_client():
    """Get or create shared HTTP client."""
    global _client
    if _client is None:
        _client = httpx.AsyncClient(
            timeout=httpx.Timeout(30.0),
            follow_redirects=True,
            http2=True,
        )
    try:
        yield _client
    finally:
        pass  # Keep client alive for reuse


async def cleanup():
    """Close HTTP client on shutdown."""
    global _client
    if _client:
        await _client.aclose()
        _client = None
```

## Error Boundaries

Each layer has defined error handling responsibilities:

| Layer | Error Handling |
|-------|----------------|
| CLI | Display user-friendly messages, exit codes |
| Orchestrator | Aggregate errors, continue on partial failure |
| Fetch Pipeline | Try fallback strategies, record attempt history |
| Strategy | Catch HTTP/network errors, return FetchResult |
| Auth | Return AuthResult with error details |

### Partial Failure Handling

```python
async def show_usage(providers: list[str] | None = None):
    """Show usage, handling partial failures gracefully."""
    if providers is None:
        providers = get_enabled_providers()

    outcomes = await fetch_all_providers(
        [get_provider(p) for p in providers]
    )
    result = aggregate_results(outcomes)

    # Display successful results
    for provider_id, snapshot in result.snapshots.items():
        display_snapshot(snapshot)

    # Report failures (but don't fail entirely)
    if result.errors:
        console.print("\n[dim]Some providers failed:[/dim]")
        for provider_id, error in result.errors.items():
            console.print(f"  [red]{provider_id}[/red]: {error}")
```

## Configuration Integration

The architecture integrates with configuration at multiple points:

```python
from vibeusage.config import Config, get_config

# Provider enablement
def get_enabled_providers() -> list[Provider]:
    """Get providers enabled in user config."""
    config = get_config()
    return [
        get_provider(pid) for pid in list_provider_ids()
        if pid in config.enabled_providers
    ]

# Auth credential paths
def get_credential_path(provider_id: str, credential_type: str) -> Path:
    """Get platform-appropriate credential path."""
    config = get_config()
    return config.credentials_dir / provider_id / credential_type

# Display preferences
def should_show_pace_colors() -> bool:
    """Check if pace-based coloring is enabled."""
    config = get_config()
    return config.display.pace_colors
```

## Extension Points

The architecture supports extension at several points:

### Adding a New Provider

1. Create provider module in `vibeusage/providers/<name>/`
2. Implement `Provider` class with metadata and strategies
3. Decorate with `@register_provider`
4. Add credential configuration to spec 06

### Adding a New Strategy Type

1. Implement `FetchStrategy` subclass
2. Add to provider's `fetch_strategies()` return list
3. Implement corresponding auth strategy if needed

### Adding a New Auth Method

1. Implement `AuthStrategy` and `AuthCredentials` classes
2. Add configuration in `vibeusage/auth/`
3. Wire into relevant provider strategies

## Performance Considerations

### Concurrent Fetching

- Default max concurrent: 5 (prevents overwhelming network/APIs)
- Per-strategy timeout: 30 seconds
- Total command timeout: 60 seconds

### Caching

- Organization IDs cached indefinitely (rarely change)
- Usage snapshots cached for staleness display
- Credentials never cached in memory after use

### Startup Time

- Lazy provider registration (import on first use)
- HTTP client created on first request
- Credentials loaded on demand

## Testing Strategy

### Unit Testing

```python
# Mock strategies for unit tests
class MockFetchStrategy(FetchStrategy):
    def __init__(self, result: FetchResult):
        self._result = result

    @property
    def name(self) -> str:
        return "mock"

    async def is_available(self) -> bool:
        return True

    async def fetch(self) -> FetchResult:
        return self._result


async def test_fetch_pipeline_fallback():
    """Test that pipeline falls back on failure."""
    strategies = [
        MockFetchStrategy(FetchResult.fail("First failed")),
        MockFetchStrategy(FetchResult.ok(mock_snapshot)),
    ]
    outcome = await execute_fetch_pipeline(mock_provider, strategies)
    assert outcome.success
    assert outcome.source == "mock"
    assert len(outcome.attempts) == 2
```

### Integration Testing

```python
# Test with real credentials (optional, CI-skipped)
@pytest.mark.integration
async def test_claude_oauth_fetch():
    """Test Claude OAuth fetch with real credentials."""
    provider = get_provider("claude")
    outcome = await fetch_single_provider("claude")
    assert outcome.success
    assert outcome.snapshot.provider == "claude"
```

## Open Questions

1. **Plugin System**: Should providers be loadable as plugins (entry points) for third-party extensions?

2. **Retry Logic**: Should strategies have built-in retry with exponential backoff, or is fallback-to-next-strategy sufficient?

3. **Rate Limiting**: Should we track and respect rate limits across strategies, or handle per-strategy?

4. **Background Refresh**: Is watch mode (`vibeusage --watch`) in scope? Would require a different execution model.

## Implementation Notes

- Use `httpx` for async HTTP (supports HTTP/2, connection pooling)
- Use `typer` with `ATyper` wrapper for async command support (see Async Boundaries section)
- Use `rich` for terminal output (supports async progress)
- Use `msgspec` for all data structures (Structs with `frozen=True`)
- Consider `structlog` for structured logging during development
- Type check with `pyright` in strict mode
