"""Tests for init command (first-run setup wizard)."""

from __future__ import annotations

import json
import os
from pathlib import Path
from unittest.mock import AsyncMock
from unittest.mock import MagicMock
from unittest.mock import patch

import typer

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands.init import init_command
from vibeusage.config.settings import Config
from vibeusage.config.settings import CredentialsConfig


class TestInitCommand:
    """Tests for init command."""

    def test_init_command_exists(self):
        """init_command is a callable."""
        assert callable(init_command)

    def test_init_json_mode_first_run(self, tmp_path):
        """JSON mode shows first_run=True when no credentials."""
        config = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            patch.dict(os.environ, {}, clear=True),
        ):
            mock_get_config.return_value = config

            ctx = MagicMock()
            ctx.meta = {"json": True, "verbose": False, "quiet": False}

            # Capture JSON output
            import sys
            from io import StringIO

            old_stdout = sys.stdout
            sys.stdout = StringIO()

            try:
                init_command(ctx, quick=False, skip=False, json_output=True)
            except typer.Exit:
                pass
            finally:
                output = sys.stdout.getvalue()
                sys.stdout = old_stdout

            # Verify JSON output
            data = json.loads(output)
            assert "first_run" in data
            assert data["first_run"] is True
            assert "configured_providers" in data
            assert data["configured_providers"] == 0
            assert "available_providers" in data

    def test_init_json_mode_configured(self, tmp_path):
        """JSON mode shows first_run=False when credentials exist."""
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
        ):
            mock_get_config.return_value = Config()

            # Create a credential file
            cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
            cred_path.parent.mkdir(parents=True, exist_ok=True)
            cred_path.write_bytes(b'{"token": "test"}')

            ctx = MagicMock()
            ctx.meta = {"json": True, "verbose": False, "quiet": False}

            # Capture JSON output
            import sys
            from io import StringIO

            old_stdout = sys.stdout
            sys.stdout = StringIO()

            try:
                init_command(ctx, quick=False, skip=False, json_output=True)
            except typer.Exit:
                pass
            finally:
                output = sys.stdout.getvalue()
                sys.stdout = old_stdout

            # Verify JSON output
            data = json.loads(output)
            assert "first_run" in data
            assert data["first_run"] is False
            assert data["configured_providers"] >= 1

    def test_init_skip_flag_first_run(self, tmp_path):
        """--skip flag shows first run message."""
        config = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            patch.dict(os.environ, {}, clear=True),
        ):
            mock_get_config.return_value = config

            ctx = MagicMock()
            ctx.meta = {"json": False, "verbose": False, "quiet": False}

            with pytest.raises(typer.Exit) as exc_info:
                init_command(ctx, quick=False, skip=True, json_output=False)
            assert exc_info.value.exit_code == ExitCode.SUCCESS

    def test_init_skip_flag_configured(self, tmp_path):
        """--skip flag shows configured message."""
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
        ):
            mock_get_config.return_value = Config()

            # Create a credential file
            cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
            cred_path.parent.mkdir(parents=True, exist_ok=True)
            cred_path.write_bytes(b'{"token": "test"}')

            ctx = MagicMock()
            ctx.meta = {"json": False, "verbose": False, "quiet": False}

            with pytest.raises(typer.Exit) as exc_info:
                init_command(ctx, quick=False, skip=True, json_output=False)
            assert exc_info.value.exit_code == ExitCode.SUCCESS

    def test_init_quick_flag(self, tmp_path):
        """--quick flag runs quick setup."""
        config = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            patch.dict(os.environ, {}, clear=True),
        ):
            mock_get_config.return_value = config

            ctx = MagicMock()
            ctx.meta = {"json": False, "verbose": False, "quiet": False}

            with pytest.raises(typer.Exit) as exc_info:
                init_command(ctx, quick=True, skip=False, json_output=False)
            assert exc_info.value.exit_code == ExitCode.SUCCESS

    def test_init_quick_flag_quiet_mode(self, tmp_path):
        """--quick flag with --quiet shows minimal output."""
        config = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            patch.dict(os.environ, {}, clear=True),
        ):
            mock_get_config.return_value = config

            ctx = MagicMock()
            ctx.meta = {"json": False, "verbose": False, "quiet": True}

            with pytest.raises(typer.Exit) as exc_info:
                init_command(ctx, quick=True, skip=False, json_output=False)
            assert exc_info.value.exit_code == ExitCode.SUCCESS


