"""Fetch strategies for vibeusage."""

from __future__ import annotations

from vibeusage.strategies.base import FetchAttempt
from vibeusage.strategies.base import FetchOutcome
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy

__all__ = [
    "FetchStrategy",
    "FetchResult",
    "FetchAttempt",
    "FetchOutcome",
]
