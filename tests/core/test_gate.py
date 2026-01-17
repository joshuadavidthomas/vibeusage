"""Tests for core/gate.py (failure gate to prevent retry flapping)."""

from __future__ import annotations

from datetime import UTC
from datetime import datetime
from datetime import timedelta
from unittest.mock import Mock
from unittest.mock import patch

import pytest

from vibeusage.core.gate import FailureGate
from vibeusage.core.gate import FailureRecord
from vibeusage.core.gate import GATE_DURATION
from vibeusage.core.gate import MAX_CONSECUTIVE_FAILURES
from vibeusage.core.gate import WINDOW_DURATION
from vibeusage.core.gate import clear_gate
from vibeusage.core.gate import gate_path
from vibeusage.core.gate import get_failure_gate
from vibeusage.errors.types import ErrorCategory


class TestFailureRecord:
    """Tests for FailureRecord dataclass."""

    def test_create_failure_record(self):
        """Create a failure record with all fields."""
        now = datetime.now(UTC)
        record = FailureRecord(
            timestamp=now,
            error_category=ErrorCategory.NETWORK,
            message="Connection timeout",
        )

        assert record.timestamp == now
        assert record.error_category == ErrorCategory.NETWORK
        assert record.message == "Connection timeout"


class TestFailureGate:
    """Tests for FailureGate class."""

    def test_create_gate(self):
        """Create a new failure gate."""
        gate = FailureGate(provider_id="claude")

        assert gate.provider_id == "claude"
        assert gate.failures == []
        assert gate.gated_until is None
        assert gate.consecutive_count == 0

    def test_record_failure_increments_count(self):
        """Recording a failure increments consecutive count."""
        gate = FailureGate(provider_id="claude")

        gate.record_failure(ErrorCategory.NETWORK, "Timeout")

        assert gate.consecutive_count == 1
        assert len(gate.failures) == 1
        assert gate.failures[0].message == "Timeout"

    def test_record_multiple_failures(self):
        """Recording multiple failures tracks all of them."""
        gate = FailureGate(provider_id="codex")

        gate.record_failure(ErrorCategory.NETWORK, "Error 1")
        gate.record_failure(ErrorCategory.AUTHENTICATION, "Error 2")
        gate.record_failure(ErrorCategory.PROVIDER, "Error 3")

        assert gate.consecutive_count == 3
        assert len(gate.failures) == 3

    def test_gate_activates_after_max_failures(self):
        """Gate activates after MAX_CONSECUTIVE_FAILURES."""
        gate = FailureGate(provider_id="copilot")

        for i in range(MAX_CONSECUTIVE_FAILURES):
            gate.record_failure(ErrorCategory.NETWORK, f"Error {i}")

        assert gate.is_gated() is True
        assert gate.gated_until is not None

    def test_gate_expires_after_duration(self):
        """Gate expires after GATE_DURATION."""
        gate = FailureGate(provider_id="cursor")

        # Trigger gating
        for _ in range(MAX_CONSECUTIVE_FAILURES):
            gate.record_failure(ErrorCategory.NETWORK, "Error")

        assert gate.is_gated() is True

        # Mock datetime.now() to return time after gate duration
        future_time = datetime.now() + GATE_DURATION + timedelta(seconds=1)
        with patch("vibeusage.core.gate.datetime") as mock_datetime:
            mock_datetime.now.return_value = future_time
            assert gate.is_gated() is False
            assert gate.gated_until is None  # Should be cleared

    def test_is_gated_returns_false_when_not_gated(self):
        """is_gated returns False when gate is not active."""
        gate = FailureGate(provider_id="gemini")

        assert gate.is_gated() is False

        # One failure doesn't trigger gate
        gate.record_failure(ErrorCategory.NETWORK, "Error")
        assert gate.is_gated() is False

    def test_gate_remaining_when_gated(self):
        """gate_remaining returns timedelta when gated."""
        gate = FailureGate(provider_id="claude")

        # Trigger gating
        for _ in range(MAX_CONSECUTIVE_FAILURES):
            gate.record_failure(ErrorCategory.NETWORK, "Error")

        remaining = gate.gate_remaining()
        assert remaining is not None
        assert remaining.total_seconds() > 0
        assert remaining <= GATE_DURATION

    def test_gate_remaining_when_not_gated(self):
        """gate_remaining returns None when not gated."""
        gate = FailureGate(provider_id="claude")

        assert gate.gate_remaining() is None

    def test_gate_remaining_after_expiry(self):
        """gate_remaining returns None after gate expires."""
        gate = FailureGate(provider_id="claude")
        gate.gated_until = datetime.now() - timedelta(seconds=1)

        assert gate.gate_remaining() is None

    def test_record_success_resets_consecutive_count(self):
        """Recording success resets consecutive failure count."""
        gate = FailureGate(provider_id="claude")

        gate.record_failure(ErrorCategory.NETWORK, "Error 1")
        gate.record_failure(ErrorCategory.NETWORK, "Error 2")
        assert gate.consecutive_count == 2

        gate.record_success()
        assert gate.consecutive_count == 0

    def test_old_failures_cleaned_up(self):
        """Old failures outside WINDOW_DURATION are cleaned up."""
        gate = FailureGate(provider_id="claude")

        # Add an old failure
        old_time = datetime.now() - WINDOW_DURATION - timedelta(seconds=1)
        old_record = FailureRecord(
            timestamp=old_time,
            error_category=ErrorCategory.NETWORK,
            message="Old error",
        )
        gate.failures.append(old_record)

        # Add a new failure
        gate.record_failure(ErrorCategory.NETWORK, "New error")

        # Old failure should be removed
        assert len(gate.failures) == 1
        assert gate.failures[0].message == "New error"

    def test_recent_failures_limit(self):
        """recent_failures respects the limit parameter."""
        gate = FailureGate(provider_id="claude")

        for i in range(10):
            gate.record_failure(ErrorCategory.NETWORK, f"Error {i}")

        recent = gate.recent_failures(limit=5)
        assert len(recent) == 5

    def test_recent_failures_default_limit(self):
        """recent_failures uses default limit of 5."""
        gate = FailureGate(provider_id="claude")

        for i in range(10):
            gate.record_failure(ErrorCategory.NETWORK, f"Error {i}")

        recent = gate.recent_failures()
        assert len(recent) == 5

    def test_recent_failures_when_empty(self):
        """recent_failures returns empty list when no failures."""
        gate = FailureGate(provider_id="claude")

        recent = gate.recent_failures()
        assert recent == []

    def test_clear_resets_all_state(self):
        """Clear resets all gate state."""
        gate = FailureGate(provider_id="claude")

        gate.record_failure(ErrorCategory.NETWORK, "Error")
        gate.record_failure(ErrorCategory.NETWORK, "Error")
        gate.record_failure(ErrorCategory.NETWORK, "Error")

        assert gate.consecutive_count == 3
        assert len(gate.failures) > 0
        assert gate.is_gated() is True

        gate.clear()

        assert gate.consecutive_count == 0
        assert len(gate.failures) == 0
        assert gate.is_gated() is False
        assert gate.gated_until is None


