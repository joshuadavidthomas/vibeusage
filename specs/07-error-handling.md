# Spec 07: Error Handling & Resilience

**Status**: Draft
**Dependencies**: 01-architecture, 02-data-models, 03-authentication, 04-providers, 05-cli-interface, 06-configuration
**Dependents**: None (final spec)

## Overview

This specification defines error handling, retry logic, and degraded operation modes for vibeusage. The system is designed to be resilient to transient failures, provide actionable error messages, and gracefully degrade when some providers are unavailable.

## Design Goals

1. **User-Friendly Messages**: Errors include clear remediation steps
2. **Graceful Degradation**: Partial failures don't break entire workflow
3. **Stale Data Display**: Show cached data when fresh fetches fail
4. **Prevent Flapping**: Don't spam retries on persistent failures
5. **Actionable Feedback**: Every error tells the user what to do next
6. **Structured for Scripting**: JSON output includes error details for automation

---

## Error Classification

### Error Categories

Errors are classified into categories that determine handling behavior:

```python
from enum import StrEnum, auto

class ErrorCategory(StrEnum):
    """Error categories for handling decisions."""

    AUTHENTICATION = "authentication"    # Credentials expired/invalid
    AUTHORIZATION = "authorization"      # Account lacks access
    RATE_LIMITED = "rate_limited"        # Too many requests
    NETWORK = "network"                  # Connection/timeout issues
    PROVIDER = "provider"                # Provider-side errors (5xx)
    PARSE = "parse"                      # Response parsing failed
    CONFIGURATION = "configuration"      # Missing/invalid config
    NOT_FOUND = "not_found"              # Resource not found (404)
    UNKNOWN = "unknown"                  # Unclassified errors
```

### Error Severity

```python
class ErrorSeverity(StrEnum):
    """Error severity levels."""

    FATAL = "fatal"        # Cannot continue, exit immediately
    RECOVERABLE = "recoverable"  # Try next strategy/provider
    TRANSIENT = "transient"      # Retry may succeed
    WARNING = "warning"          # Non-blocking, inform user
```

### Error Structure

```python
from datetime import datetime

import msgspec


class VibeusageError(msgspec.Struct, frozen=True):
    """Structured error with category and remediation."""

    message: str                           # Human-readable error message
    category: ErrorCategory
    severity: ErrorSeverity
    provider: str | None = None            # Provider that caused error
    remediation: str | None = None         # How to fix it
    details: dict | None = None            # Technical details for debugging
    timestamp: datetime = msgspec.field(default_factory=lambda: datetime.now().astimezone())
```

**Note**: msgspec Structs are natively JSON-serializable via `msgspec.json.encode()`. No manual `to_dict()` method needed.

---

## HTTP Error Handling

### Status Code Mapping

Map HTTP status codes to error categories and handling behavior:

```python
import msgspec


class HTTPErrorMapping(msgspec.Struct, frozen=True):
    """How to handle an HTTP status code."""

    category: ErrorCategory
    severity: ErrorSeverity
    should_retry: bool = False
    should_fallback: bool = True
    retry_after_header: bool = False  # Check Retry-After header


HTTP_ERROR_MAPPINGS: dict[int, HTTPErrorMapping] = {
    # Authentication errors
    401: HTTPErrorMapping(
        category=ErrorCategory.AUTHENTICATION,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),

    # Authorization errors
    403: HTTPErrorMapping(
        category=ErrorCategory.AUTHORIZATION,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),

    # Not found
    404: HTTPErrorMapping(
        category=ErrorCategory.NOT_FOUND,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),

    # Rate limiting
    429: HTTPErrorMapping(
        category=ErrorCategory.RATE_LIMITED,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=False,  # Don't try other strategies, they'll be limited too
        retry_after_header=True,
    ),

    # Server errors
    500: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    502: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    503: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    504: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
}


def classify_http_error(status_code: int, response_body: str | None = None) -> HTTPErrorMapping:
    """Classify an HTTP error by status code."""
    if status_code in HTTP_ERROR_MAPPINGS:
        return HTTP_ERROR_MAPPINGS[status_code]

    # Default mapping for unrecognized codes
    if 400 <= status_code < 500:
        return HTTPErrorMapping(
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
            should_fallback=True,
        )
    elif 500 <= status_code < 600:
        return HTTPErrorMapping(
            category=ErrorCategory.PROVIDER,
            severity=ErrorSeverity.TRANSIENT,
            should_retry=True,
            should_fallback=True,
        )
    else:
        return HTTPErrorMapping(
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
        )
```

### HTTP Error Handler

