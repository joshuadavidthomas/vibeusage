"""Fetch pipeline for executing provider fetch strategies."""
from __future__ import annotations

import asyncio
import time

from vibeusage.config.cache import cache_snapshot as save_snapshot
from vibeusage.config.cache import load_cached_snapshot
from vibeusage.config.settings import get_config
from vibeusage.core.gate import get_failure_gate
from vibeusage.core.gate import save_gate
from vibeusage.strategies.base import FetchAttempt
from vibeusage.strategies.base import FetchOutcome
from vibeusage.strategies.base import FetchStrategy


async def execute_fetch_pipeline(
    provider_id: str,
    strategies: list[FetchStrategy],
    use_cache: bool = True,
) -> FetchOutcome:
    """Execute fetch strategies in priority order.

    Tries each strategy in sequence until one succeeds.
    Tracks attempts for debugging.

    Args:
        provider_id: Provider identifier
        strategies: Ordered list of fetch strategies to try
        use_cache: Whether to return cached data as fallback

    Returns:
        FetchOutcome with result or error
    """
    config = get_config()
    gate = get_failure_gate(provider_id)
    attempts: list[FetchAttempt] = []

    # Check if provider is gated
    if gate.is_gated():
        remaining = gate.gate_remaining()
        if remaining:
            # Try to return cached data
            if use_cache:
                cached = load_cached_snapshot(provider_id)
                if cached:
                    return FetchOutcome(
                        provider_id=provider_id,
                        success=True,
                        snapshot=cached,
                        source="cache",
                        attempts=[],
                        cached=True,
                        gate_remaining=remaining,
                    )
            # No cache available - return gated outcome
            return FetchOutcome(
                provider_id=provider_id,
                success=False,
                snapshot=None,
                source=None,
                attempts=attempts,
                error=Exception(f"Provider gated for {remaining}"),
                gated=True,
            )

    # Try each strategy in order
    for strategy in strategies:
        # Check if strategy is available
        if not strategy.is_available():
            attempts.append(
                FetchAttempt(
                    strategy=strategy.name,
                    success=False,
                    error=Exception("Strategy not available"),
                    duration_ms=0,
                )
            )
            continue

        # Execute fetch with timeout
        start_time = time.monotonic()
        try:
            timeout = config.fetch.timeout
            result = await asyncio.wait_for(strategy.fetch(), timeout=timeout)
            duration_ms = (time.monotonic() - start_time) * 1000

            if result.success and result.snapshot:
                # Record success
                gate.record_success()
                save_gate(gate)

                # Save to cache
                save_snapshot(result.snapshot)

                return FetchOutcome(
                    provider_id=provider_id,
                    success=True,
                    snapshot=result.snapshot,
                    source=strategy.name,
                    attempts=attempts,
                )
            elif result.fatal:
                # Fatal error - stop trying strategies
                attempts.append(
                    FetchAttempt(
                        strategy=strategy.name,
                        success=False,
                        error=result.error,
                        duration_ms=duration_ms,
                    )
                )
                return FetchOutcome(
                    provider_id=provider_id,
                    success=False,
                    snapshot=None,
                    source=None,
                    attempts=attempts,
                    error=result.error,
                    fatal=True,
                )
            else:
                # Recoverable error - try next strategy
                attempts.append(
                    FetchAttempt(
                        strategy=strategy.name,
                        success=False,
                        error=result.error,
                        duration_ms=duration_ms,
                    )
                )

        except asyncio.TimeoutError:
            duration_ms = (time.monotonic() - start_time) * 1000
            attempts.append(
                FetchAttempt(
                    strategy=strategy.name,
                    success=False,
                    error=Exception("Fetch timed out"),
                    duration_ms=duration_ms,
                )
            )
            continue
        except Exception as e:
            duration_ms = (time.monotonic() - start_time) * 1000
            attempts.append(
                FetchAttempt(
                    strategy=strategy.name,
                    success=False,
                    error=e,
                    duration_ms=duration_ms,
                )
            )
            continue

    # All strategies failed - record failure
    from vibeusage.errors.classify import classify_exception

    last_error = (
        attempts[-1].error if attempts else Exception("No strategies available")
    )
    classified = classify_exception(last_error)
    gate.record_failure(classified.category, str(last_error))
    save_gate(gate)

    # Try to return cached data as fallback
    if use_cache:
        cached = load_cached_snapshot(provider_id)
        if cached:
            return FetchOutcome(
                provider_id=provider_id,
                success=True,
                snapshot=cached,
                source="cache",
                attempts=attempts,
                cached=True,
            )

    return FetchOutcome(
        provider_id=provider_id,
        success=False,
        snapshot=None,
        source=None,
        attempts=attempts,
        error=last_error,
    )


async def fetch_with_cache_fallback(
    provider_id: str,
    strategies: list[FetchStrategy],
) -> FetchOutcome:
    """Fetch with automatic cache fallback on failure.

    Always returns cached data if available when fresh fetch fails.
    """
    outcome = await execute_fetch_pipeline(provider_id, strategies, use_cache=True)
    return outcome


async def fetch_with_gate(
    provider_id: str,
    strategies: list[FetchStrategy],
) -> FetchOutcome:
    """Fetch respecting failure gate state.

    Returns cached data if provider is gated, otherwise fetches fresh.
    """
    return await execute_fetch_pipeline(provider_id, strategies, use_cache=True)
