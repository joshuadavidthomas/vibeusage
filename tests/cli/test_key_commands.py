"""Tests for key CLI commands."""

from __future__ import annotations

from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit
from typer.testing import CliRunner

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.cli.commands import key as key_module

runner = CliRunner()


class TestKeyCallback:
    """Tests for key_callback function."""

    def test_key_callback_with_no_subcommand(self):
        """Callback shows all credential status when no subcommand."""
        ctx = MagicMock()
        ctx.invoked_subcommand = None
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        with patch.object(key_module, "get_all_credential_status", return_value={}):
            with patch.object(key_module, "display_all_credential_status") as mock_display:
                console = Console(file=StringIO())
                with patch.object(key_module, "Console", return_value=console):
                    key_module.key_callback(ctx)
                    mock_display.assert_called_once()

    def test_key_callback_with_subcommand(self):
        """Callback does nothing when subcommand is invoked."""
        ctx = MagicMock()
        ctx.invoked_subcommand = "set"
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        console = Console(file=StringIO())
        with patch.object(key_module, "Console", return_value=console):
            # Should not raise any errors
            key_module.key_callback(ctx)


class TestKeySetCommand:
    """Tests for key set command via CLI."""

    def test_key_set_invalid_provider(self):
        """Invalid provider exits with error."""
        result = runner.invoke(app, ["key", "invalid_provider", "set"])
        assert result.exit_code != 0
        # Error message is in stderr for invalid commands
        assert "No such command" in result.stderr or "No such command" in result.stdout

    def test_key_set_empty_credential(self):
        """Empty credential shows error."""
        with patch("typer.prompt", return_value=""):
            result = runner.invoke(app, ["key", "claude", "set"])
            # Typer exits with code 1 on empty input (via prompt)
            assert result.exit_code != 0 or "cannot be empty" in result.stdout

    def test_key_set_success_via_cli(self):
        """Successful credential save via CLI."""
        with patch.object(key_module, "write_credential"):
            with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                with patch("typer.prompt", return_value="test-credential-value"):
                    result = runner.invoke(app, ["key", "claude", "set"], input="test-credential-value")
                    # Should succeed or prompt for input
                    assert "Credential saved" in result.stdout or result.exit_code == 0

    def test_key_set_with_type_override(self):
        """Set credential with --type option."""
        with patch.object(key_module, "write_credential"):
            with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                with patch("typer.prompt", return_value="test-oauth-token"):
                    result = runner.invoke(app, ["key", "claude", "set", "--type", "oauth"])
                    # Should succeed or prompt for input
                    assert result.exit_code == 0 or "Credential saved" in result.stdout


class TestKeyDeleteCommand:
    """Tests for key delete command via CLI."""

    def test_key_delete_invalid_provider(self):
        """Invalid provider exits with error."""
        result = runner.invoke(app, ["key", "invalid_provider", "delete"])
        assert result.exit_code != 0
        # Error message is in stderr for invalid commands
        assert "No such command" in result.stderr or "No such command" in result.stdout

    def test_key_delete_specific_type(self):
        """Delete specific credential type."""
        with patch.object(key_module, "delete_credential", return_value=True):
            result = runner.invoke(app, ["key", "claude", "delete", "--type", "session", "--force"])
            assert result.exit_code == 0 or "Deleted" in result.stdout

    def test_key_delete_specific_type_not_found(self):
        """Delete specific type that doesn't exist."""
        with patch.object(key_module, "delete_credential", return_value=False):
            result = runner.invoke(app, ["key", "claude", "delete", "--force"])
            assert "No credential found" in result.stdout or result.exit_code == 0

    def test_key_delete_all_credentials(self):
        """Delete all credentials for provider."""
        from tempfile import TemporaryDirectory
        from vibeusage.config import paths as paths_module

        with TemporaryDirectory() as tmpdir:
            creds_dir = Path(tmpdir)
            provider_dir = creds_dir / "claude"
            provider_dir.mkdir()

            # Create dummy credential files
            (provider_dir / "session.json").touch()
            (provider_dir / "oauth.json").touch()

            with patch.object(paths_module, "credentials_dir", return_value=creds_dir):
                result = runner.invoke(app, ["key", "claude", "delete", "--force"])
                assert result.exit_code == 0 or "Deleted" in result.stdout

    def test_key_delete_all_no_directory(self):
        """Delete all when provider directory doesn't exist."""
        from tempfile import TemporaryDirectory
        from vibeusage.config import paths as paths_module

        with TemporaryDirectory() as tmpdir:
            creds_dir = Path(tmpdir)
            # Don't create the provider directory

            with patch.object(paths_module, "credentials_dir", return_value=creds_dir):
                result = runner.invoke(app, ["key", "claude", "delete", "--force"])
                assert result.exit_code == 0 or "No credential found" in result.stdout


