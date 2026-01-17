"""Tests for cache module functions."""

from __future__ import annotations

from collections.abc import Generator
from datetime import UTC
from datetime import datetime
from datetime import timedelta
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import msgspec
import pytest

from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot

# Import cache functions after paths for proper patching
from vibeusage.config import cache as cache_module


@pytest.fixture
def temp_cache_dirs(tmp_path: Path) -> Generator[dict[str, Path]]:
    """Temporary cache directories for testing.

    This fixture patches all cache-related directory functions to use
    a temporary directory, ensuring tests don't affect actual cache.
    """
    cache = tmp_path / "cache"
    state = tmp_path / "state"
    cache.mkdir()
    state.mkdir()

    snapshots = cache / "snapshots"
    org_ids = cache / "org-ids"
    gates = state / "gates"

    with (
        patch("vibeusage.config.cache.snapshots_dir", return_value=snapshots),
        patch("vibeusage.config.cache.org_ids_dir", return_value=org_ids),
        patch("vibeusage.config.cache.gate_dir", return_value=gates),
    ):
        yield {"snapshots": snapshots, "org_ids": org_ids, "gates": gates, "cache": cache, "state": state}


class TestSnapshotPath:
    """Tests for snapshot_path function."""

    def test_snapshot_path_returns_correct_path(self, temp_cache_dirs):
        """snapshot_path returns correct Path object."""
        result = cache_module.snapshot_path("test_provider")
        expected = temp_cache_dirs["snapshots"] / "test_provider.msgpack"
        assert result == expected

    def test_snapshot_path_different_providers(self, temp_cache_dirs):
        """snapshot_path returns unique paths for different providers."""
        path1 = cache_module.snapshot_path("claude")
        path2 = cache_module.snapshot_path("codex")
        assert path1 != path2
        assert "claude" in str(path1)
        assert "codex" in str(path2)


class TestCacheSnapshot:
    """Tests for cache_snapshot function."""

    def test_cache_snapshot_creates_directory(self, temp_cache_dirs):
        """cache_snapshot creates parent directory if it doesn't exist."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        snapshots = temp_cache_dirs["snapshots"]
        assert snapshots.exists()
        assert (snapshots / "test_provider.msgpack").exists()

    def test_cache_snapshot_writes_correct_data(self, temp_cache_dirs):
        """cache_snapshot writes correct msgpack data."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC),
            periods=(
                UsagePeriod(
                    name="Session",
                    utilization=65,
                    period_type=PeriodType.SESSION,
                    resets_at=datetime(2025, 1, 15, 15, 0, 0, tzinfo=UTC),
                ),
            ),
        )
        cache_module.cache_snapshot(snapshot)
        loaded = cache_module.load_cached_snapshot("test_provider")
        assert loaded is not None
        assert loaded.provider == "test_provider"
        assert len(loaded.periods) == 1
        assert loaded.periods[0].utilization == 65

    def test_cache_snapshot_overwrites_existing(self, temp_cache_dirs):
        """cache_snapshot overwrites existing cache file."""
        snapshot1 = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC),
            periods=(
                UsagePeriod(
                    name="Session",
                    utilization=50,
                    period_type=PeriodType.SESSION,
                    resets_at=datetime(2025, 1, 15, 15, 0, 0, tzinfo=UTC),
                ),
            ),
        )
        cache_module.cache_snapshot(snapshot1)

        snapshot2 = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime(2025, 1, 15, 13, 0, 0, tzinfo=UTC),
            periods=(
                UsagePeriod(
                    name="Session",
                    utilization=75,
                    period_type=PeriodType.SESSION,
                    resets_at=datetime(2025, 1, 15, 16, 0, 0, tzinfo=UTC),
                ),
            ),
        )
        cache_module.cache_snapshot(snapshot2)

        loaded = cache_module.load_cached_snapshot("test_provider")
        assert loaded.periods[0].utilization == 75


