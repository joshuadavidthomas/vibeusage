"""Orchestration for multi-provider fetch operations."""

from __future__ import annotations

import asyncio
from collections import defaultdict

from vibeusage.config.settings import get_config
from vibeusage.core.aggregate import AggregatedResult
from vibeusage.core.aggregate import aggregate_results
from vibeusage.core.fetch import execute_fetch_pipeline
from vibeusage.strategies.base import FetchOutcome


async def fetch_single_provider(
    provider_id: str,
    strategies: list,
    on_complete: callable | None = None,
) -> FetchOutcome:
    """Fetch usage data from a single provider.

    Args:
        provider_id: Provider identifier
        strategies: List of fetch strategies to try
        on_complete: Optional callback called with outcome after fetch

    Returns:
        FetchOutcome with result or error
    """
    outcome = await execute_fetch_pipeline(provider_id, strategies)

    if on_complete:
        on_complete(outcome)

    return outcome


async def fetch_all_providers(
    provider_map: dict[str, list],
    on_complete: callable | None = None,
) -> dict[str, FetchOutcome]:
    """Fetch usage data from all providers concurrently.

    Args:
        provider_map: Dict of provider_id to list of fetch strategies
        on_complete: Optional callback called with each outcome after fetch

    Returns:
        Dict of provider_id to FetchOutcome
    """
    config = get_config()
    max_concurrent = config.fetch.max_concurrent

    outcomes: dict[str, FetchOutcome] = {}

    async def fetch_with_callback(provider_id: str, strategies: list):
        outcome = await execute_fetch_pipeline(provider_id, strategies)
        outcomes[provider_id] = outcome
        if on_complete:
            on_complete(outcome)

    # Create tasks for all providers
    tasks = [
        fetch_with_callback(pid, strategies) for pid, strategies in provider_map.items()
    ]

    # Run with semaphore for concurrency control
    semaphore = asyncio.Semaphore(max_concurrent)

    async def bounded_task(task):
        async with semaphore:
            return await task

    bounded_tasks = [bounded_task(t) for t in tasks]
    await asyncio.gather(*bounded_tasks, return_exceptions=True)

    return outcomes


async def fetch_enabled_providers(
    provider_map: dict[str, list],
    on_complete: callable | None = None,
) -> dict[str, FetchOutcome]:
    """Fetch only enabled providers based on config.

    Args:
        provider_map: Dict of provider_id to list of fetch strategies
        on_complete: Optional callback called with each outcome after fetch

    Returns:
        Dict of provider_id to FetchOutcome for enabled providers
    """
    config = get_config()
    enabled_map = {
        pid: strategies
        for pid, strategies in provider_map.items()
        if config.is_provider_enabled(pid)
    }

    return await fetch_all_providers(enabled_map, on_complete)


def categorize_results(outcomes: dict[str, FetchOutcome]) -> dict[str, list]:
    """Categorize outcomes by result type.

    Returns dict with keys: 'success', 'failure', 'cached', 'gated'
    """
    categories = defaultdict(list)

    for provider_id, outcome in outcomes.items():
        if outcome.gated:
            categories["gated"].append(provider_id)
        elif outcome.success:
            if outcome.cached:
                categories["cached"].append(provider_id)
            else:
                categories["success"].append(provider_id)
        else:
            categories["failure"].append(provider_id)

    return dict(categories)


async def fetch_with_partial_failure_handling(
    provider_map: dict[str, list],
) -> tuple[AggregatedResult, dict[str, list]]:
    """Fetch all providers and handle partial failures gracefully.

    Returns:
        Tuple of (AggregatedResult, categorized_results)
    """
    outcomes = await fetch_enabled_providers(provider_map)
    aggregated = aggregate_results(outcomes)
    categories = categorize_results(outcomes)

    return aggregated, categories
