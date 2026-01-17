"""Tests for key CLI commands."""

from __future__ import annotations

from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands import key as key_module


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
    """Tests for key_set_command function."""

    def test_key_set_invalid_provider(self):
        """Invalid provider exits with CONFIG_ERROR."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            console = Console(file=StringIO())
            with patch.object(key_module, "Console", return_value=console):
                with pytest.raises(TyperExit) as exc_info:
                    key_module.key_set_command("invalid", "session")
                assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_key_set_empty_credential(self):
        """Empty credential exits with CONFIG_ERROR."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch("typer.prompt", return_value=""):
                console = Console(file=StringIO())
                with patch.object(key_module, "Console", return_value=console):
                    with pytest.raises(TyperExit) as exc_info:
                        key_module.key_set_command("claude", "session")
                    assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_key_set_success(self):
        """Successful credential save."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch("typer.prompt", return_value="test-credential-value"):
                with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                    with patch.object(key_module, "write_credential"):
                        console = Console(file=StringIO())
                        with patch.object(key_module, "Console", return_value=console):
                            key_module.key_set_command("claude", "session")

    def test_key_set_write_error(self):
        """Write error exits with GENERAL_ERROR."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch("typer.prompt", return_value="test-credential"):
                with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                    with patch.object(key_module, "write_credential", side_effect=Exception("Write error")):
                        console = Console(file=StringIO())
                        with patch.object(key_module, "Console", return_value=console):
                            with pytest.raises(TyperExit) as exc_info:
                                key_module.key_set_command("claude", "session")
                            assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR


class TestKeyDeleteCommand:
    """Tests for key_delete_command function."""

    def test_key_delete_invalid_provider(self):
        """Invalid provider exits with CONFIG_ERROR."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            console = Console(file=StringIO())
            with patch.object(key_module, "Console", return_value=console):
                with pytest.raises(TyperExit) as exc_info:
                    key_module.key_delete_command("invalid", None)
                assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_key_delete_specific_type(self):
        """Delete specific credential type."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                with patch.object(key_module, "delete_credential", return_value=True):
                    console = Console(file=StringIO())
                    with patch.object(key_module, "Console", return_value=console):
                        key_module.key_delete_command("claude", "session")

    def test_key_delete_specific_type_not_found(self):
        """Delete specific type that doesn't exist."""
        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with patch.object(key_module, "credential_path", return_value=Path("/cred/claude.json")):
                with patch.object(key_module, "delete_credential", return_value=False):
                    console = Console(file=StringIO())
                    with patch.object(key_module, "Console", return_value=console):
                        key_module.key_delete_command("claude", "session")

    def test_key_delete_all_credentials(self):
        """Delete all credentials for provider."""
        from tempfile import TemporaryDirectory
        from vibeusage.config import paths as paths_module

        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with TemporaryDirectory() as tmpdir:
                creds_dir = Path(tmpdir)
                provider_dir = creds_dir / "claude"
                provider_dir.mkdir()

                # Create dummy credential files
                (provider_dir / "session.json").touch()
                (provider_dir / "oauth.json").touch()

                with patch.object(paths_module, "credentials_dir", return_value=creds_dir):
                    with patch.object(key_module, "delete_credential", wraps=key_module.delete_credential) as mock_del:
                        console = Console(file=StringIO())
                        with patch.object(key_module, "Console", return_value=console):
                            key_module.key_delete_command("claude", None)
                            # delete_credential is called for each file
                            assert mock_del.call_count >= 2

    def test_key_delete_all_no_directory(self):
        """Delete all when provider directory doesn't exist."""
        from tempfile import TemporaryDirectory
        from vibeusage.config import paths as paths_module

        with patch.object(key_module, "list_provider_ids", return_value=["claude", "codex"]):
            with TemporaryDirectory() as tmpdir:
                creds_dir = Path(tmpdir)
                # Don't create the provider directory

                with patch.object(paths_module, "credentials_dir", return_value=creds_dir):
                    console = Console(file=StringIO())
                    with patch.object(key_module, "Console", return_value=console):
                        key_module.key_delete_command("claude", None)


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

        with patch.object(key_module, "get_all_credential_status", return_value=all_status):
            with patch.object(
                key_module,
                "find_provider_credential",
                side_effect=[
                    (True, "vibeusage", Path("/creds/claude/session.json")),
                    (False, None, None),
                ],
            ):
                console = Console(file=StringIO())
                key_module.display_all_credential_status(
                    console, json_mode=False, verbose=True, quiet=False
                )
                output = console.file.getvalue()
                assert "Credential Paths:" in output


class TestDisplayProviderCredentialStatus:
    """Tests for display_provider_credential_status function."""

    def test_display_provider_configured(self):
        """Display configured provider."""
        with patch.object(
            key_module,
            "find_provider_credential",
            return_value=(True, "vibeusage", Path("/creds/claude/session.json")),
        ):
            console = Console(file=StringIO())
            key_module.display_provider_credential_status(
                console, "claude", has_creds=True, source="vibeusage"
            )
            output = console.file.getvalue()
            assert "configured" in output

    def test_display_provider_not_configured(self):
        """Display unconfigured provider."""
        with patch.object(
            key_module, "find_provider_credential", return_value=(False, None, None)
        ):
            console = Console(file=StringIO())
            key_module.display_provider_credential_status(
                console, "claude", has_creds=False, source=None
            )
            output = console.file.getvalue()
            assert "not configured" in output

    def test_display_provider_source_labels(self):
        """Test various source label mappings."""
        console = Console(file=StringIO())

        with patch.object(key_module, "find_provider_credential", return_value=(True, None, None)):
            key_module.display_provider_credential_status(
                console, "claude", has_creds=True, source="vibeusage"
            )
            output = console.file.getvalue()
            assert "vibeusage storage" in output

        console.file.truncate(0)
        console.file.seek(0)

        with patch.object(key_module, "find_provider_credential", return_value=(True, None, None)):
            key_module.display_provider_credential_status(
                console, "claude", has_creds=True, source="provider_cli"
            )
            output = console.file.getvalue()
            assert "provider CLI" in output


class TestKeyAppRegistration:
    """Tests for key_app ATyper setup."""

    def test_key_app_exists(self):
        """key_app should be properly defined."""
        assert hasattr(key_module, "key_app")
        assert key_module.key_app.info.help == "Manage credentials for providers."

    def test_key_set_command_exists(self):
        """key_set_command should be registered."""
        assert hasattr(key_module, "key_set_command")

    def test_key_delete_command_exists(self):
        """key_delete_command should be registered."""
        assert hasattr(key_module, "key_delete_command")
