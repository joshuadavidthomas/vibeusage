"""Core orchestration and utilities for vibeusage."""

from vibeusage.core.aggregate import AggregatedResult, aggregate_results
from vibeusage.core.fetch import (
    execute_fetch_pipeline,
    fetch_with_cache_fallback,
    fetch_with_gate,
)
from vibeusage.core.gate import (
    FailureGate,
    FailureRecord,
    GATE_DURATION,
    MAX_CONSECUTIVE_FAILURES,
    WINDOW_DURATION,
    clear_gate,
    get_failure_gate,
    gate_path,
    load_gate,
    save_gate,
)
from vibeusage.core.http import cleanup, fetch_url, get_http_client, get_timeout_config
from vibeusage.core.orchestrator import (
    categorize_results,
    fetch_all_providers,
    fetch_enabled_providers,
    fetch_single_provider,
    fetch_with_partial_failure_handling,
)
from vibeusage.core.retry import (
    RetryConfig,
    calculate_retry_delay,
    should_retry_exception,
    with_retry,
)

__all__ = [
    # http
    "get_http_client",
    "cleanup",
    "fetch_url",
    "get_timeout_config",
    # retry
    "RetryConfig",
    "calculate_retry_delay",
    "should_retry_exception",
    "with_retry",
    # gate
    "FailureGate",
    "FailureRecord",
    "MAX_CONSECUTIVE_FAILURES",
    "GATE_DURATION",
    "WINDOW_DURATION",
    "get_failure_gate",
    "save_gate",
    "load_gate",
    "clear_gate",
    "gate_path",
    # fetch
    "execute_fetch_pipeline",
    "fetch_with_cache_fallback",
    "fetch_with_gate",
    # aggregate
    "AggregatedResult",
    "aggregate_results",
    # orchestrator
    "fetch_single_provider",
    "fetch_all_providers",
    "fetch_enabled_providers",
    "categorize_results",
    "fetch_with_partial_failure_handling",
]
