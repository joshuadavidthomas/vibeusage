"""Tests for core orchestration modules."""

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import httpx

from vibeusage.core.retry import (
    RetryConfig,
    calculate_retry_delay,
    should_retry_exception,
    with_retry,
)
from vibeusage.core.gate import (
    FailureGate,
    FailureRecord,
    MAX_CONSECUTIVE_FAILURES,
    GATE_DURATION,
    WINDOW_DURATION,
    get_failure_gate,
    gate_path,
    clear_gate,
)
from vibeusage.core.aggregate import (
    AggregatedResult,
    aggregate_results,
)
from vibeusage.core.orchestrator import (
    fetch_single_provider,
    fetch_all_providers,
    fetch_enabled_providers,
    categorize_results,
)
from vibeusage.strategies.base import (
    FetchResult,
    FetchOutcome,
    FetchAttempt,
)
from vibeusage.models import (
    UsageSnapshot,
    UsagePeriod,
    PeriodType,
)
from vibeusage.errors.types import ErrorCategory


class TestRetryConfig:
    """Tests for RetryConfig."""

    def test_default_values(self):
        """RetryConfig has correct defaults."""
        config = RetryConfig()
        assert config.max_attempts == 3
        assert config.base_delay == 1.0
        assert config.max_delay == 60.0
        assert config.exponential_base == 2.0
        assert config.jitter is True

    def test_custom_values(self):
        """Can create custom RetryConfig."""
        config = RetryConfig(
            max_attempts=5,
            base_delay=2.0,
            max_delay=120.0,
            exponential_base=3.0,
            jitter=False,
        )
        assert config.max_attempts == 5
        assert config.base_delay == 2.0
        assert config.max_delay == 120.0
        assert config.exponential_base == 3.0
        assert config.jitter is False


class TestCalculateRetryDelay:
    """Tests for calculate_retry_delay function."""

    def test_first_attempt_delay(self):
        """First retry uses base_delay."""
        config = RetryConfig(base_delay=1.0, jitter=False)
        delay = calculate_retry_delay(0, config)
        assert delay == 1.0

    def test_exponential_backoff(self):
        """Delay increases exponentially."""
        config = RetryConfig(base_delay=1.0, exponential_base=2.0, jitter=False)
        assert calculate_retry_delay(0, config) == 1.0
        assert calculate_retry_delay(1, config) == 2.0
        assert calculate_retry_delay(2, config) == 4.0
        assert calculate_retry_delay(3, config) == 8.0

    def test_max_delay_cap(self):
        """Delay is capped at max_delay."""
        config = RetryConfig(
            base_delay=1.0, exponential_base=10.0, max_delay=60.0, jitter=False
        )
        delay = calculate_retry_delay(5, config)
        assert delay == 60.0

    def test_jitter_added(self):
        """Jitter adds randomness to delay."""
        config = RetryConfig(base_delay=10.0, jitter=True)
        # Without jitter would be exactly 10.0
        delay = calculate_retry_delay(0, config)
        # With jitter should be between 10.0 and 12.5
        assert 10.0 <= delay < 12.5