class TestLoadCachedSnapshot:
    """Tests for load_cached_snapshot function."""

    def test_load_cached_snapshot_file_not_exists(self, temp_cache_dirs):
        """load_cached_snapshot returns None when file doesn't exist."""
        result = cache_module.load_cached_snapshot("nonexistent")
        assert result is None

    def test_load_cached_snapshot_decode_error(self, temp_cache_dirs):
        """load_cached_snapshot returns None on msgspec.DecodeError."""
        # Create a corrupt msgpack file
        snapshots = temp_cache_dirs["snapshots"]
        snapshots.mkdir(parents=True, exist_ok=True)
        cache_file = snapshots / "test_provider.msgpack"
        cache_file.write_bytes(b"corrupt data that is not valid msgpack")
        result = cache_module.load_cached_snapshot("test_provider")
        assert result is None

    def test_load_cached_snapshot_os_error(self, temp_cache_dirs):
        """load_cached_snapshot returns None on OSError."""
        # Create a file and make it unreadable
        snapshots = temp_cache_dirs["snapshots"]
        snapshots.mkdir(parents=True, exist_ok=True)
        cache_file = snapshots / "test_provider.msgpack"
        cache_file.write_bytes(b"{}")

        with patch.object(Path, "read_bytes", side_effect=OSError("Permission denied")):
            result = cache_module.load_cached_snapshot("test_provider")
            assert result is None

    def test_load_cached_snapshot_valid_data(self, temp_cache_dirs):
        """load_cached_snapshot returns valid UsageSnapshot."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC),
            periods=(
                UsagePeriod(
                    name="Session",
                    utilization=65,
                    period_type=PeriodType.SESSION,
                    resets_at=datetime(2025, 1, 15, 15, 0, 0, tzinfo=UTC),
                ),
            ),
            overage=OverageUsage(
                used=10,
                limit=100,
                currency="USD",
                is_enabled=True,
            ),
        )
        cache_module.cache_snapshot(snapshot)
        loaded = cache_module.load_cached_snapshot("test_provider")
        assert loaded is not None
        assert loaded.provider == "test_provider"
        assert loaded.overage is not None
        assert loaded.overage.used == 10


class TestIsSnapshotFresh:
    """Tests for is_snapshot_fresh function."""

    def test_is_snapshot_fresh_returns_true_when_fresh(self, temp_cache_dirs):
        """is_snapshot_fresh returns True for recent snapshot."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC) - timedelta(minutes=30),  # 30 min ago
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        result = cache_module.is_snapshot_fresh("test_provider", stale_threshold_minutes=60)
        assert result is True

    def test_is_snapshot_fresh_returns_false_when_stale(self, temp_cache_dirs):
        """is_snapshot_fresh returns False for old snapshot."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC) - timedelta(minutes=90),  # 90 min ago
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        result = cache_module.is_snapshot_fresh("test_provider", stale_threshold_minutes=60)
        assert result is False

    def test_is_snapshot_fresh_returns_false_when_none(self, temp_cache_dirs):
        """is_snapshot_fresh returns False when snapshot doesn't exist."""
        result = cache_module.is_snapshot_fresh("nonexistent", stale_threshold_minutes=60)
        assert result is False

    def test_is_snapshot_fresh_custom_threshold(self, temp_cache_dirs):
        """is_snapshot_fresh respects custom stale threshold."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC) - timedelta(minutes=45),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        # With 30 min threshold, should be stale
        assert cache_module.is_snapshot_fresh("test_provider", stale_threshold_minutes=30) is False
        # With 60 min threshold, should be fresh
        assert cache_module.is_snapshot_fresh("test_provider", stale_threshold_minutes=60) is True


class TestGetSnapshotAgeMinutes:
    """Tests for get_snapshot_age_minutes function."""

    def test_get_snapshot_age_minutes_returns_correct_value(self, temp_cache_dirs):
        """get_snapshot_age_minutes returns correct age in minutes."""
        fetched_at = datetime.now(UTC) - timedelta(minutes=45, seconds=30)
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=fetched_at,
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        age = cache_module.get_snapshot_age_minutes("test_provider")
        assert age == 45  # Should round down

    def test_get_snapshot_age_minutes_returns_none_when_none(self, temp_cache_dirs):
        """get_snapshot_age_minutes returns None when snapshot doesn't exist."""
        age = cache_module.get_snapshot_age_minutes("nonexistent")
        assert age is None

    def test_get_snapshot_age_minutes_zero_for_fresh(self, temp_cache_dirs):
        """get_snapshot_age_minutes returns 0 for very recent snapshot."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC) - timedelta(seconds=30),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        age = cache_module.get_snapshot_age_minutes("test_provider")
        assert age == 0


class TestOrgIdPath:
    """Tests for org_id_path function."""

    def test_org_id_path_returns_correct_path(self, temp_cache_dirs):
        """org_id_path returns correct Path object."""
        result = cache_module.org_id_path("test_provider")
        expected = temp_cache_dirs["org_ids"] / "test_provider.txt"
        assert result == expected


class TestCacheOrgId:
    """Tests for cache_org_id function."""

    def test_cache_org_id_creates_directory(self, temp_cache_dirs):
        """cache_org_id creates parent directory if it doesn't exist."""
        cache_module.cache_org_id("test_provider", "org_12345")
        org_ids = temp_cache_dirs["org_ids"]
        assert org_ids.exists()
        assert (org_ids / "test_provider.txt").exists()

    def test_cache_org_id_writes_correct_org_id(self, temp_cache_dirs):
        """cache_org_id writes correct org ID to file."""
        cache_module.cache_org_id("test_provider", "org_12345")
        loaded = cache_module.load_cached_org_id("test_provider")
        assert loaded == "org_12345"

    def test_cache_org_id_trims_whitespace(self, temp_cache_dirs):
        """cache_org_id strips whitespace when reading."""
        cache_module.cache_org_id("test_provider", "  org_12345  ")
        loaded = cache_module.load_cached_org_id("test_provider")
        assert loaded == "org_12345"