```python
import httpx
from typing import Callable, TypeVar

T = TypeVar('T')

async def handle_http_request(
    client: httpx.AsyncClient,
    method: str,
    url: str,
    *,
    max_retries: int = 3,
    base_delay: float = 1.0,
    headers: dict | None = None,
    json: dict | None = None,
    on_retry: Callable[[int, float], None] | None = None,
) -> httpx.Response:
    """
    Make an HTTP request with automatic retry for transient errors.

    Args:
        client: HTTP client
        method: HTTP method
        url: Request URL
        max_retries: Maximum retry attempts
        base_delay: Base delay for exponential backoff
        headers: Request headers
        json: JSON body
        on_retry: Callback for retry notification

    Returns:
        HTTP response

    Raises:
        HTTPError: If all retries exhausted or non-retryable error
    """
    last_error: httpx.HTTPStatusError | None = None

    for attempt in range(max_retries + 1):
        try:
            response = await client.request(
                method, url, headers=headers, json=json
            )
            response.raise_for_status()
            return response

        except httpx.HTTPStatusError as e:
            last_error = e
            mapping = classify_http_error(e.response.status_code)

            if not mapping.should_retry or attempt >= max_retries:
                raise

            # Calculate delay
            delay = base_delay * (2 ** attempt)

            # Check Retry-After header
            if mapping.retry_after_header:
                retry_after = e.response.headers.get("Retry-After")
                if retry_after:
                    try:
                        delay = max(delay, float(retry_after))
                    except ValueError:
                        pass  # Ignore invalid header

            if on_retry:
                on_retry(attempt + 1, delay)

            await asyncio.sleep(delay)

        except (httpx.ConnectError, httpx.TimeoutException) as e:
            last_error = e

            if attempt >= max_retries:
                raise

            delay = base_delay * (2 ** attempt)
            if on_retry:
                on_retry(attempt + 1, delay)

            await asyncio.sleep(delay)

    # Should not reach here, but just in case
    if last_error:
        raise last_error
    raise RuntimeError("Unexpected state in retry loop")
```

---

## Network Error Handling

### Connection Errors

```python
import httpx

def classify_network_error(error: Exception) -> VibeusageError:
    """Classify network-related errors."""

    if isinstance(error, httpx.ConnectTimeout):
        return VibeusageError(
            message="Connection timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="Check your internet connection and try again.",
        )

    if isinstance(error, httpx.ReadTimeout):
        return VibeusageError(
            message="Request timed out waiting for response",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="The provider may be slow. Try again or check provider status.",
        )

    if isinstance(error, httpx.ConnectError):
        return VibeusageError(
            message="Failed to connect to server",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="Check your internet connection. The provider may be down.",
        )

    if isinstance(error, httpx.HTTPStatusError):
        return classify_http_status_error(error)

    return VibeusageError(
        message=f"Network error: {error}",
        category=ErrorCategory.NETWORK,
        severity=ErrorSeverity.TRANSIENT,
        remediation="Check your internet connection and try again.",
    )


def classify_http_status_error(error: httpx.HTTPStatusError) -> VibeusageError:
    """Classify HTTP status errors into structured errors."""

    status = error.response.status_code
    mapping = classify_http_error(status)

    # Try to extract error message from response
    try:
        body = error.response.json()
        detail = body.get("error", body.get("message", str(status)))
    except Exception:
        detail = error.response.text[:200] if error.response.text else str(status)

    message = f"HTTP {status}: {detail}"

    return VibeusageError(
        message=message,
        category=mapping.category,
        severity=mapping.severity,
        details={"status_code": status, "response": detail},
    )
```

### Timeout Configuration

```python
from vibeusage.config import get_config

def get_timeout_config() -> httpx.Timeout:
    """Get timeout configuration from settings."""
    config = get_config()
    timeout_seconds = config.fetch.timeout

    return httpx.Timeout(
        connect=10.0,           # Connection timeout
        read=timeout_seconds,   # Read timeout (configurable)
        write=10.0,             # Write timeout
        pool=5.0,               # Pool timeout
    )
```

---

## Authentication Fallback Chains

### Strategy Fallback

When an auth strategy fails, try the next one in the chain:

```python
import msgspec
from vibeusage.strategies.base import FetchStrategy, FetchResult


class FetchAttempt(msgspec.Struct):
    """Record of a fetch attempt for debugging."""

    strategy: str
    success: bool
    error: VibeusageError | None = None
    duration_ms: int = 0


class FetchPipelineResult(msgspec.Struct):
    """Result of executing a fetch pipeline."""

    success: bool
    snapshot: UsageSnapshot | None = None
    source: str | None = None
    attempts: list[FetchAttempt] = msgspec.field(default_factory=list)
    final_error: VibeusageError | None = None


async def execute_fetch_pipeline(
    provider_id: str,
    strategies: list[FetchStrategy],
) -> FetchPipelineResult:
    """
    Execute fetch strategies in order until one succeeds.

    Strategies are tried in priority order. A strategy can signal:
    - Success: Stop and return result
    - Recoverable failure: Try next strategy
    - Fatal failure: Stop immediately (e.g., rate limit)
    """
    attempts = []

    for strategy in strategies:
        # Check if strategy is available (fast check, no network)
        if not await strategy.is_available():
            attempts.append(FetchAttempt(
                strategy=strategy.name,
                success=False,
                error=VibeusageError(
                    message=f"Strategy '{strategy.name}' not available",
                    category=ErrorCategory.CONFIGURATION,
                    severity=ErrorSeverity.RECOVERABLE,
                    provider=provider_id,
                ),
            ))
            continue

        # Execute strategy
        start = time.monotonic()
        try:
            result = await asyncio.wait_for(
                strategy.fetch(),
                timeout=get_config().fetch.timeout,
            )
        except asyncio.TimeoutError:
            error = VibeusageError(
                message=f"Strategy '{strategy.name}' timed out",
                category=ErrorCategory.NETWORK,
                severity=ErrorSeverity.TRANSIENT,
                provider=provider_id,
                remediation="Try again or check your network connection.",
            )
            attempts.append(FetchAttempt(
                strategy=strategy.name,
                success=False,
                error=error,
                duration_ms=int((time.monotonic() - start) * 1000),
            ))
            continue

        except Exception as e:
            error = classify_exception(e, provider_id)
            attempts.append(FetchAttempt(
                strategy=strategy.name,
                success=False,
                error=error,
                duration_ms=int((time.monotonic() - start) * 1000),
            ))
            continue

        duration_ms = int((time.monotonic() - start) * 1000)

        if result.success:
            attempts.append(FetchAttempt(
                strategy=strategy.name,
                success=True,
                duration_ms=duration_ms,
            ))
            return FetchPipelineResult(
                success=True,
                snapshot=result.snapshot,
                source=strategy.name,
                attempts=attempts,
            )

        # Strategy failed
        error = VibeusageError(
            message=result.error or f"Strategy '{strategy.name}' failed",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )
        attempts.append(FetchAttempt(
            strategy=strategy.name,
            success=False,
            error=error,
            duration_ms=duration_ms,
        ))

        # Check if we should stop (fatal error)
        if not result.should_fallback:
            break

    # All strategies failed
    final_error = attempts[-1].error if attempts else VibeusageError(
        message="No fetch strategies available",
        category=ErrorCategory.CONFIGURATION,
        severity=ErrorSeverity.FATAL,
        provider=provider_id,
        remediation=f"Run 'vibeusage auth {provider_id}' to configure credentials.",
    )

    return FetchPipelineResult(
        success=False,
        attempts=attempts,
        final_error=final_error,
    )
```

### Auth Error Messages

Provider-specific error messages with remediation:

```python
AUTH_ERROR_TEMPLATES: dict[str, dict[str, str]] = {
    "claude": {
        "401": (
            "Claude session expired or invalid.\n"
            "Run: vibeusage auth claude"
        ),
        "403": (
            "Access denied. Your account may not have a Claude Max subscription.\n"
            "Check your subscription at: https://claude.ai/settings/billing"
        ),
        "no_credentials": (
            "No Claude credentials found.\n"
            "Run 'vibeusage auth claude' to configure authentication."
        ),
        "cli_not_found": (
            "Claude CLI not found in PATH.\n"
            "Install it from: https://claude.ai/download"
        ),
    },
    "codex": {
        "401": (
            "Codex session expired or invalid.\n"
            "Run: vibeusage auth codex"
        ),
        "403": (
            "Access denied. Your account may not have a ChatGPT Plus/Pro subscription.\n"
            "Check your subscription at: https://chatgpt.com/settings/subscription"
        ),
        "no_credentials": (
            "No Codex credentials found.\n"
            "Run 'vibeusage auth codex' to configure authentication."
        ),
    },
    "copilot": {
        "401": (
            "GitHub token expired.\n"
            "Run: vibeusage auth copilot"
        ),
        "403": (
            "GitHub Copilot not enabled for this account.\n"
            "Enable it at: https://github.com/settings/copilot"
        ),
        "no_credentials": (
            "No Copilot credentials found.\n"
            "Run 'vibeusage auth copilot' to authenticate with GitHub."
        ),
    },
    "cursor": {
        "401": (
            "Cursor session expired.\n"
            "Log into cursor.com in your browser, then run:\n"
            "  vibeusage auth cursor"
        ),
        "no_credentials": (
            "No Cursor session found.\n"
            "Log into cursor.com in your browser first."
        ),
    },
    "gemini": {
        "401": (
            "Gemini credentials expired.\n"
            "Run: vibeusage auth gemini"
        ),
        "403": (
            "Gemini quota exceeded or access denied.\n"
            "Check your usage at: https://aistudio.google.com/app/usage"
        ),
    },
}


def get_auth_error_message(
    provider_id: str,
    error_type: str,
) -> str:
    """Get provider-specific auth error message."""
    templates = AUTH_ERROR_TEMPLATES.get(provider_id, {})
    return templates.get(error_type, f"Authentication error for {provider_id}.")
```

