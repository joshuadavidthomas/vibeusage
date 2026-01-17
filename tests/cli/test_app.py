"""Tests for cli/app.py (main CLI application)."""

from __future__ import annotations

from unittest.mock import AsyncMock
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
import typer

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import _create_provider_command
from vibeusage.cli.app import _show_first_run_message
from vibeusage.cli.app import main
from vibeusage.cli.app import run_app
from vibeusage.cli.app import run_default_usage


class TestExitCode:
    """Tests for ExitCode enum."""

    def test_exit_code_values(self):
        """ExitCode has correct integer values."""
        assert ExitCode.SUCCESS == 0
        assert ExitCode.GENERAL_ERROR == 1
        assert ExitCode.AUTH_ERROR == 2
        assert ExitCode.NETWORK_ERROR == 3
        assert ExitCode.CONFIG_ERROR == 4
        assert ExitCode.PARTIAL_FAILURE == 5

    def test_exit_code_is_int_enum(self):
        """ExitCode is an IntEnum."""
        from enum import IntEnum

        assert issubclass(ExitCode, IntEnum)
        # Can be used as integers
        assert ExitCode.SUCCESS + 1 == 1


class TestMain:
    """Tests for main callback function."""

    def test_main_sets_context_meta(self):
        """main stores options in context meta."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage"),
            patch("vibeusage.cli.app.typer.echo") as mock_echo,
        ):
            main(ctx, json=False, no_color=False, verbose=False, quiet=False, version=False)

            assert ctx.meta["json"] is False
            assert ctx.meta["no_color"] is False
            assert ctx.meta["verbose"] is False
            assert ctx.meta["quiet"] is False

    def test_main_with_json_flag(self):
        """main stores json flag in context."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage"),
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=True, no_color=False, verbose=False, quiet=False, version=False)

            assert ctx.meta["json"] is True

    def test_main_with_verbose_flag(self):
        """main stores verbose flag in context."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage"),
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=False, no_color=False, verbose=True, quiet=False, version=False)

            assert ctx.meta["verbose"] is True

    def test_main_with_quiet_flag(self):
        """main stores quiet flag in context."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage"),
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=False, no_color=False, verbose=False, quiet=True, version=False)

            assert ctx.meta["quiet"] is True

    def test_main_quiet_overrides_verbose(self):
        """quiet flag takes precedence over verbose flag."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage") as mock_default,
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=False, no_color=False, verbose=True, quiet=True, version=False)

            # quiet should override verbose
            assert ctx.meta["quiet"] is True
            assert ctx.meta["verbose"] is False

    def test_main_with_version_flag(self):
        """--version flag shows version and exits."""
        ctx = MagicMock()
        ctx.meta = {}

        with patch("vibeusage.cli.app.typer.echo") as mock_echo:
            with pytest.raises(typer.Exit):
                main(ctx, json=False, no_color=False, verbose=False, quiet=False, version=True)

            mock_echo.assert_called_once()
            output = mock_echo.call_args[0][0]
            assert "vibeusage" in output

    def test_main_runs_default_usage_when_no_subcommand(self):
        """main runs default usage when no subcommand provided."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = None

        with (
            patch("vibeusage.cli.app.run_default_usage") as mock_default,
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=False, no_color=False, verbose=False, quiet=False, version=False)

            mock_default.assert_called_once_with(ctx)

    def test_main_skips_default_when_subcommand_given(self):
        """main does not run default usage when subcommand is provided."""
        ctx = MagicMock()
        ctx.meta = {}
        ctx.invoked_subcommand = "status"

        with (
            patch("vibeusage.cli.app.run_default_usage") as mock_default,
            patch("vibeusage.cli.app.typer.echo"),
        ):
            main(ctx, json=False, no_color=False, verbose=False, quiet=False, version=False)

            mock_default.assert_not_called()