class TestLoadCachedOrgId:
    """Tests for load_cached_org_id function."""

    def test_load_cached_org_id_file_not_exists(self, temp_cache_dirs):
        """load_cached_org_id returns None when file doesn't exist."""
        result = cache_module.load_cached_org_id("nonexistent")
        assert result is None

    def test_load_cached_org_id_os_error(self, temp_cache_dirs):
        """load_cached_org_id returns None on OSError."""
        # Create a file
        org_ids = temp_cache_dirs["org_ids"]
        org_ids.mkdir(parents=True, exist_ok=True)
        cache_file = org_ids / "test_provider.txt"
        cache_file.write_text("org_123")

        with patch.object(Path, "read_text", side_effect=OSError("Permission denied")):
            result = cache_module.load_cached_org_id("test_provider")
            assert result is None

    def test_load_cached_org_id_valid_data(self, temp_cache_dirs):
        """load_cached_org_id returns correct org ID."""
        cache_module.cache_org_id("test_provider", "org_abc123")
        result = cache_module.load_cached_org_id("test_provider")
        assert result == "org_abc123"


class TestClearOrgIdCache:
    """Tests for clear_org_id_cache function."""

    def test_clear_org_id_cache_specific_provider(self, temp_cache_dirs):
        """clear_org_id_cache deletes specific provider's cache file."""
        cache_module.cache_org_id("provider1", "org_1")
        cache_module.cache_org_id("provider2", "org_2")
        org_ids = temp_cache_dirs["org_ids"]
        assert (org_ids / "provider1.txt").exists()
        assert (org_ids / "provider2.txt").exists()

        cache_module.clear_org_id_cache("provider1")
        assert not (org_ids / "provider1.txt").exists()
        assert (org_ids / "provider2.txt").exists()

    def test_clear_org_id_cache_all_providers(self, temp_cache_dirs):
        """clear_org_id_cache deletes all org ID files when provider is None."""
        cache_module.cache_org_id("provider1", "org_1")
        cache_module.cache_org_id("provider2", "org_2")
        org_ids = temp_cache_dirs["org_ids"]
        assert (org_ids / "provider1.txt").exists()
        assert (org_ids / "provider2.txt").exists()

        cache_module.clear_org_id_cache(None)
        assert not (org_ids / "provider1.txt").exists()
        assert not (org_ids / "provider2.txt").exists()

    def test_clear_org_id_cache_nonexistent_file(self, temp_cache_dirs):
        """clear_org_id_cache doesn't raise error for nonexistent file."""
        # Should not raise any errors
        cache_module.clear_org_id_cache("nonexistent")