---

## Stale Data Handling

### Displaying Cached Data on Failure

When fresh data can't be fetched, show cached data with a warning:

```python
from vibeusage.config.cache import load_cached_snapshot, is_snapshot_fresh
from vibeusage.models import UsageSnapshot


class FetchResultWithFallback(msgspec.Struct):
    """Fetch result that may include stale cached data."""

    snapshot: UsageSnapshot | None
    is_fresh: bool
    error: VibeusageError | None = None
    stale_age_minutes: int | None = None


async def fetch_with_cache_fallback(
    provider_id: str,
) -> FetchResultWithFallback:
    """
    Fetch usage data, falling back to cache on failure.

    Returns fresh data if fetch succeeds, otherwise returns
    cached data (if available) with stale indicator.
    """
    # Try fresh fetch
    from vibeusage.providers import get_provider

    provider = get_provider(provider_id)
    pipeline_result = await execute_fetch_pipeline(
        provider_id,
        provider.fetch_strategies(),
    )

    if pipeline_result.success and pipeline_result.snapshot:
        # Success - cache and return fresh data
        cache_snapshot(pipeline_result.snapshot)
        return FetchResultWithFallback(
            snapshot=pipeline_result.snapshot,
            is_fresh=True,
        )

    # Fetch failed - try cache
    cached = load_cached_snapshot(provider_id)
    if cached:
        age = datetime.now(cached.fetched_at.tzinfo) - cached.fetched_at
        age_minutes = int(age.total_seconds() / 60)

        return FetchResultWithFallback(
            snapshot=cached,
            is_fresh=False,
            error=pipeline_result.final_error,
            stale_age_minutes=age_minutes,
        )

    # No cache available
    return FetchResultWithFallback(
        snapshot=None,
        is_fresh=False,
        error=pipeline_result.final_error,
    )
```

### Stale Data CLI Display

```python
from rich.console import Console
from rich.panel import Panel

console = Console()

def display_with_staleness(result: FetchResultWithFallback) -> None:
    """Display usage data with staleness warning if applicable."""

    if result.snapshot is None:
        # Complete failure
        show_error(result.error)
        return

    if not result.is_fresh:
        # Show stale warning
        age = result.stale_age_minutes or 0
        if age < 60:
            age_str = f"{age} minute{'s' if age != 1 else ''}"
        else:
            hours = age // 60
            age_str = f"{hours} hour{'s' if hours != 1 else ''}"

        console.print(f"[yellow]Showing cached data from {age_str} ago[/yellow]")

        if result.error:
            console.print(f"[dim]Fetch failed: {result.error.message}[/dim]")

        console.print()

    # Display the snapshot (fresh or stale)
    from vibeusage.cli.display import UsageDisplay
    console.print(UsageDisplay(result.snapshot))
```

---

## Failure Gates

### Preventing Retry Flapping

Track failures to avoid spamming retries on persistent errors:

```python
from datetime import datetime, timedelta
from collections import defaultdict

import msgspec


class FailureRecord(msgspec.Struct):
    """Record of a failure for gate tracking."""

    timestamp: datetime
    error_category: ErrorCategory
    message: str


class FailureGate:
    """
    Track failures to prevent excessive retries.

    After consecutive failures, temporarily "gate" (block) requests
    to allow the provider to recover.
    """

    # Configurable thresholds
    MAX_CONSECUTIVE_FAILURES = 3
    GATE_DURATION = timedelta(minutes=5)
    WINDOW_DURATION = timedelta(minutes=10)

    def __init__(self):
        self._failures: dict[str, list[FailureRecord]] = defaultdict(list)
        self._gated_until: dict[str, datetime] = {}

    def record_failure(
        self,
        provider_id: str,
        error: VibeusageError,
    ) -> None:
        """Record a failure for a provider."""
        now = datetime.now().astimezone()

        # Clean old failures outside window
        self._failures[provider_id] = [
            f for f in self._failures[provider_id]
            if now - f.timestamp < self.WINDOW_DURATION
        ]

        # Record new failure
        self._failures[provider_id].append(FailureRecord(
            timestamp=now,
            error_category=error.category,
            message=error.message,
        ))

        # Check if we should gate
        if len(self._failures[provider_id]) >= self.MAX_CONSECUTIVE_FAILURES:
            self._gated_until[provider_id] = now + self.GATE_DURATION

    def record_success(self, provider_id: str) -> None:
        """Record a success, resetting failure tracking."""
        self._failures[provider_id].clear()
        self._gated_until.pop(provider_id, None)

    def is_gated(self, provider_id: str) -> bool:
        """Check if a provider is currently gated."""
        until = self._gated_until.get(provider_id)
        if until is None:
            return False

        if datetime.now().astimezone() >= until:
            # Gate expired
            self._gated_until.pop(provider_id, None)
            return False

        return True

    def gate_remaining(self, provider_id: str) -> timedelta | None:
        """Get remaining gate duration, if gated."""
        until = self._gated_until.get(provider_id)
        if until is None:
            return None

        remaining = until - datetime.now().astimezone()
        if remaining.total_seconds() <= 0:
            return None

        return remaining

    def recent_failures(self, provider_id: str) -> list[FailureRecord]:
        """Get recent failures for diagnostics."""
        now = datetime.now().astimezone()
        return [
            f for f in self._failures[provider_id]
            if now - f.timestamp < self.WINDOW_DURATION
        ]


# Global failure gate instance
_failure_gate = FailureGate()


def get_failure_gate() -> FailureGate:
    """Get global failure gate."""
    return _failure_gate
```