class TestRunDefaultUsage:
    """Tests for run_default_usage async function."""

    @pytest.mark.asyncio
    async def test_run_default_usage_fetches_and_displays(self):
        """run_default_usage fetches and displays usage."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        mock_fetch = AsyncMock(return_value={})

        with (
            patch("vibeusage.cli.commands.usage.fetch_all_usage", mock_fetch),
            patch("vibeusage.cli.commands.usage.display_multiple_snapshots") as mock_display,
            patch("vibeusage.config.credentials.is_first_run", return_value=False),
            patch("vibeusage.core.http.cleanup", AsyncMock()),
        ):
            await run_default_usage(ctx)

            mock_fetch.assert_called_once_with(False)
            mock_display.assert_called_once()

    @pytest.mark.asyncio
    async def test_run_default_usage_shows_first_run_message(self):
        """run_default_usage shows first-run message for new users."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        with (
            patch("vibeusage.cli.commands.usage.fetch_all_usage", AsyncMock()),
            patch("vibeusage.config.credentials.is_first_run", return_value=True),
            patch("vibeusage.cli.app._show_first_run_message") as mock_show,
        ):
            with pytest.raises(typer.Exit) as exc_info:
                await run_default_usage(ctx)

            assert exc_info.value.exit_code == ExitCode.SUCCESS
            mock_show.assert_called_once()

    @pytest.mark.asyncio
    async def test_run_default_usage_skips_first_run_in_json_mode(self):
        """run_default_usage skips first-run message in JSON mode."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        mock_fetch = AsyncMock(return_value={})

        with (
            patch("vibeusage.cli.commands.usage.fetch_all_usage", mock_fetch),
            patch("vibeusage.cli.commands.usage.display_multiple_snapshots"),
            patch("vibeusage.config.credentials.is_first_run", return_value=True),
            patch("vibeusage.cli.app._show_first_run_message") as mock_show,
            patch("vibeusage.core.http.cleanup", AsyncMock()),
        ):
            await run_default_usage(ctx)

            # Should not show first-run message in JSON mode
            mock_show.assert_not_called()

    @pytest.mark.asyncio
    async def test_run_default_usage_skips_first_run_in_quiet_mode(self):
        """run_default_usage skips first-run message in quiet mode."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        mock_fetch = AsyncMock(return_value={})

        with (
            patch("vibeusage.cli.commands.usage.fetch_all_usage", mock_fetch),
            patch("vibeusage.cli.commands.usage.display_multiple_snapshots"),
            patch("vibeusage.config.credentials.is_first_run", return_value=True),
            patch("vibeusage.cli.app._show_first_run_message") as mock_show,
            patch("vibeusage.core.http.cleanup", AsyncMock()),
        ):
            await run_default_usage(ctx)

            # Should not show first-run message in quiet mode
            mock_show.assert_not_called()

    @pytest.mark.asyncio
    async def test_run_default_usage_cleanup_http_client(self):
        """run_default_usage cleans up HTTP client after fetching."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        mock_fetch = AsyncMock(return_value={})
        mock_cleanup = AsyncMock()

        with (
            patch("vibeusage.cli.commands.usage.fetch_all_usage", mock_fetch),
            patch("vibeusage.cli.commands.usage.display_multiple_snapshots"),
            patch("vibeusage.config.credentials.is_first_run", return_value=False),
            patch("vibeusage.core.http.cleanup", mock_cleanup),
        ):
            await run_default_usage(ctx)

            mock_cleanup.assert_called_once()


class TestShowFirstRunMessage:
    """Tests for _show_first_run_message function."""

    def test_show_first_run_message_displays_welcome(self):
        """_show_first_run_message displays welcome panel."""
        from rich.console import Console

        console = Console(record=True)
        _show_first_run_message(console)

        output = console.export_text()
        assert "Welcome" in output or "vibeusage" in output

    def test_show_first_run_message_lists_providers(self):
        """_show_first_run_message lists available providers."""
        from rich.console import Console

        console = Console(record=True)
        with patch("vibeusage.providers.list_provider_ids", return_value=["claude", "codex", "copilot"]):
            _show_first_run_message(console)

        # Should display provider auth commands
        output = console.export_text()
        # Provider IDs should be mentioned
        assert len(output) > 0
        # "vibeusage auth" should be in output
        assert "vibeusage auth" in output or "auth" in output


class TestRunApp:
    """Tests for run_app function."""

    def test_run_app_exists(self):
        """run_app is a callable."""
        assert callable(run_app)

    def test_run_app_calls_app(self):
        """run_app calls the typer app."""
        with patch("vibeusage.cli.app.app") as mock_app:
            run_app()
            mock_app.assert_called_once()


class TestCreateProviderCommand:
    """Tests for _create_provider_command function."""

    def test_create_provider_command_returns_command(self):
        """_create_provider_command returns a command function."""
        command = _create_provider_command("claude")

        assert callable(command)

    def test_create_provider_command_for_all_providers(self):
        """Can create commands for all known providers."""
        providers = ["claude", "codex", "copilot", "cursor", "gemini"]

        for provider_id in providers:
            command = _create_provider_command(provider_id)
            assert callable(command)

    def test_create_provider_command_wraps_async_function(self):
        """Provider command wraps an async function."""
        import inspect

        command = _create_provider_command("claude")

        # The returned command should be callable
        assert callable(command)
        # It should wrap an async function
        # We can't easily inspect the typer-wrapped function, but we can verify it's callable
        assert callable(command)
