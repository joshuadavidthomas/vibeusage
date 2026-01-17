"""Tests for status CLI command."""

from __future__ import annotations

from io import StringIO
from unittest.mock import AsyncMock
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands import status as status_module
from vibeusage.models import StatusLevel


class TestStatusCommand:
    """Tests for status_command function."""

    @pytest.mark.asyncio
    async def test_status_command_json_mode(self, utc_now, capsys):
        """JSON mode outputs status as JSON."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, True)
                    captured = capsys.readouterr()
                    assert '"level"' in captured.out or '"operational"' in captured.out

    @pytest.mark.asyncio
    async def test_status_command_context_json_mode(self, utc_now, capsys):
        """Uses json from context meta."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, False)
                    captured = capsys.readouterr()
                    assert '"level"' in captured.out or '"operational"' in captured.out

    @pytest.mark.asyncio
    async def test_status_command_quiet_mode(self, utc_now):
        """Quiet mode shows minimal output."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, False)
                    output = console.file.getvalue()
                    assert "claude:" in output

    @pytest.mark.asyncio
    async def test_status_command_normal_mode(self, utc_now):
        """Normal mode shows status table."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, False)
                    output = console.file.getvalue()
                    assert "Provider Status" in output
                    assert "Provider" in output

    @pytest.mark.asyncio
    async def test_status_command_verbose_mode(self, utc_now):
        """Verbose mode shows timing info."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": True, "quiet": False}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, False)
                    output = console.file.getvalue()
                    assert "Fetched in" in output
                    assert "ms" in output

    @pytest.mark.asyncio
    async def test_status_command_keyboard_interrupt(self):
        """Keyboard interrupt exits with GENERAL_ERROR."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        async def mock_fetch():
            raise KeyboardInterrupt()

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    with pytest.raises(TyperExit) as exc_info:
                        await status_module.status_command(ctx, False)
                    assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR

    @pytest.mark.asyncio
    async def test_status_command_exception(self):
        """Generic exception exits with GENERAL_ERROR."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        async def mock_fetch():
            raise Exception("Test error")

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup"):
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    with pytest.raises(TyperExit) as exc_info:
                        await status_module.status_command(ctx, False)
                    assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR

    @pytest.mark.asyncio
    async def test_status_command_cleanup_always_runs(self, utc_now):
        """Cleanup runs even on exception."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        async def mock_fetch():
            return {"claude": mock_status}

        with patch.object(status_module, "fetch_all_statuses", side_effect=mock_fetch):
            with patch.object(status_module, "cleanup") as mock_cleanup:
                console = Console(file=StringIO())
                with patch.object(status_module, "Console", return_value=console):
                    await status_module.status_command(ctx, False)
                    mock_cleanup.assert_called_once()


class TestFetchAllStatuses:
    """Tests for fetch_all_statuses function."""

    @pytest.mark.asyncio
    async def test_fetch_all_statuses_success(self, utc_now):
        """Successfully fetch status from all providers."""
        mock_provider = MagicMock()
        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )
        mock_provider.fetch_status = AsyncMock(return_value=mock_status)

        with patch.object(
            status_module, "get_all_providers", return_value={"claude": lambda: mock_provider}
        ):
            result = await status_module.fetch_all_statuses()
            assert "claude" in result
            assert result["claude"].level == StatusLevel.OPERATIONAL

    @pytest.mark.asyncio
    async def test_fetch_all_statuses_provider_error(self):
        """Provider error returns UNKNOWN status."""
        mock_provider = MagicMock()
        mock_provider.fetch_status = AsyncMock(side_effect=Exception("Provider error"))

        with patch.object(
            status_module, "get_all_providers", return_value={"claude": lambda: mock_provider}
        ):
            result = await status_module.fetch_all_statuses()
            assert "claude" in result
            assert result["claude"].level == StatusLevel.UNKNOWN


class TestDisplayStatusTable:
    """Tests for display_status_table function."""

    def test_display_status_table_normal(self, utc_now):
        """Normal mode shows table with all columns."""
        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        statuses = {"claude": mock_status}

        console = Console(file=StringIO())
        status_module.display_status_table(console, statuses, verbose=False, quiet=False)
        output = console.file.getvalue()
        assert "Provider Status" in output
        assert "Provider" in output

    def test_display_status_table_quiet(self, utc_now):
        """Quiet mode shows minimal output."""
        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        statuses = {"claude": mock_status}

        console = Console(file=StringIO())
        status_module.display_status_table(console, statuses, verbose=False, quiet=True)
        output = console.file.getvalue()
        assert "claude:" in output

    def test_display_status_table_verbose(self, utc_now):
        """Verbose mode includes timing info."""
        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        statuses = {"claude": mock_status}

        console = Console(file=StringIO())
        status_module.display_status_table(
            console, statuses, verbose=True, quiet=False, duration_ms=123.45
        )
        output = console.file.getvalue()
        assert "Fetched in" in output
        assert "123ms" in output