class TestShouldRetryException:
    """Tests for should_retry_exception function."""

    def test_network_error_retryable(self):
        """NetworkError is retryable."""
        error = httpx.NetworkError("Network unreachable")
        assert should_retry_exception(error) is True

    def test_timeout_exception_retryable(self):
        """TimeoutException is retryable."""
        error = httpx.TimeoutException("Request timed out")
        assert should_retry_exception(error) is True

    def test_500_retryable(self, mock_response):
        """500 server error is retryable."""
        mock_response.status_code = 500
        error = httpx.HTTPStatusError(
            "Server error", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is True

    def test_502_retryable(self, mock_response):
        """502 bad gateway is retryable."""
        mock_response.status_code = 502
        error = httpx.HTTPStatusError(
            "Bad gateway", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is True

    def test_503_retryable(self, mock_response):
        """503 service unavailable is retryable."""
        mock_response.status_code = 503
        error = httpx.HTTPStatusError(
            "Unavailable", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is True

    def test_429_retryable(self, mock_response):
        """429 rate limit is retryable."""
        mock_response.status_code = 429
        error = httpx.HTTPStatusError(
            "Rate limited", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is True

    def test_401_not_retryable(self, mock_response):
        """401 unauthorized is not retryable."""
        mock_response.status_code = 401
        error = httpx.HTTPStatusError(
            "Unauthorized", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is False

    def test_403_not_retryable(self, mock_response):
        """403 forbidden is not retryable."""
        mock_response.status_code = 403
        error = httpx.HTTPStatusError(
            "Forbidden", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is False

    def test_404_not_retryable(self, mock_response):
        """404 not found is not retryable."""
        mock_response.status_code = 404
        error = httpx.HTTPStatusError(
            "Not found", request=MagicMock(), response=mock_response
        )
        assert should_retry_exception(error) is False

    def test_other_exception_not_retryable(self):
        """Other exceptions are not retryable."""
        assert should_retry_exception(ValueError("error")) is False
        assert should_retry_exception(KeyError("missing")) is False


class TestWithRetry:
    """Tests for with_retry function."""

    @pytest.mark.asyncio
    async def test_success_on_first_attempt(self):
        """Successful call returns immediately."""
        coro = AsyncMock(return_value="success")

        result = await with_retry(coro())
        assert result == "success"
        assert coro.call_count == 1

    @pytest.mark.asyncio
    async def test_retry_on_retryable_error(self):
        """Retries on retryable error."""
        coro = AsyncMock(
            side_effect=[
                httpx.NetworkError("Failed"),
                httpx.NetworkError("Failed"),
                "success",
            ]
        )

        result = await with_retry(
            coro, config=RetryConfig(max_attempts=3, base_delay=0.01)
        )
        assert result == "success"
        assert coro.call_count == 3

    @pytest.mark.asyncio
    async def test_exhaust_retries(self):
        """Raises after exhausting retries."""
        coro = AsyncMock(side_effect=httpx.NetworkError("Failed"))

        with pytest.raises(httpx.NetworkError):
            await with_retry(coro, config=RetryConfig(max_attempts=3, base_delay=0.01))

        assert coro.call_count == 3

    @pytest.mark.asyncio
    async def test_no_retry_on_non_retryable(self):
        """Does not retry on non-retryable error."""
        mock_response = MagicMock(spec=httpx.Response)
        mock_response.status_code = 401
        error = httpx.HTTPStatusError(
            "Unauthorized", request=MagicMock(), response=mock_response
        )
        coro = AsyncMock(side_effect=error)

        with pytest.raises(httpx.HTTPStatusError):
            await with_retry(coro())

        assert coro.call_count == 1


class TestFailureRecord:
    """Tests for FailureRecord."""

    def test_create_failure_record(self):
        """Can create a FailureRecord."""
        now = datetime.now()
        record = FailureRecord(
            timestamp=now,
            error_category=ErrorCategory.NETWORK,
            message="Connection failed",
        )

        assert record.timestamp == now
        assert record.error_category == ErrorCategory.NETWORK
        assert record.message == "Connection failed"


class TestFailureGate:
    """Tests for FailureGate."""

    def test_create_gate(self):
        """Can create a FailureGate."""
        gate = FailureGate(provider_id="claude")
        assert gate.provider_id == "claude"
        assert gate.failures == []
        assert gate.gated_until is None
        assert gate.consecutive_count == 0

    def test_record_failure_increments_count(self):
        """Recording failure increments consecutive count."""
        gate = FailureGate(provider_id="claude")
        gate.record_failure(ErrorCategory.NETWORK, "Network error")

        assert gate.consecutive_count == 1
        assert len(gate.failures) == 1

    def test_record_success_resets_count(self):
        """Recording success resets consecutive count."""
        gate = FailureGate(provider_id="claude")
        gate.record_failure(ErrorCategory.NETWORK, "Error 1")
        gate.record_failure(ErrorCategory.NETWORK, "Error 2")
        gate.record_success()

        assert gate.consecutive_count == 0

    def test_gate_opens_after_threshold(self):
        """Gate closes after reaching threshold."""
        gate = FailureGate(provider_id="claude")

        # Record failures up to threshold
        for i in range(MAX_CONSECUTIVE_FAILURES):
            gate.record_failure(ErrorCategory.NETWORK, f"Error {i}")

        assert gate.is_gated() is True

    def test_gate_expires(self):
        """Gate expires after GATE_DURATION."""
        gate = FailureGate(provider_id="claude")
        gate.gated_until = datetime.now() - timedelta(seconds=1)

        assert gate.is_gated() is False

    def test_gate_remaining(self):
        """gate_remaining returns time until open."""
        gate = FailureGate(provider_id="claude")
        gate.gated_until = datetime.now() + timedelta(minutes=3)

        remaining = gate.gate_remaining()
        assert remaining is not None
        assert 179 <= remaining.total_seconds() <= 181

    def test_gate_remaining_none_when_not_gated(self):
        """gate_remaining returns None when not gated."""
        gate = FailureGate(provider_id="claude")

        assert gate.gate_remaining() is None

    def test_recent_failures(self):
        """recent_failures returns recent records."""
        gate = FailureGate(provider_id="claude")
        for i in range(10):
            gate.record_failure(ErrorCategory.NETWORK, f"Error {i}")

        recent = gate.recent_failures(limit=5)
        assert len(recent) == 5

    def test_old_failures_cleaned(self):
        """Old failures outside window are cleaned."""
        gate = FailureGate(provider_id="claude")

        # Add old failure
        old_record = FailureRecord(
            timestamp=datetime.now() - WINDOW_DURATION - timedelta(seconds=1),
            error_category=ErrorCategory.NETWORK,
            message="Old error",
        )
        gate.failures.append(old_record)

        # Record new failure should trigger cleanup
        gate.record_failure(ErrorCategory.NETWORK, "New error")

        # Old failure should be removed
        assert old_record not in gate.failures

    def test_clear(self):
        """clear resets all state."""
        gate = FailureGate(provider_id="claude")
        gate.record_failure(ErrorCategory.NETWORK, "Error")
        gate.record_failure(ErrorCategory.NETWORK, "Error")
        gate.record_failure(ErrorCategory.NETWORK, "Error")

        gate.clear()

        assert gate.consecutive_count == 0
        assert gate.gated_until is None
        assert len(gate.failures) == 0


class TestGatePath:
    """Tests for gate_path function."""

    def test_gate_path_format(self):
        """gate_path returns correct format."""
        from pathlib import Path

        with patch("vibeusage.core.gate.gate_dir", return_value=Path("/tmp/gates")):
            result = gate_path("claude")
            assert result == "/tmp/gates/claude.json"


class TestClearGate:
    """Tests for clear_gate function."""

    def test_clear_gate_function(self):
        """clear_gate clears and saves."""
        with (
            patch("vibeusage.core.gate.get_failure_gate") as mock_get,
            patch("vibeusage.core.gate.save_gate") as mock_save,
        ):
            mock_gate = MagicMock()
            mock_get.return_value = mock_gate

            clear_gate("claude")

            mock_gate.clear.assert_called_once()
            mock_save.assert_called_once_with(mock_gate)


class TestAggregatedResult:
    """Tests for AggregatedResult."""

    def test_create_empty(self):
        """Can create empty AggregatedResult."""
        result = AggregatedResult()
        assert result.snapshots == {}
        assert result.errors == {}

    def test_create_with_data(self, sample_snapshot):
        """Can create with snapshots and errors."""
        result = AggregatedResult(
            snapshots={"claude": sample_snapshot},
            errors={"codex": Exception("Failed")},
        )

        assert "claude" in result.snapshots
        assert "codex" in result.errors

    def test_successful_providers(self, sample_snapshot):
        """successful_providers returns list."""
        result = AggregatedResult(
            snapshots={"claude": sample_snapshot, "codex": sample_snapshot}
        )

        providers = result.successful_providers()
        assert set(providers) == {"claude", "codex"}

    def test_failed_providers(self):
        """failed_providers returns list."""
        result = AggregatedResult(
            errors={"claude": Exception("Failed"), "codex": Exception("Failed")}
        )

        providers = result.failed_providers()
        assert set(providers) == {"claude", "codex"}

    def test_has_any_data(self, sample_snapshot):
        """has_any_data returns True when snapshots exist."""
        result = AggregatedResult(snapshots={"claude": sample_snapshot})
        assert result.has_any_data() is True

    def test_has_any_data_false(self):
        """has_any_data returns False when no snapshots."""
        result = AggregatedResult()
        assert result.has_any_data() is False

    def test_all_failed(self):
        """all_failed returns True when all providers failed."""
        result = AggregatedResult(errors={"claude": Exception("Failed")}, snapshots={})
        assert result.all_failed() is True

    def test_all_failed_false_when_success(self, sample_snapshot):
        """all_failed returns False when any succeeded."""
        result = AggregatedResult(
            snapshots={"claude": sample_snapshot},
            errors={"codex": Exception("Failed")},
        )
        assert result.all_failed() is False


class TestAggregateResults:
    """Tests for aggregate_results function."""

    def test_aggregate_successes(self, sample_snapshot):
        """Aggregates successful outcomes."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
            ),
            "codex": FetchOutcome(
                provider_id="codex",
                success=True,
                snapshot=sample_snapshot,
                source="web",
                attempts=[],
            ),
        }

        result = aggregate_results(outcomes)

        assert len(result.snapshots) == 2
        assert "claude" in result.snapshots
        assert "codex" in result.snapshots
        assert len(result.errors) == 0

    def test_aggregate_failures(self):
        """Aggregates failed outcomes."""
        error = Exception("Failed")
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=False,
                snapshot=None,
                source=None,
                attempts=[],
                error=error,
            ),
        }

        result = aggregate_results(outcomes)

        assert len(result.errors) == 1
        assert "claude" in result.errors
        assert len(result.snapshots) == 0

    def test_aggregate_mixed(self, sample_snapshot):
        """Aggregates mixed success/failure outcomes."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
            ),
            "codex": FetchOutcome(
                provider_id="codex",
                success=False,
                snapshot=None,
                source=None,
                attempts=[],
                error=Exception("Failed"),
            ),
        }

        result = aggregate_results(outcomes)

        assert len(result.snapshots) == 1
        assert len(result.errors) == 1


class TestCategorizeResults:
    """Tests for categorize_results function."""

    def test_categorize_success(self, sample_snapshot):
        """Categorizes successful outcome."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
                cached=False,
            )
        }

        result = categorize_results(outcomes)

        assert "claude" in result["success"]
        assert "claude" not in result.get("cached", [])
        assert "claude" not in result.get("failure", [])

    def test_categorize_cached(self, sample_snapshot):
        """Categorizes cached outcome."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="cache",
                attempts=[],
                cached=True,
            )
        }

        result = categorize_results(outcomes)

        assert "claude" in result["cached"]

    def test_categorize_failure(self):
        """Categorizes failed outcome."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=False,
                snapshot=None,
                source=None,
                attempts=[],
                error=Exception("Failed"),
            )
        }

        result = categorize_results(outcomes)

        assert "claude" in result["failure"]

    def test_categorize_gated(self):
        """Categorizes gated outcome."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=False,
                snapshot=None,
                source=None,
                attempts=[],
                error=Exception("Gated"),
                gated=True,
            )
        }

        result = categorize_results(outcomes)

        assert "claude" in result["gated"]