class TestGateConstants:
    """Tests for gate constants."""

    def test_max_consecutive_failures_is_three(self):
        """MAX_CONSECUTIVE_FAILURES should be 3."""
        assert MAX_CONSECUTIVE_FAILURES == 3

    def test_gate_duration_is_five_minutes(self):
        """GATE_DURATION should be 5 minutes."""
        assert GATE_DURATION == timedelta(minutes=5)

    def test_window_duration_is_ten_minutes(self):
        """WINDOW_DURATION should be 10 minutes."""
        assert WINDOW_DURATION == timedelta(minutes=10)


class TestGetFailureGate:
    """Tests for get_failure_gate function."""

    def test_get_gate_creates_new_gate(self):
        """Get gate creates a new gate for unknown provider."""
        # Reset global gates
        import vibeusage.core.gate
        vibeusage.core.gate._gates = {}

        gate = get_failure_gate("claude")

        assert gate.provider_id == "claude"
        assert gate.consecutive_count == 0

    def test_get_gate_reuses_existing_gate(self):
        """Get gate reuses existing gate for known provider."""
        import vibeusage.core.gate
        vibeusage.core.gate._gates = {}

        gate1 = get_failure_gate("claude")
        gate1.record_failure(ErrorCategory.NETWORK, "Test")

        gate2 = get_failure_gate("claude")

        assert gate1 is gate2
        assert gate2.consecutive_count == 1