class TestOutputJsonStatus:
    """Tests for output_json_status function."""

    def test_output_json_status(self, utc_now, capsys):
        """Output statuses as JSON."""
        mock_status = MagicMock(
            level=StatusLevel.OPERATIONAL,
            description="All systems operational",
            updated_at=utc_now,
        )

        statuses = {"claude": mock_status}

        console = Console(file=StringIO())
        status_module.output_json_status(statuses)
        captured = capsys.readouterr()
        assert '"level"' in captured.out


class TestStatusSymbol:
    """Tests for status_symbol function."""

    def test_status_symbol_operational(self):
        """Returns correct symbol for OPERATIONAL."""
        assert status_module.status_symbol(StatusLevel.OPERATIONAL) == "●"

    def test_status_symbol_degraded(self):
        """Returns correct symbol for DEGRADED."""
        assert status_module.status_symbol(StatusLevel.DEGRADED) == "◐"

    def test_status_symbol_partial_outage(self):
        """Returns correct symbol for PARTIAL_OUTAGE."""
        assert status_module.status_symbol(StatusLevel.PARTIAL_OUTAGE) == "◑"

    def test_status_symbol_major_outage(self):
        """Returns correct symbol for MAJOR_OUTAGE."""
        assert status_module.status_symbol(StatusLevel.MAJOR_OUTAGE) == "○"

    def test_status_symbol_unknown(self):
        """Returns correct symbol for UNKNOWN."""
        assert status_module.status_symbol(StatusLevel.UNKNOWN) == "?"


class TestStatusColor:
    """Tests for status_color function."""

    def test_status_color_operational(self):
        """Returns correct color for OPERATIONAL."""
        assert status_module.status_color(StatusLevel.OPERATIONAL) == "green"

    def test_status_color_degraded(self):
        """Returns correct color for DEGRADED."""
        assert status_module.status_color(StatusLevel.DEGRADED) == "yellow"

    def test_status_color_partial_outage(self):
        """Returns correct color for PARTIAL_OUTAGE."""
        assert status_module.status_color(StatusLevel.PARTIAL_OUTAGE) == "orange"

    def test_status_color_major_outage(self):
        """Returns correct color for MAJOR_OUTAGE."""
        assert status_module.status_color(StatusLevel.MAJOR_OUTAGE) == "red"

    def test_status_color_unknown(self):
        """Returns correct color for UNKNOWN."""
        assert status_module.status_color(StatusLevel.UNKNOWN) == "dim"


class TestFormatStatusUpdated:
    """Tests for format_status_updated function."""

    def test_format_status_updated_none(self):
        """Returns 'unknown' for None."""
        assert status_module.format_status_updated(None) == "unknown"

    def test_format_status_updated_days_ago(self):
        """Returns days ago for old timestamps."""
        from datetime import timedelta
        from datetime import UTC
        from datetime import datetime

        now = datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC)
        past_time = now - timedelta(days=5)

        with patch("vibeusage.cli.commands.status.datetime", wraps=datetime) as mock_dt:
            original_now = datetime.now
            mock_dt.now = lambda tz=None: now if tz == UTC else original_now(tz)
            assert status_module.format_status_updated(past_time) == "5d ago"

    def test_format_status_updated_hours_ago(self):
        """Returns hours ago for recent timestamps."""
        from datetime import timedelta
        from datetime import UTC
        from datetime import datetime

        now = datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC)
        past_time = now - timedelta(hours=3)

        with patch("vibeusage.cli.commands.status.datetime", wraps=datetime) as mock_dt:
            original_now = datetime.now
            mock_dt.now = lambda tz=None: now if tz == UTC else original_now(tz)
            assert status_module.format_status_updated(past_time) == "3h ago"

    def test_format_status_updated_minutes_ago(self):
        """Returns minutes ago for very recent timestamps."""
        from datetime import timedelta
        from datetime import UTC
        from datetime import datetime

        now = datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC)
        past_time = now - timedelta(minutes=15)

        with patch("vibeusage.cli.commands.status.datetime", wraps=datetime) as mock_dt:
            original_now = datetime.now
            mock_dt.now = lambda tz=None: now if tz == UTC else original_now(tz)
            assert status_module.format_status_updated(past_time) == "15m ago"

    def test_format_status_updated_just_now(self):
        """Returns 'just now' for very recent timestamps."""
        from datetime import UTC
        from datetime import datetime

        now = datetime(2025, 1, 15, 12, 0, 0, tzinfo=UTC)

        with patch("vibeusage.cli.commands.status.datetime", wraps=datetime) as mock_dt:
            original_now = datetime.now
            mock_dt.now = lambda tz=None: now if tz == UTC else original_now(tz)
            assert status_module.format_status_updated(now) == "just now"
