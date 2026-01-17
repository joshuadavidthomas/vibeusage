"""Tests for cache CLI commands."""

from __future__ import annotations

from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands import cache as cache_module
from vibeusage.models import UsageSnapshot


class TestCacheShowCommand:
    """Tests for cache_show_command function."""

    def test_cache_show_json_mode(self, utc_now, capsys):
        """JSON mode outputs cache data as JSON."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        with patch.object(cache_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(cache_module, "snapshot_path") as mock_snap_path:
                with patch.object(cache_module, "org_id_path") as mock_org_path:
                    # Setup mock paths
                    snap_path = MagicMock(spec=Path)
                    snap_path.exists.return_value = True
                    org_path = MagicMock(spec=Path)
                    org_path.exists.return_value = False

                    mock_snap_path.return_value = snap_path
                    mock_org_path.return_value = org_path

                    with patch.object(cache_module, "load_cached_snapshot", return_value=snapshot):
                        with patch.object(cache_module, "get_config") as mock_config:
                            mock_config.return_value = MagicMock(
                                fetch=MagicMock(stale_threshold_minutes=60)
                            )
                            cache_module.cache_show_command(ctx)
                            captured = capsys.readouterr()
                            assert '"claude"' in captured.out or '"snapshot"' in captured.out

    def test_cache_show_quiet_mode(self, utc_now):
        """Quiet mode shows minimal output."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        with patch.object(cache_module, "list_provider_ids", return_value=["claude"]):
            with patch.object(cache_module, "snapshot_path") as mock_snap_path:
                with patch.object(cache_module, "org_id_path") as mock_org_path:
                    snap_path = MagicMock(spec=Path)
                    snap_path.exists.return_value = True
                    org_path = MagicMock(spec=Path)
                    org_path.exists.return_value = False

                    mock_snap_path.return_value = snap_path
                    mock_org_path.return_value = org_path

                    with patch.object(cache_module, "load_cached_snapshot", return_value=snapshot):
                        with patch.object(cache_module, "get_config") as mock_config:
                            mock_config.return_value = MagicMock(
                                fetch=MagicMock(stale_threshold_minutes=60)
                            )
                            console = Console(file=StringIO())
                            with patch.object(cache_module, "Console", return_value=console):
                                cache_module.cache_show_command(ctx)
                                output = console.file.getvalue()
                                assert "claude:" in output
                                # Should not have table formatting in quiet mode
                                assert "Cache Status" not in output

    def test_cache_show_normal_mode_table(self, utc_now):
        """Normal mode shows table with cache status."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        with patch.object(cache_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(cache_module, "snapshot_path") as mock_snap_path:
                with patch.object(cache_module, "org_id_path") as mock_org_path:
                    snap_path = MagicMock(spec=Path)
                    snap_path.exists.return_value = True
                    org_path = MagicMock(spec=Path)
                    org_path.exists.return_value = True

                    mock_snap_path.return_value = snap_path
                    mock_org_path.return_value = org_path

                    with patch.object(cache_module, "load_cached_snapshot", return_value=snapshot):
                        with patch.object(cache_module, "get_config") as mock_config:
                            mock_config.return_value = MagicMock(
                                fetch=MagicMock(stale_threshold_minutes=60)
                            )
                            console = Console(file=StringIO())
                            with patch.object(cache_module, "Console", return_value=console):
                                with patch.object(cache_module, "cache_dir", return_value=Path("/cache")):
                                    cache_module.cache_show_command(ctx)
                                    output = console.file.getvalue()
                                    assert "Cache Status" in output
                                    assert "Provider" in output
                                    assert "Snapshot" in output
                                    assert "Cache directory:" in output

    def test_cache_show_verbose_mode(self, utc_now):
        """Verbose mode shows additional info including stale threshold."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": True, "quiet": False}

        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=utc_now,
        )

        with patch.object(cache_module, "list_provider_ids", return_value=["claude"]):
            with patch.object(cache_module, "snapshot_path") as mock_snap_path:
                with patch.object(cache_module, "org_id_path") as mock_org_path:
                    snap_path = MagicMock(spec=Path)
                    snap_path.exists.return_value = True
                    org_path = MagicMock(spec=Path)
                    org_path.exists.return_value = False

                    mock_snap_path.return_value = snap_path
                    mock_org_path.return_value = org_path

                    with patch.object(cache_module, "load_cached_snapshot", return_value=snapshot):
                        with patch.object(cache_module, "get_config") as mock_config:
                            mock_config.return_value = MagicMock(
                                fetch=MagicMock(stale_threshold_minutes=60)
                            )
                            console = Console(file=StringIO())
                            with patch.object(cache_module, "Console", return_value=console):
                                with patch.object(cache_module, "cache_dir", return_value=Path("/cache")):
                                    cache_module.cache_show_command(ctx)
                                    output = console.file.getvalue()
                                    assert "Stale threshold:" in output
                                    assert "60 minutes" in output

    def test_cache_show_no_snapshot(self):
        """Provider with no snapshot shows 'none' status."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        with patch.object(cache_module, "list_provider_ids", return_value=["claude"]):
            with patch.object(cache_module, "snapshot_path") as mock_snap_path:
                with patch.object(cache_module, "org_id_path") as mock_org_path:
                    snap_path = MagicMock(spec=Path)
                    snap_path.exists.return_value = False
                    org_path = MagicMock(spec=Path)
                    org_path.exists.return_value = False

                    mock_snap_path.return_value = snap_path
                    mock_org_path.return_value = org_path

                    with patch.object(cache_module, "get_config") as mock_config:
                        mock_config.return_value = MagicMock(
                            fetch=MagicMock(stale_threshold_minutes=60)
                        )
                        console = Console(file=StringIO())
                        with patch.object(cache_module, "Console", return_value=console):
                            with patch.object(cache_module, "cache_dir", return_value=Path("/cache")):
                                cache_module.cache_show_command(ctx)
                                output = console.file.getvalue()
                                # Should show table even with no snapshots
                                assert "Cache Status" in output


class TestCacheClearCommand:
    """Tests for cache_clear_command function."""

    def test_cache_clear_all(self):
        """Clear all cache without provider argument."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        with patch.object(cache_module, "clear_all_cache") as mock_clear:
            console = Console(file=StringIO())
            with patch.object(cache_module, "Console", return_value=console):
                cache_module.cache_clear_command(ctx, None, False)
                mock_clear.assert_called_once_with()
                output = console.file.getvalue()
                assert "Cleared all cache" in output

    def test_cache_clear_all_org_only(self):
        """Clear all org ID cache only."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        with patch.object(cache_module, "clear_all_cache") as mock_clear:
            console = Console(file=StringIO())
            with patch.object(cache_module, "Console", return_value=console):
                cache_module.cache_clear_command(ctx, None, True)
                mock_clear.assert_called_once_with(org_ids_only=True)
                output = console.file.getvalue()
                assert "org ID cache" in output

    def test_cache_clear_specific_provider(self):
        """Clear cache for specific provider."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        with patch.object(cache_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(cache_module, "clear_provider_cache") as mock_clear:
                console = Console(file=StringIO())
                with patch.object(cache_module, "Console", return_value=console):
                    cache_module.cache_clear_command(ctx, "claude", False)
                    mock_clear.assert_called_once_with("claude")

    def test_cache_clear_specific_provider_org_only(self):
        """Clear org ID cache for specific provider."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        with patch.object(cache_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(cache_module, "clear_org_id_cache") as mock_clear:
                console = Console(file=StringIO())
                with patch.object(cache_module, "Console", return_value=console):
                    cache_module.cache_clear_command(ctx, "claude", True)
                    mock_clear.assert_called_once_with("claude")

    def test_cache_clear_invalid_provider(self):
        """Invalid provider exits with CONFIG_ERROR."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        with patch.object(cache_module, "list_provider_ids", return_value=["claude", "codex"]):
            console = Console(file=StringIO())
            with patch.object(cache_module, "Console", return_value=console):
                with pytest.raises(TyperExit) as exc_info:
                    cache_module.cache_clear_command(ctx, "invalid", False)
                assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR
                output = console.file.getvalue()
                assert "Unknown provider:" in output

    def test_cache_clear_json_mode(self, capsys):
        """JSON mode outputs result as JSON."""
        ctx = MagicMock()
        ctx.meta = {"json": True}

        with patch.object(cache_module, "clear_all_cache"):
            cache_module.cache_clear_command(ctx, None, False)
            captured = capsys.readouterr()
            assert '"success"' in captured.out
            assert '"message"' in captured.out


class TestCacheCallback:
    """Tests for cache_callback function."""

    def test_cache_callback_stores_context(self):
        """Callback stores context for subcommands."""
        ctx = MagicMock()
        ctx.invoked_subcommand = "show"

        # Should not raise any errors
        cache_module.cache_callback(ctx)


class TestCacheAppRegistration:
    """Tests for cache_app ATyper setup."""

    def test_cache_app_exists(self):
        """cache_app should be properly defined."""
        assert hasattr(cache_module, "cache_app")
        assert cache_module.cache_app.info.help == "Manage cached usage data."

    def test_cache_show_command_exists(self):
        """cache_show_command should be registered."""
        assert hasattr(cache_module, "cache_show_command")

    def test_cache_clear_command_exists(self):
        """cache_clear_command should be registered."""
        assert hasattr(cache_module, "cache_clear_command")
