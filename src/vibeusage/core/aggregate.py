"""Result aggregation for multi-provider fetches."""

from __future__ import annotations

from dataclasses import dataclass
from dataclasses import field
from datetime import datetime

from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchOutcome


@dataclass(frozen=True)
class AggregatedResult:
    """Result from fetching multiple providers."""

    snapshots: dict[str, UsageSnapshot] = field(default_factory=dict)
    errors: dict[str, Exception] = field(default_factory=dict)
    fetched_at: datetime = field(default_factory=datetime.now)

    def successful_providers(self) -> list[str]:
        """Get list of providers that succeeded."""
        return list(self.snapshots.keys())

    def failed_providers(self) -> list[str]:
        """Get list of providers that failed."""
        return list(self.errors.keys())

    def has_any_data(self) -> bool:
        """Check if any provider returned data."""
        return len(self.snapshots) > 0

    def all_failed(self) -> bool:
        """Check if all providers failed."""
        return len(self.snapshots) == 0 and len(self.errors) > 0


def aggregate_results(outcomes: dict[str, FetchOutcome]) -> AggregatedResult:
    """Aggregate fetch outcomes into a single result.

    Args:
        outcomes: Dict of provider_id to FetchOutcome

    Returns:
        AggregatedResult with snapshots and errors
    """
    snapshots: dict[str, UsageSnapshot] = {}
    errors: dict[str, Exception] = {}

    for provider_id, outcome in outcomes.items():
        if outcome.success and outcome.snapshot:
            snapshots[provider_id] = outcome.snapshot
        elif outcome.error:
            errors[provider_id] = outcome.error

    return AggregatedResult(
        snapshots=snapshots,
        errors=errors,
    )