### Gated Fetch Flow

```python
async def fetch_with_gate(provider_id: str) -> FetchResultWithFallback:
    """
    Fetch with failure gate protection.

    If provider is gated due to recent failures, return cached data
    immediately without attempting a fetch.
    """
    gate = get_failure_gate()

    # Check if gated
    if gate.is_gated(provider_id):
        remaining = gate.gate_remaining(provider_id)
        cached = load_cached_snapshot(provider_id)

        return FetchResultWithFallback(
            snapshot=cached,
            is_fresh=False,
            error=VibeusageError(
                message=f"Provider temporarily unavailable (retry in {format_timedelta(remaining)})",
                category=ErrorCategory.RATE_LIMITED,
                severity=ErrorSeverity.TRANSIENT,
                provider=provider_id,
                remediation="Recent failures detected. Waiting before retry.",
            ),
            stale_age_minutes=calculate_age_minutes(cached) if cached else None,
        )

    # Attempt fetch
    result = await fetch_with_cache_fallback(provider_id)

    # Update gate
    if result.is_fresh:
        gate.record_success(provider_id)
    elif result.error:
        gate.record_failure(provider_id, result.error)

    return result
```

---

## Partial Failure Handling

### Multi-Provider Results

When fetching from multiple providers, some may succeed and others fail:

```python
class MultiProviderResult(msgspec.Struct):
    """Aggregated result from fetching multiple providers."""

    successes: dict[str, UsageSnapshot]   # provider_id -> snapshot
    failures: dict[str, VibeusageError]   # provider_id -> error
    stale: dict[str, UsageSnapshot]       # provider_id -> stale cached snapshot

    @property
    def has_any_data(self) -> bool:
        """Check if we have any data to display."""
        return bool(self.successes) or bool(self.stale)

    @property
    def all_failed(self) -> bool:
        """Check if all providers failed."""
        return not self.successes and not self.stale


async def fetch_all_with_partial_failure(
    provider_ids: list[str],
) -> MultiProviderResult:
    """
    Fetch from all providers, collecting partial results.

    Providers are fetched concurrently. Failures for one provider
    don't affect others.
    """
    successes = {}
    failures = {}
    stale = {}

    async def fetch_one(provider_id: str) -> None:
        result = await fetch_with_gate(provider_id)

        if result.is_fresh and result.snapshot:
            successes[provider_id] = result.snapshot
        elif result.snapshot:
            stale[provider_id] = result.snapshot
            if result.error:
                failures[provider_id] = result.error
        elif result.error:
            failures[provider_id] = result.error

    # Fetch concurrently with semaphore
    config = get_config()
    semaphore = asyncio.Semaphore(config.fetch.max_concurrent)

    async def fetch_with_limit(provider_id: str) -> None:
        async with semaphore:
            await fetch_one(provider_id)

    await asyncio.gather(*[fetch_with_limit(p) for p in provider_ids])

    return MultiProviderResult(
        successes=successes,
        failures=failures,
        stale=stale,
    )
```

### Displaying Partial Results

```python
def display_multi_provider_result(result: MultiProviderResult) -> None:
    """Display results from multiple providers, handling partial failures."""

    # Display successful/stale results
    all_snapshots = {**result.successes, **result.stale}

    for provider_id, snapshot in all_snapshots.items():
        is_stale = provider_id in result.stale

        if is_stale:
            console.print(f"[yellow]{provider_id.title()} (cached)[/yellow]")

        from vibeusage.cli.display import ProviderPanel
        console.print(ProviderPanel(snapshot))
        console.print()

    # Show failures summary
    if result.failures:
        failed_only = {
            pid: err for pid, err in result.failures.items()
            if pid not in result.stale  # Don't show failure if we have stale data
        }

        if failed_only:
            console.print("[dim]Some providers failed:[/dim]")
            for provider_id, error in failed_only.items():
                console.print(f"  [red]{provider_id}[/red]: {error.message}")
            console.print()
```

