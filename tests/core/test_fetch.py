"""Tests for core fetch pipeline module."""

from __future__ import annotations

from datetime import timedelta
from unittest.mock import AsyncMock
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest

from vibeusage.core import fetch as fetch_module
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchAttempt
from vibeusage.strategies.base import FetchOutcome
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy


class MockStrategy(FetchStrategy):
    """Mock fetch strategy for testing."""

    def __init__(
        self,
        name: str,
        available: bool = True,
        fetch_result: FetchResult | None = None,
    ):
        self._name = name
        self._available = available
        self._fetch_result = fetch_result

    @property
    def name(self) -> str:
        return self._name

    def is_available(self) -> bool:
        return self._available

    async def fetch(self) -> FetchResult:
        if self._fetch_result:
            return self._fetch_result
        return FetchResult(
            success=False,
            snapshot=None,
            error="Not implemented",
            should_fallback=True,
        )


class TestExecuteFetchPipeline:
    """Tests for execute_fetch_pipeline function."""

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_success(self, utc_now):
        """Successful fetch returns snapshot."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(
            name="test_strategy",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "save_gate"):
                    with patch.object(fetch_module, "save_snapshot"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy], use_cache=False
                        )
                        assert result.success is True
                        assert result.snapshot is not None
                        assert result.provider_id == "claude"
                        assert result.source == "test_strategy"

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_gated_no_cache(self):
        """Gated provider returns error when no cache available."""
        strategy = MockStrategy(name="test_strategy", available=True)

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = True
                mock_gate.gate_remaining.return_value = timedelta(minutes=5)
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "load_cached_snapshot", return_value=None):
                    result = await fetch_module.execute_fetch_pipeline(
                        "claude", [strategy], use_cache=True
                    )
                    assert result.success is False
                    assert result.gated is True
                    assert result.error is not None

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_gated_with_cache(self, utc_now):
        """Gated provider returns cached snapshot when available."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(name="test_strategy", available=True)

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = True
                mock_gate.gate_remaining.return_value = timedelta(minutes=5)
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "load_cached_snapshot", return_value=snapshot):
                    result = await fetch_module.execute_fetch_pipeline(
                        "claude", [strategy], use_cache=True
                    )
                    assert result.success is True
                    assert result.snapshot is not None
                    assert result.cached is True

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_strategy_not_available(self):
        """Skips strategies that are not available."""
        strategy1 = MockStrategy(name="unavailable", available=False)
        snapshot = UsageSnapshot(provider="claude", periods=[], fetched_at=utc_now)
        strategy2 = MockStrategy(
            name="available",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "save_gate"):
                    with patch.object(fetch_module, "save_snapshot"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy1, strategy2], use_cache=False
                        )
                        assert result.success is True
                        assert result.source == "available"

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_fatal_error_stops(self):
        """Fatal error (should_fallback=False) stops trying remaining strategies."""
        strategy = MockStrategy(
            name="fatal",
            available=True,
            fetch_result=FetchResult(
                success=False, snapshot=None, error="Fatal error", should_fallback=False
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                result = await fetch_module.execute_fetch_pipeline(
                    "claude", [strategy], use_cache=False
                )
                assert result.success is False
                assert result.fatal is True

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_timeout(self):
        """Timeout is recorded and continues to next strategy."""
        snapshot = UsageSnapshot(provider="claude", periods=[], fetched_at=utc_now)

        async def timeout_fetch():
            import asyncio

            await asyncio.sleep(5)  # This will timeout

        strategy1 = MockStrategy(name="timeout", available=True)
        strategy1.fetch = timeout_fetch

        strategy2 = MockStrategy(
            name="backup",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=0.1))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "save_gate"):
                    with patch.object(fetch_module, "save_snapshot"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy1, strategy2], use_cache=False
                        )
                        assert result.success is True
                        assert result.source == "backup"

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_all_strategies_fail_with_cache(
        self, utc_now
    ):
        """Returns cached snapshot when all strategies fail."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(
            name="failing",
            available=True,
            fetch_result=FetchResult(
                success=False, snapshot=None, error="Failed", should_fallback=True
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "load_cached_snapshot", return_value=snapshot):
                    with patch.object(fetch_module, "save_gate"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy], use_cache=True
                        )
                        assert result.success is True
                        assert result.cached is True
                        assert result.snapshot is not None

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_all_strategies_fail_no_cache(self):
        """Returns error when all strategies fail and no cache."""
        strategy = MockStrategy(
            name="failing",
            available=True,
            fetch_result=FetchResult(
                success=False, snapshot=None, error="Failed", should_fallback=True
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "load_cached_snapshot", return_value=None):
                    with patch.object(fetch_module, "save_gate"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy], use_cache=False
                        )
                        assert result.success is False
                        assert result.snapshot is None
                        assert result.error is not None

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_records_success_on_gate(self, utc_now):
        """Successful fetch records success on failure gate."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(
            name="test",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "save_gate") as mock_save:
                    with patch.object(fetch_module, "save_snapshot"):
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy], use_cache=False
                        )
                        assert result.success is True
                        mock_gate.record_success.assert_called_once()
                        mock_save.assert_called()

    @pytest.mark.asyncio
    async def test_execute_fetch_pipeline_records_failure_on_gate(self):
        """Failed fetch records failure on failure gate."""
        strategy = MockStrategy(
            name="failing",
            available=True,
            fetch_result=FetchResult(
                success=False, snapshot=None, error="Failed", should_fallback=True
            ),
        )

        with patch.object(fetch_module, "get_config") as mock_config:
            mock_config.return_value = MagicMock(fetch=MagicMock(timeout=30))
            with patch.object(fetch_module, "get_failure_gate") as mock_get_gate:
                mock_gate = MagicMock()
                mock_gate.is_gated.return_value = False
                mock_get_gate.return_value = mock_gate

                with patch.object(fetch_module, "load_cached_snapshot", return_value=None):
                    with patch.object(fetch_module, "save_gate") as mock_save:
                        result = await fetch_module.execute_fetch_pipeline(
                            "claude", [strategy], use_cache=False
                        )
                        assert result.success is False
                        mock_gate.record_failure.assert_called_once()
                        mock_save.assert_called()