class TestGatePath:
    """Tests for gate_path function."""

    def test_gate_path_returns_string(self):
        """gate_path returns a string path."""
        path = gate_path("claude")

        assert isinstance(path, str)
        assert "claude.json" in path

    def test_gate_path_includes_provider_id(self):
        """gate_path includes provider ID in filename."""
        path = gate_path("codex")

        assert "codex.json" in path


class TestClearGate:
    """Tests for clear_gate function."""

    def test_clear_gate_function(self):
        """clear_gate function clears and saves gate."""
        import vibeusage.core.gate
        vibeusage.core.gate._gates = {}

        # Create and gate a provider
        gate = get_failure_gate("claude")
        for _ in range(MAX_CONSECUTIVE_FAILURES):
            gate.record_failure(ErrorCategory.NETWORK, "Error")

        assert gate.is_gated() is True

        # Clear should reset the gate
        with patch("vibeusage.core.gate.save_gate") as mock_save:
            clear_gate("claude")
            mock_save.assert_called_once()

        assert gate.is_gated() is False
        assert gate.consecutive_count == 0


class TestLoadGate:
    """Tests for load_gate function."""

    def test_load_gate_with_no_state(self):
        """load_gate returns None when no state exists."""
        import vibeusage.core.gate
        vibeusage.core.gate._gates = {}

        with patch("vibeusage.config.cache.load_cached_gate_state") as mock_load:
            mock_load.return_value = None
            gate = vibeusage.core.gate.load_gate("claude")

            assert gate is None

    def test_load_gate_with_state(self):
        """load_gate reconstructs gate from saved state."""
        import vibeusage.core.gate
        vibeusage.core.gate._gates = {}

        state = {
            "gated_until": "2025-01-15T12:00:00",
            "failures": [
                {
                    "timestamp": "2025-01-15T11:00:00",
                    "error_category": "NETWORK",
                    "message": "Timeout",
                }
            ],
        }

        with patch("vibeusage.config.cache.load_cached_gate_state") as mock_load:
            mock_load.return_value = state
            gate = vibeusage.core.gate.load_gate("claude")

            assert gate.provider_id == "claude"
            assert gate.gated_until is not None
            assert len(gate.failures) == 1
            assert gate.failures[0].message == "Timeout"


class TestSaveGate:
    """Tests for save_gate function."""

    def test_save_gate_serializes_state(self):
        """save_gate serializes gate state to disk format."""
        import vibeusage.core.gate

        now = datetime.now(UTC)
        gate = FailureGate(provider_id="claude")
        gate.failures.append(
            FailureRecord(
                timestamp=now,
                error_category=ErrorCategory.NETWORK,
                message="Test error",
            )
        )

        with patch("vibeusage.config.cache.cache_gate_state") as mock_cache:
            vibeusage.core.gate.save_gate(gate)

            mock_cache.assert_called_once()
            call_args = mock_cache.call_args
            assert call_args[0][0] == "claude"
            # failures_data and gated_until should be passed
            assert len(call_args[0][1]) == 1  # failures_data
