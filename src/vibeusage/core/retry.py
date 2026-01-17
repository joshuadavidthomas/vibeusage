"""Retry logic with exponential backoff for vibeusage."""

import asyncio
import random
from dataclasses import dataclass
from datetime import timedelta

import httpx


@dataclass(frozen=True)
class RetryConfig:
    """Configuration for retry behavior."""

    max_attempts: int = 3
    base_delay: float = 1.0  # seconds
    max_delay: float = 60.0  # seconds
    exponential_base: float = 2.0
    jitter: bool = True


def calculate_retry_delay(
    attempt: int,
    config: RetryConfig,
) -> float:
    """Calculate delay before next retry using exponential backoff.

    Args:
        attempt: Which attempt we just completed (0-indexed)
        config: Retry configuration

    Returns:
        Delay in seconds
    """
    # Exponential backoff
    delay = config.base_delay * (config.exponential_base**attempt)

    # Apply jitter if enabled
    if config.jitter:
        # Add up to 25% random jitter
        delay *= 1.0 + (random.random() * 0.25)

    # Cap at max delay
    return min(delay, config.max_delay)


def should_retry_exception(exc: Exception) -> bool:
    """Determine if an exception should trigger a retry.

    Network errors, timeouts, and 5xx errors are retryable.
    """
    # httpx network errors
    if isinstance(exc, httpx.NetworkError):
        return True
    if isinstance(exc, httpx.TimeoutException):
        return True

    # HTTP status errors
    if isinstance(exc, httpx.HTTPStatusError):
        status = exc.response.status_code
        # Retry on 5xx server errors and 429 rate limit
        return status >= 500 or status == 429

    return False


async def with_retry(
    coro_or_factory,
    config: RetryConfig | None = None,
) -> any:
    """Execute a coroutine with retry logic.

    Args:
        coro_or_factory: Coroutine to execute, or a callable that returns a coroutine
                        (useful for retries where a fresh coroutine is needed each attempt)
        config: Retry configuration (uses defaults if None)

    Returns:
        Result of the coroutine

    Raises:
        The last exception if all retries are exhausted
    """
    if config is None:
        config = RetryConfig()

    last_exception = None

    for attempt in range(config.max_attempts):
        # Create/get the coroutine for this attempt
        if callable(coro_or_factory):
            coro = coro_or_factory()
        else:
            coro = coro_or_factory

        try:
            return await coro
        except Exception as e:
            last_exception = e

            # Don't retry if this isn't a retryable error
            if not should_retry_exception(e):
                raise

            # Don't wait after the last attempt
            if attempt < config.max_attempts - 1:
                delay = calculate_retry_delay(attempt, config)
                await asyncio.sleep(delay)

    # All retries exhausted
    if last_exception is not None:
        raise last_exception