---

## Retry Logic

### Exponential Backoff

```python
import random


class RetryConfig(msgspec.Struct):
    """Configuration for retry behavior."""

    max_attempts: int = 3
    base_delay: float = 1.0      # Base delay in seconds
    max_delay: float = 30.0      # Maximum delay
    exponential_base: float = 2.0
    jitter: bool = True          # Add randomness to prevent thundering herd


def calculate_retry_delay(
    attempt: int,
    config: RetryConfig,
) -> float:
    """Calculate delay before next retry attempt."""
    delay = config.base_delay * (config.exponential_base ** attempt)
    delay = min(delay, config.max_delay)

    if config.jitter:
        # Add up to 25% jitter
        jitter = delay * 0.25 * random.random()
        delay += jitter

    return delay


async def with_retry(
    func: Callable[[], Awaitable[T]],
    config: RetryConfig | None = None,
    on_retry: Callable[[int, float, Exception], None] | None = None,
) -> T:
    """
    Execute async function with retry logic.

    Args:
        func: Async function to execute
        config: Retry configuration
        on_retry: Callback before each retry (attempt, delay, exception)

    Returns:
        Function result

    Raises:
        Last exception if all retries exhausted
    """
    config = config or RetryConfig()
    last_exception: Exception | None = None

    for attempt in range(config.max_attempts):
        try:
            return await func()
        except Exception as e:
            last_exception = e

            # Check if we should retry
            if not should_retry_exception(e):
                raise

            if attempt >= config.max_attempts - 1:
                raise

            delay = calculate_retry_delay(attempt, config)

            if on_retry:
                on_retry(attempt + 1, delay, e)

            await asyncio.sleep(delay)

    # Should not reach here
    assert last_exception is not None
    raise last_exception


def should_retry_exception(e: Exception) -> bool:
    """Determine if an exception should trigger retry."""
    import httpx

    # Network errors are retryable
    if isinstance(e, (httpx.ConnectError, httpx.TimeoutException)):
        return True

    # HTTP errors depend on status code
    if isinstance(e, httpx.HTTPStatusError):
        mapping = classify_http_error(e.response.status_code)
        return mapping.should_retry

    # Parse errors are not retryable
    if isinstance(e, (json.JSONDecodeError, KeyError, ValueError)):
        return False

    # Unknown errors - don't retry by default
    return False
```

---

## CLI Error Display

### Error Formatting

```python
from rich.console import Console
from rich.panel import Panel
from rich.text import Text

console = Console()

def show_error(error: VibeusageError) -> None:
    """Display a formatted error message."""

    # Color by severity
    colors = {
        ErrorSeverity.FATAL: "red",
        ErrorSeverity.RECOVERABLE: "yellow",
        ErrorSeverity.TRANSIENT: "yellow",
        ErrorSeverity.WARNING: "dim",
    }
    color = colors.get(error.severity, "red")

    # Build message content
    content = Text()
    content.append(error.message, style=color)

    if error.remediation:
        content.append("\n\n")
        content.append(error.remediation, style="dim")

    # Title includes provider if available
    title = "Error"
    if error.provider:
        title = f"{error.provider.title()} Error"

    console.print(Panel(content, title=title, border_style=color))


def show_partial_failures(failures: dict[str, VibeusageError]) -> None:
    """Show summary of partial failures."""
    if not failures:
        return

    console.print()
    console.print("[dim]─── Errors ───[/dim]")

    for provider_id, error in failures.items():
        console.print(f"[red]{provider_id}[/red]: {error.message}")
        if error.remediation:
            console.print(f"  [dim]{error.remediation}[/dim]")
```

### Exit Codes

