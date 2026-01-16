"""Fetch strategy base classes."""

from __future__ import annotations

from abc import ABC, abstractmethod

import msgspec

from vibeusage.models import UsageSnapshot


class FetchResult(msgspec.Struct, frozen=True):
    """Result of a fetch attempt."""

    success: bool
    snapshot: UsageSnapshot | None = None
    error: str | None = None
    should_fallback: bool = True  # Whether to try next strategy

    @classmethod
    def ok(cls, snapshot: UsageSnapshot) -> type[FetchResult]:
        return cls(success=True, snapshot=snapshot, should_fallback=False)

    @classmethod
    def fail(cls, error: str, should_fallback: bool = True) -> type[FetchResult]:
        return cls(success=False, error=error, should_fallback=should_fallback)

    @classmethod
    def fatal(cls, error: str) -> type[FetchResult]:
        """Error that should not trigger fallback (e.g., rate limit)."""
        return cls(success=False, error=error, should_fallback=False)


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
    cached: bool = False  # Whether result came from cache
    gated: bool = False  # Whether provider is failure-gated
    fatal: bool = False  # Whether error was fatal (stop fallback)
    gate_remaining: str | None = None  # Human-readable gate duration


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