class TestFetchWithCacheFallback:
    """Tests for fetch_with_cache_fallback function."""

    @pytest.mark.asyncio
    async def test_fetch_with_cache_fallback_success(self, utc_now):
        """Successful fetch returns snapshot."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(
            name="test",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(
            fetch_module,
            "execute_fetch_pipeline",
            return_value=FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=snapshot,
                source="test",
                attempts=[],
            ),
        ) as mock_pipeline:
            result = await fetch_module.fetch_with_cache_fallback("claude", [strategy])
            assert result.success is True
            mock_pipeline.assert_called_once_with("claude", [strategy], use_cache=True)


class TestFetchWithGate:
    """Tests for fetch_with_gate function."""

    @pytest.mark.asyncio
    async def test_fetch_with_gate_respects_gate(self, utc_now):
        """Fetch with gate respects failure gate state."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        strategy = MockStrategy(
            name="test",
            available=True,
            fetch_result=FetchResult(
                success=True, snapshot=snapshot, error=None, should_fallback=False
            ),
        )

        with patch.object(
            fetch_module,
            "execute_fetch_pipeline",
            return_value=FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=snapshot,
                source="test",
                attempts=[],
            ),
        ) as mock_pipeline:
            result = await fetch_module.fetch_with_gate("claude", [strategy])
            assert result.success is True
            mock_pipeline.assert_called_once_with("claude", [strategy], use_cache=True)


class TestFetchAttempt:
    """Tests for FetchAttempt dataclass."""

    def test_fetch_attempt_creation(self):
        """FetchAttempt can be created with all fields."""
        attempt = FetchAttempt(
            strategy="test_strategy",
            success=True,
            error=None,
            duration_ms=123,
        )
        assert attempt.strategy == "test_strategy"
        assert attempt.success is True
        assert attempt.error is None
        assert attempt.duration_ms == 123


class TestFetchOutcome:
    """Tests for FetchOutcome dataclass."""

    def test_fetch_outcome_success(self, utc_now):
        """FetchOutcome for successful fetch."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=snapshot,
            source="test_strategy",
            attempts=[],
        )
        assert outcome.provider_id == "claude"
        assert outcome.success is True
        assert outcome.snapshot is not None
        assert outcome.source == "test_strategy"
        assert outcome.cached is False
        assert outcome.gated is False
        assert outcome.fatal is False

    def test_fetch_outcome_failure(self):
        """FetchOutcome for failed fetch."""
        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[
                FetchAttempt(
                    strategy="test",
                    success=False,
                    error="Failed",
                    duration_ms=100,
                )
            ],
            error="All strategies failed",
        )
        assert outcome.provider_id == "claude"
        assert outcome.success is False
        assert outcome.snapshot is None
        assert outcome.error is not None

    def test_fetch_outcome_cached(self, utc_now):
        """FetchOutcome for cached result."""
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=snapshot,
            source="cache",
            attempts=[],
            cached=True,
        )
        assert outcome.cached is True
        assert outcome.source == "cache"

    def test_fetch_outcome_gated(self):
        """FetchOutcome for gated provider."""
        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            error="Provider gated for 5:00",
            gated=True,
            gate_remaining=timedelta(minutes=5),
        )
        assert outcome.gated is True
        assert outcome.gate_remaining == timedelta(minutes=5)

    def test_fetch_outcome_fatal(self):
        """FetchOutcome for fatal error."""
        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            error="Fatal configuration error",
            fatal=True,
        )
        assert outcome.fatal is True