```python
class ExitCode:
    """CLI exit codes."""

    SUCCESS = 0           # All operations succeeded
    GENERAL_ERROR = 1     # Unspecified error
    AUTH_ERROR = 2        # Authentication/authorization failed
    NETWORK_ERROR = 3     # Network connectivity issues
    CONFIG_ERROR = 4      # Configuration/input invalid
    PARTIAL_FAILURE = 5   # Some providers failed (but have some data)


def exit_code_for_error(error: VibeusageError) -> int:
    """Map error to appropriate exit code."""
    category_codes = {
        ErrorCategory.AUTHENTICATION: ExitCode.AUTH_ERROR,
        ErrorCategory.AUTHORIZATION: ExitCode.AUTH_ERROR,
        ErrorCategory.NETWORK: ExitCode.NETWORK_ERROR,
        ErrorCategory.CONFIGURATION: ExitCode.CONFIG_ERROR,
        ErrorCategory.RATE_LIMITED: ExitCode.NETWORK_ERROR,
        ErrorCategory.PROVIDER: ExitCode.GENERAL_ERROR,
        ErrorCategory.PARSE: ExitCode.GENERAL_ERROR,
        ErrorCategory.NOT_FOUND: ExitCode.GENERAL_ERROR,
        ErrorCategory.UNKNOWN: ExitCode.GENERAL_ERROR,
    }
    return category_codes.get(error.category, ExitCode.GENERAL_ERROR)


def exit_code_for_result(result: MultiProviderResult) -> int:
    """Determine exit code for multi-provider result."""
    if result.all_failed:
        # Pick the "worst" error
        if result.failures:
            first_error = next(iter(result.failures.values()))
            return exit_code_for_error(first_error)
        return ExitCode.GENERAL_ERROR

    if result.failures and not result.successes:
        # Only have stale data
        return ExitCode.PARTIAL_FAILURE

    if result.failures:
        # Some succeeded, some failed
        return ExitCode.PARTIAL_FAILURE

    return ExitCode.SUCCESS
```

---

## JSON Error Output

### Structured Error Response

With msgspec, JSON serialization is automatic. No manual `to_dict()` methods needed.

```python
import msgspec


class ErrorResponse(msgspec.Struct):
    """Wrapper for error JSON output."""

    error: VibeusageError


class MultiProviderResponse(msgspec.Struct):
    """JSON output structure for multi-provider results."""

    providers: dict[str, UsageSnapshot]
    errors: dict[str, VibeusageError]
    stale: list[str]


def format_json_error(error: VibeusageError) -> bytes:
    """Format error for JSON output using msgspec."""
    return msgspec.json.encode(ErrorResponse(error=error))


def format_json_result(result: MultiProviderResult) -> bytes:
    """Format multi-provider result for JSON output."""
    response = MultiProviderResponse(
        providers={**result.successes, **result.stale},
        errors=result.failures,
        stale=list(result.stale.keys()),
    )
    return msgspec.json.encode(response)
```

---

## Exception Classification

### Classify Any Exception

```python
def classify_exception(
    e: Exception,
    provider_id: str | None = None,
) -> VibeusageError:
    """Classify any exception into a structured error."""
    import httpx

    # Network errors
    if isinstance(e, httpx.TimeoutException):
        return VibeusageError(
            message="Request timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Check your network connection and try again.",
        )

    if isinstance(e, httpx.ConnectError):
        return VibeusageError(
            message="Failed to connect to server",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Check your internet connection. The provider may be down.",
        )

    if isinstance(e, httpx.HTTPStatusError):
        return classify_http_status_error(e)

    # Parse errors
    if isinstance(e, json.JSONDecodeError):
        return VibeusageError(
            message="Failed to parse response",
            category=ErrorCategory.PARSE,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
            details={"error": str(e)},
        )

    if isinstance(e, (KeyError, ValueError, TypeError)):
        return VibeusageError(
            message=f"Invalid response format: {e}",
            category=ErrorCategory.PARSE,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    # Async errors
    if isinstance(e, asyncio.TimeoutError):
        return VibeusageError(
            message="Operation timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Try again. If the issue persists, check provider status.",
        )

    if isinstance(e, asyncio.CancelledError):
        return VibeusageError(
            message="Operation cancelled",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    # File errors
    if isinstance(e, FileNotFoundError):
        return VibeusageError(
            message=f"File not found: {e.filename}",
            category=ErrorCategory.CONFIGURATION,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    if isinstance(e, PermissionError):
        return VibeusageError(
            message=f"Permission denied: {e.filename}",
            category=ErrorCategory.CONFIGURATION,
            severity=ErrorSeverity.FATAL,
            provider=provider_id,
            remediation="Check file permissions for vibeusage config directory.",
        )

    # Unknown
    return VibeusageError(
        message=str(e),
        category=ErrorCategory.UNKNOWN,
        severity=ErrorSeverity.RECOVERABLE,
        provider=provider_id,
        details={"type": type(e).__name__},
    )
```

---

## Diagnostic Information

### Verbose Error Output