class TestClearProviderCache:
    """Tests for clear_provider_cache function."""

    def test_clear_provider_cache_deletes_both_files(self, temp_cache_dirs):
        """clear_provider_cache deletes both snapshot and org ID files."""
        # Create both files
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        cache_module.cache_org_id("test_provider", "org_123")

        snap_path = temp_cache_dirs["snapshots"] / "test_provider.msgpack"
        org_path = temp_cache_dirs["org_ids"] / "test_provider.txt"
        assert snap_path.exists()
        assert org_path.exists()

        cache_module.clear_provider_cache("test_provider")
        assert not snap_path.exists()
        assert not org_path.exists()

    def test_clear_provider_cache_missing_ok(self, temp_cache_dirs):
        """clear_provider_cache doesn't raise when files don't exist."""
        # Should not raise any errors
        cache_module.clear_provider_cache("nonexistent")


class TestClearSnapshotCache:
    """Tests for clear_snapshot_cache function."""

    def test_clear_snapshot_cache_specific_provider(self, temp_cache_dirs):
        """clear_snapshot_cache deletes specific provider's snapshot."""
        snapshot1 = UsageSnapshot(
            provider="provider1",
            fetched_at=datetime.now(UTC),
            periods=(),
        )
        snapshot2 = UsageSnapshot(
            provider="provider2",
            fetched_at=datetime.now(UTC),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot1)
        cache_module.cache_snapshot(snapshot2)

        snapshots = temp_cache_dirs["snapshots"]
        assert (snapshots / "provider1.msgpack").exists()
        assert (snapshots / "provider2.msgpack").exists()

        cache_module.clear_snapshot_cache("provider1")
        assert not (snapshots / "provider1.msgpack").exists()
        assert (snapshots / "provider2.msgpack").exists()

    def test_clear_snapshot_cache_all_providers(self, temp_cache_dirs):
        """clear_snapshot_cache deletes all snapshot files when provider is None."""
        for i in range(3):
            snapshot = UsageSnapshot(
                provider=f"provider{i}",
                fetched_at=datetime.now(UTC),
                periods=(),
            )
            cache_module.cache_snapshot(snapshot)

        cache_module.clear_snapshot_cache(None)
        snapshots = temp_cache_dirs["snapshots"]
        assert not list(snapshots.glob("*.msgpack"))


class TestClearAllCache:
    """Tests for clear_all_cache function."""

    def test_clear_all_cache_specific_provider(self, temp_cache_dirs):
        """clear_all_cache deletes both snapshot and org ID for specific provider."""
        snapshot = UsageSnapshot(
            provider="test_provider",
            fetched_at=datetime.now(UTC),
            periods=(),
        )
        cache_module.cache_snapshot(snapshot)
        cache_module.cache_org_id("test_provider", "org_123")

        snap_path = temp_cache_dirs["snapshots"] / "test_provider.msgpack"
        org_path = temp_cache_dirs["org_ids"] / "test_provider.txt"
        assert snap_path.exists()
        assert org_path.exists()

        cache_module.clear_all_cache("test_provider")
        assert not snap_path.exists()
        assert not org_path.exists()

    def test_clear_all_cache_all_providers(self, temp_cache_dirs):
        """clear_all_cache deletes all files in both cache directories."""
        # Create multiple files in both directories
        for i in range(3):
            snapshot = UsageSnapshot(
                provider=f"provider{i}",
                fetched_at=datetime.now(UTC),
                periods=(),
            )
            cache_module.cache_snapshot(snapshot)
            cache_module.cache_org_id(f"provider{i}", f"org_{i}")

        cache_module.clear_all_cache(None)
        snapshots = temp_cache_dirs["snapshots"]
        org_ids = temp_cache_dirs["org_ids"]
        assert not list(snapshots.glob("*.msgpack"))
        assert not list(org_ids.glob("*.txt"))


class TestGatePath:
    """Tests for gate_path function."""

    def test_gate_path_returns_correct_path(self, temp_cache_dirs):
        """gate_path returns correct Path object."""
        result = cache_module.gate_path("test_provider")
        expected = temp_cache_dirs["gates"] / "test_provider.msgpack"
        assert result == expected