class TestShowWelcome:
    """Tests for _show_welcome function."""

    def test_show_welcome_message(self):
        """_show_welcome displays welcome panel."""
        from rich.console import Console

        from vibeusage.cli.commands.init import _show_welcome

        console = Console()
        # Should not raise any errors
        _show_welcome(console)


class TestShowAlreadyConfigured:
    """Tests for _show_already_configured function."""

    def test_shows_configuration_table(self):
        """_show_already_configured displays provider status table."""
        from rich.console import Console

        from vibeusage.cli.commands.init import _show_already_configured

        console = Console()
        # Should not raise any errors
        _show_already_configured(console, verbose=False, quiet=False)

    def test_quiet_mode_minimal_output(self):
        """_show_already_configured in quiet mode shows minimal output."""
        from rich.console import Console

        from vibeusage.cli.commands.init import _show_already_configured

        console = Console()
        # Should not raise any errors
        _show_already_configured(console, verbose=False, quiet=True)


class TestQuickSetup:
    """Tests for _quick_setup function."""

    def test_quick_setup_shows_instructions(self, tmp_path):
        """_quick_setup shows Claude setup instructions."""
        config = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            patch.dict(os.environ, {}, clear=True),
        ):
            mock_get_config.return_value = config

            from rich.console import Console

            from vibeusage.cli.commands.init import _quick_setup

            console = Console()

            # This should not raise when quiet=False and Claude not configured
            # It just prints instructions
            try:
                _quick_setup(console, verbose=False, quiet=False)
            except typer.Exit:
                # Exit is OK too
                pass

    def test_quick_setup_already_configured(self, tmp_path):
        """_quick_setup shows message when Claude already configured."""
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
        ):
            mock_get_config.return_value = Config()

            # Create Claude credential
            cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
            cred_path.parent.mkdir(parents=True, exist_ok=True)
            cred_path.write_bytes(b'{"token": "test"}')

            from rich.console import Console

            from vibeusage.cli.commands.init import _quick_setup

            console = Console()

            with pytest.raises(typer.Exit) as exc_info:
                _quick_setup(console, verbose=False, quiet=False)
            assert exc_info.value.exit_code == ExitCode.SUCCESS


class TestRunAuthProvider:
    """Tests for _run_auth_for_provider function."""

    def test_run_auth_claude(self, tmp_path):
        """_run_auth_for_provider handles Claude provider."""
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            # Patch the auth command at its source module
            patch("vibeusage.cli.commands.auth.auth_claude_command") as mock_auth,
        ):
            mock_get_config.return_value = Config()

            from vibeusage.cli.commands.init import _run_auth_for_provider

            # The function should call the auth command
            _run_auth_for_provider("claude", verbose=False, quiet=True)
            # The auth command should have been called
            mock_auth.assert_called_once()

    def test_run_auth_generic_provider(self, tmp_path):
        """_run_auth_for_provider handles generic providers."""
        with (
            patch(
                "vibeusage.config.credentials.credentials_dir",
                return_value=tmp_path / "credentials",
            ),
            patch("vibeusage.config.settings.get_config") as mock_get_config,
            # Patch the auth command at its source module
            patch("vibeusage.cli.commands.auth.auth_generic_command") as mock_auth,
        ):
            mock_get_config.return_value = Config()

            from vibeusage.cli.commands.init import _run_auth_for_provider

            # The function should call the auth command
            _run_auth_for_provider("codex", verbose=False, quiet=True)
            # The auth command should have been called
            mock_auth.assert_called_once()


class TestProviderDescriptions:
    """Tests for provider description constants."""

    def test_provider_descriptions_exist(self):
        """Provider descriptions dictionary has expected keys."""
        from vibeusage.cli.commands.init import _PROVIDER_DESCRIPTIONS

        expected_providers = ["claude", "codex", "copilot", "cursor", "gemini"]
        for provider in expected_providers:
            assert provider in _PROVIDER_DESCRIPTIONS
            assert isinstance(_PROVIDER_DESCRIPTIONS[provider], str)
            assert len(_PROVIDER_DESCRIPTIONS[provider]) > 0

    def test_setup_commands_exist(self):
        """Setup commands dictionary has expected keys."""
        from vibeusage.cli.commands.init import _QUICK_SETUP_COMMANDS

        expected_providers = ["claude", "codex", "copilot", "cursor", "gemini"]
        for provider in expected_providers:
            assert provider in _QUICK_SETUP_COMMANDS
            assert "vibeusage auth" in _QUICK_SETUP_COMMANDS[provider]


# Import pytest after we use it in type hints
import pytest