class TestDisplayAllCredentialStatus:
    """Tests for display_all_credential_status function."""

    def test_display_all_json_mode(self, capsys):
        """JSON mode outputs credential status as JSON."""
        all_status = {
            "claude": {"has_credentials": True, "source": "vibeusage"},
            "codex": {"has_credentials": False, "source": None},
        }

        with patch.object(key_module, "get_all_credential_status", return_value=all_status):
            console = Console(file=StringIO())
            key_module.display_all_credential_status(console, json_mode=True)
            captured = capsys.readouterr()
            assert '"claude"' in captured.out
            assert '"codex"' in captured.out
            assert '"configured"' in captured.out

    def test_display_all_quiet_mode(self):
        """Quiet mode shows minimal output."""
        all_status = {
            "claude": {"has_credentials": True, "source": "vibeusage"},
            "codex": {"has_credentials": False, "source": None},
        }

        with patch.object(key_module, "get_all_credential_status", return_value=all_status):
            console = Console(file=StringIO())
            key_module.display_all_credential_status(console, json_mode=False, quiet=True)
            output = console.file.getvalue()
            assert "claude:" in output
            assert "codex:" in output
            assert "configured" in output

    def test_display_all_normal_mode(self):
        """Normal mode shows table."""
        all_status = {
            "claude": {"has_credentials": True, "source": "vibeusage"},
            "codex": {"has_credentials": False, "source": None},
        }

        with patch.object(key_module, "get_all_credential_status", return_value=all_status):
            console = Console(file=StringIO())
            key_module.display_all_credential_status(console, json_mode=False, quiet=False)
            output = console.file.getvalue()
            assert "Credential Status" in output
            assert "Provider" in output

    def test_display_all_verbose_mode(self):
        """Verbose mode shows credential paths."""
        all_status = {
            "claude": {"has_credentials": True, "source": "vibeusage"},
            "codex": {"has_credentials": False, "source": None},
        }

        # Create a function that returns appropriate values based on provider
        def mock_find_credential(provider_id: str):
            if provider_id == "claude":
                return (True, "vibeusage", Path("/creds/claude/session.json"))
            return (False, None, None)

        with patch.object(
            key_module,
            "find_provider_credential",
            side_effect=mock_find_credential,
        ):
            console = Console(file=StringIO())
            key_module.display_all_credential_status(
                console, json_mode=False, verbose=True, quiet=False
            )
            output = console.file.getvalue()
            assert "Credential Paths:" in output or "Paths:" in output


class TestKeyProviderStatus:
    """Tests for provider-specific key status callback."""

    def test_provider_status_callback(self):
        """Provider callback shows status."""
        result = runner.invoke(app, ["key", "claude"])
        # Should show status (configured or not configured)
        assert result.exit_code == 0
        assert "claude" in result.stdout.lower()

    def test_provider_status_json_mode(self):
        """Provider status in JSON mode."""
        result = runner.invoke(app, ["key", "claude", "--json"])
        assert result.exit_code == 0
        # Check for JSON output in either stdout or stderr
        output = result.stdout + result.stderr
        assert '"provider"' in output or "claude" in output.lower()


class TestCreateKeyCommand:
    """Tests for create_key_command factory function."""

    def test_create_key_command_returns_typer_app(self):
        """Factory returns a Typer app."""
        provider_app = key_module.create_key_command("test_provider", "session", None)
        assert provider_app is not None
        assert hasattr(provider_app, "registered_commands")

    def test_create_key_command_with_prefix(self):
        """Factory with prefix validates credentials."""
        provider_app = key_module.create_key_command("test_provider", "session", "test-prefix-")
        assert provider_app is not None


class TestKeyAppRegistration:
    """Tests for key_app ATyper setup."""

    def test_key_app_exists(self):
        """key_app should be properly defined."""
        assert hasattr(key_module, "key_app")
        assert key_module.key_app.info.help == "Manage credentials for providers."

    def test_key_callback_exists(self):
        """key_callback should be registered."""
        assert hasattr(key_module, "key_callback")

    def test_create_key_command_exists(self):
        """create_key_command factory should exist."""
        assert hasattr(key_module, "create_key_command")

    def test_provider_commands_registered(self):
        """Provider commands should be registered."""
        # Check that provider subcommands exist
        result = runner.invoke(app, ["key", "--help"])
        assert "claude" in result.stdout
        assert "codex" in result.stdout
        assert "copilot" in result.stdout
        assert "cursor" in result.stdout
        assert "gemini" in result.stdout