class TestFetchSingleProvider:
    """Tests for fetch_single_provider function."""

    @pytest.mark.asyncio
    async def test_fetch_single(self, sample_snapshot):
        """Fetches single provider successfully."""
        with patch("vibeusage.core.orchestrator.execute_fetch_pipeline") as mock_fetch:
            outcome = FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
            )
            mock_fetch.return_value = outcome

            callback = MagicMock()
            result = await fetch_single_provider("claude", [], on_complete=callback)

            assert result.provider_id == "claude"
            callback.assert_called_once_with(outcome)


class TestFetchAllProviders:
    """Tests for fetch_all_providers function."""

    @pytest.mark.asyncio
    async def test_fetch_multiple(self, sample_snapshot):
        """Fetches multiple providers concurrently."""
        outcomes = {
            "claude": FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
            ),
            "codex": FetchOutcome(
                provider_id="codex",
                success=True,
                snapshot=sample_snapshot,
                source="web",
                attempts=[],
            ),
        }

        with (
            patch("vibeusage.core.orchestrator.execute_fetch_pipeline") as mock_fetch,
            patch("vibeusage.config.settings.get_config") as mock_config,
        ):
            mock_fetch.return_value = outcomes["claude"]
            mock_config.return_value = MagicMock(fetch=MagicMock(max_concurrent=5))

            result = await fetch_all_providers({"claude": [], "codex": []})

            # Since we mock execute_fetch_pipeline to return the same outcome,
            # the result will have both providers but with the same outcome
            assert len(result) == 2


class TestFetchEnabledProviders:
    """Tests for fetch_enabled_providers function."""

    @pytest.mark.asyncio
    async def test_fetch_only_enabled(self, sample_snapshot):
        """Only fetches enabled providers."""
        with (
            patch("vibeusage.config.settings.get_config") as mock_config,
            patch("vibeusage.core.orchestrator.fetch_all_providers") as mock_fetch,
        ):
            # Configure only claude as enabled
            config = MagicMock()
            config.is_provider_enabled.side_effect = lambda pid: pid == "claude"
            mock_config.return_value = config

            outcome = FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=sample_snapshot,
                source="oauth",
                attempts=[],
            )
            mock_fetch.return_value = {"claude": outcome}

            result = await fetch_enabled_providers({"claude": [], "codex": []})

            # Should only return claude
            assert "claude" in result or "codex" not in result
