"""Core orchestration and utilities for vibeusage."""

from __future__ import annotations

from vibeusage.core.aggregate import AggregatedResult
from vibeusage.core.aggregate import aggregate_results
from vibeusage.core.fetch import execute_fetch_pipeline
from vibeusage.core.fetch import fetch_with_cache_fallback
from vibeusage.core.fetch import fetch_with_gate
from vibeusage.core.gate import GATE_DURATION
from vibeusage.core.gate import MAX_CONSECUTIVE_FAILURES
from vibeusage.core.gate import WINDOW_DURATION
from vibeusage.core.gate import FailureGate
from vibeusage.core.gate import FailureRecord
from vibeusage.core.gate import clear_gate
from vibeusage.core.gate import gate_path
from vibeusage.core.gate import get_failure_gate
from vibeusage.core.gate import load_gate
from vibeusage.core.gate import save_gate
from vibeusage.core.http import cleanup
from vibeusage.core.http import fetch_url
from vibeusage.core.http import get_http_client
from vibeusage.core.http import get_timeout_config
from vibeusage.core.orchestrator import categorize_results
from vibeusage.core.orchestrator import fetch_all_providers
from vibeusage.core.orchestrator import fetch_enabled_providers
from vibeusage.core.orchestrator import fetch_single_provider
from vibeusage.core.orchestrator import fetch_with_partial_failure_handling
from vibeusage.core.retry import RetryConfig
from vibeusage.core.retry import calculate_retry_delay
from vibeusage.core.retry import should_retry_exception
from vibeusage.core.retry import with_retry

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