class TestCacheGateState:
    """Tests for cache_gate_state function."""

    def test_cache_gate_state_creates_directory(self, temp_cache_dirs):
        """cache_gate_state creates parent directory if it doesn't exist."""
        failures = [{"error": "test"}]
        cache_module.cache_gate_state("test_provider", failures, None)
        gates = temp_cache_dirs["gates"]
        assert gates.exists()
        assert (gates / "test_provider.msgpack").exists()

    def test_cache_gate_state_with_gated_until(self, temp_cache_dirs):
        """cache_gate_state saves state with gated_until datetime."""
        failures = [
            {"error": "Connection timeout", "category": "network"},
        ]
        gated_until = datetime(2025, 1, 15, 14, 0, 0, tzinfo=UTC)
        cache_module.cache_gate_state("test_provider", failures, gated_until)

        loaded = cache_module.load_cached_gate_state("test_provider")
        assert loaded is not None
        assert loaded["failures"] == failures
        assert loaded["gated_until"] == gated_until

    def test_cache_gate_state_without_gated_until(self, temp_cache_dirs):
        """cache_gate_state saves state with None gated_until."""
        failures = [{"error": "test"}]
        cache_module.cache_gate_state("test_provider", failures, None)

        loaded = cache_module.load_cached_gate_state("test_provider")
        assert loaded is not None
        assert loaded["failures"] == failures
        assert loaded["gated_until"] is None


class TestLoadCachedGateState:
    """Tests for load_cached_gate_state function."""

    def test_load_cached_gate_state_file_not_exists(self, temp_cache_dirs):
        """load_cached_gate_state returns None when file doesn't exist."""
        result = cache_module.load_cached_gate_state("nonexistent")
        assert result is None

    def test_load_cached_gate_state_decode_error(self, temp_cache_dirs):
        """load_cached_gate_state returns None on msgspec.DecodeError."""
        # Create a corrupt file
        gates = temp_cache_dirs["gates"]
        gates.mkdir(parents=True, exist_ok=True)
        cache_file = gates / "test_provider.msgpack"
        cache_file.write_bytes(b"corrupt data")
        result = cache_module.load_cached_gate_state("test_provider")
        assert result is None

    def test_load_cached_gate_state_os_error(self, temp_cache_dirs):
        """load_cached_gate_state returns None on OSError."""
        gates = temp_cache_dirs["gates"]
        gates.mkdir(parents=True, exist_ok=True)
        cache_file = gates / "test_provider.msgpack"
        cache_file.write_bytes(b"{}")

        with patch.object(Path, "read_bytes", side_effect=OSError("Permission denied")):
            result = cache_module.load_cached_gate_state("test_provider")
            assert result is None

    def test_load_cached_gate_state_value_error(self, temp_cache_dirs):
        """load_cached_gate_state returns None on ValueError (invalid datetime)."""
        # Create file with invalid datetime format
        gates = temp_cache_dirs["gates"]
        gates.mkdir(parents=True, exist_ok=True)
        cache_file = gates / "test_provider.msgpack"
        state = {
            "failures": [{"error": "test"}],
            "gated_until": "invalid-datetime-format",
        }
        cache_file.write_bytes(msgspec.json.encode(state))
        result = cache_module.load_cached_gate_state("test_provider")
        assert result is None

    def test_load_cached_gate_state_parses_datetime_correctly(self, temp_cache_dirs):
        """load_cached_gate_state parses ISO format datetime correctly."""
        failures = [{"error": "test"}]
        gated_until = datetime(2025, 1, 15, 14, 30, 0, tzinfo=UTC)
        cache_module.cache_gate_state("test_provider", failures, gated_until)

        loaded = cache_module.load_cached_gate_state("test_provider")
        assert loaded is not None
        assert isinstance(loaded["gated_until"], datetime)
        assert loaded["gated_until"] == gated_until

    def test_load_cached_gate_state_valid_state(self, temp_cache_dirs):
        """load_cached_gate_state returns valid state dict."""
        failures = [
            {"error": "Connection timeout", "category": "network"},
            {"error": "Rate limit", "category": "rate_limit"},
        ]
        gated_until = datetime(2025, 1, 15, 16, 0, 0, tzinfo=UTC)
        cache_module.cache_gate_state("test_provider", failures, gated_until)

        loaded = cache_module.load_cached_gate_state("test_provider")
        assert loaded is not None
        assert len(loaded["failures"]) == 2
        assert loaded["failures"][0]["error"] == "Connection timeout"
        assert loaded["gated_until"] == gated_until