```python
def show_verbose_error(
    error: VibeusageError,
    attempts: list[FetchAttempt] | None = None,
) -> None:
    """Show detailed error information for debugging."""

    # Basic error
    show_error(error)

    if not attempts:
        return

    # Show attempt history
    console.print()
    console.print("[dim]Fetch Attempts:[/dim]")

    for attempt in attempts:
        status = "[green]success[/green]" if attempt.success else "[red]failed[/red]"
        line = f"  {attempt.strategy}: {status}"

        if attempt.duration_ms:
            line += f" ({attempt.duration_ms}ms)"

        console.print(line)

        if attempt.error and not attempt.success:
            console.print(f"    [dim]{attempt.error.message}[/dim]")


def show_diagnostic_info() -> None:
    """Show diagnostic information for troubleshooting."""
    from vibeusage import __version__
    from vibeusage.config import config_dir, cache_dir
    import platform
    import sys

    console.print("[bold]Diagnostic Information[/bold]")
    console.print()

    console.print(f"vibeusage version: {__version__}")
    console.print(f"Python version: {sys.version}")
    console.print(f"Platform: {platform.platform()}")
    console.print()

    console.print(f"Config directory: {config_dir()}")
    console.print(f"Cache directory: {cache_dir()}")
    console.print()

    # Show credential status
    from vibeusage.config.credentials import check_provider_credentials
    from vibeusage.providers import list_provider_ids

    console.print("[dim]Credential Status:[/dim]")
    for provider_id in list_provider_ids():
        status, source = check_provider_credentials(provider_id)
        console.print(f"  {provider_id}: {status} ({source or 'none'})")

    console.print()

    # Show failure gate status
    gate = get_failure_gate()
    console.print("[dim]Failure Gate Status:[/dim]")
    for provider_id in list_provider_ids():
        if gate.is_gated(provider_id):
            remaining = gate.gate_remaining(provider_id)
            console.print(f"  {provider_id}: [red]gated[/red] ({format_timedelta(remaining)} remaining)")
        else:
            failures = gate.recent_failures(provider_id)
            if failures:
                console.print(f"  {provider_id}: {len(failures)} recent failures")
            else:
                console.print(f"  {provider_id}: [green]ok[/green]")
```

---

## Implementation Checklist

- [ ] `vibeusage/errors/__init__.py` - Module exports
- [ ] `vibeusage/errors/types.py` - ErrorCategory, ErrorSeverity, VibeusageError
- [ ] `vibeusage/errors/http.py` - HTTP error classification and handling
- [ ] `vibeusage/errors/network.py` - Network error handling
- [ ] `vibeusage/errors/messages.py` - Provider-specific error messages
- [ ] `vibeusage/errors/classify.py` - Exception classification
- [ ] `vibeusage/core/gate.py` - FailureGate implementation
- [ ] `vibeusage/core/retry.py` - Retry logic with backoff
- [ ] `vibeusage/core/fetch.py` - Fetch pipeline with fallbacks
- [ ] `vibeusage/cli/errors.py` - CLI error display
- [ ] Integration tests for error scenarios

---

## Testing Error Handling

### Unit Tests

```python
import pytest
from vibeusage.errors import classify_http_error, ErrorCategory

def test_classify_401():
    mapping = classify_http_error(401)
    assert mapping.category == ErrorCategory.AUTHENTICATION
    assert mapping.should_fallback is True

def test_classify_429():
    mapping = classify_http_error(429)
    assert mapping.category == ErrorCategory.RATE_LIMITED
    assert mapping.should_retry is True
    assert mapping.should_fallback is False


@pytest.mark.asyncio
async def test_retry_backoff():
    attempts = []

    async def failing_func():
        attempts.append(1)
        raise httpx.ConnectError("test")

    with pytest.raises(httpx.ConnectError):
        await with_retry(failing_func, RetryConfig(max_attempts=3))

    assert len(attempts) == 3
```

### Integration Tests

```python
@pytest.mark.asyncio
async def test_fetch_with_cache_fallback(mock_provider):
    # First call succeeds, caches result
    result1 = await fetch_with_cache_fallback("test_provider")
    assert result1.is_fresh

    # Simulate failure
    mock_provider.simulate_failure(401)

    # Second call falls back to cache
    result2 = await fetch_with_cache_fallback("test_provider")
    assert not result2.is_fresh
    assert result2.snapshot is not None
    assert result2.error is not None
```

---

## Open Questions

1. **Retry limits per session**: Should we track total retries per CLI invocation to prevent excessive attempts across all providers?

2. **Error telemetry**: Should we support optional anonymous error reporting to help improve the tool?

3. **Offline mode**: Should we support an explicit `--offline` flag that only uses cached data without attempting fetches?

4. **Custom error handlers**: Should users be able to configure custom error handling (e.g., webhook on failure)?

5. **Error aggregation**: When multiple strategies fail with different errors, which error should be surfaced to the user?

## Implementation Notes

- Use `msgspec.Struct` for all error types - provides native JSON serialization
- Use structured errors throughout the codebase, never raise raw exceptions
- All user-facing error messages should include remediation steps
- Log detailed error information at DEBUG level for troubleshooting
- Test error paths as thoroughly as success paths
- Consider using `structlog` for structured logging of errors
- The failure gate should persist across CLI invocations (perhaps via cache file)
- Use `msgspec.json.encode()` for JSON error output - no manual serialization needed
