"""Tests for config CLI commands."""

from __future__ import annotations

from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands import config as config_module
from vibeusage.config.settings import Config
from vibeusage.config.settings import DisplayConfig
from vibeusage.config.settings import FetchConfig
from vibeusage.config.settings import CredentialsConfig


class TestConfigShowCommand:
    """Tests for config_show_command function."""

    def test_config_show_json_mode(self, capsys):
        """JSON mode outputs config as JSON."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        config = Config(
            fetch=FetchConfig(
                timeout=30,
                stale_threshold_minutes=60,
                max_concurrent=5,
            ),
            enabled_providers=None,
            display=DisplayConfig(
                show_remaining=True,
                pace_colors=True,
                reset_format="countdown",
            ),
            credentials=CredentialsConfig(
                use_keyring=False,
                reuse_provider_credentials=True,
            ),
        )

        with patch.object(config_module, "get_config", return_value=config):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                config_module.config_show_command(ctx)
                captured = capsys.readouterr()
                assert '"timeout"' in captured.out
                assert '"stale_threshold_minutes"' in captured.out
                assert '"max_concurrent"' in captured.out

    def test_config_show_quiet_mode(self):
        """Quiet mode shows only config file path."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        with patch.object(config_module, "get_config", return_value=Config()):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                console = Console(file=StringIO())
                with patch.object(config_module, "Console", return_value=console):
                    config_module.config_show_command(ctx)
                output = console.file.getvalue()
                assert "/config/config.toml" in output

    def test_config_show_normal_mode(self):
        """Normal mode shows TOML formatted config in panel."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        with patch.object(config_module, "get_config", return_value=Config()):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                console = Console(file=StringIO())
                with patch.object(config_module, "Console", return_value=console):
                    config_module.config_show_command(ctx)
                output = console.file.getvalue()
                assert "Config:" in output

    def test_config_show_verbose_mode(self):
        """Verbose mode shows additional config file info."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": True, "quiet": False}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = True
        config_path.stat.return_value.st_size = 1024

        with patch.object(config_module, "get_config", return_value=Config()):
            with patch.object(config_module, "config_file", return_value=config_path):
                console = Console(file=StringIO())
                with patch.object(config_module, "Console", return_value=console):
                    config_module.config_show_command(ctx)
                output = console.file.getvalue()
                assert "Config file location:" in output
                assert "File size:" in output

    def test_config_show_verbose_no_file(self):
        """Verbose mode when config file doesn't exist."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": True, "quiet": False}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = False

        with patch.object(config_module, "get_config", return_value=Config()):
            with patch.object(config_module, "config_file", return_value=config_path):
                console = Console(file=StringIO())
                with patch.object(config_module, "Console", return_value=console):
                    config_module.config_show_command(ctx)
                output = console.file.getvalue()
                assert "Using default configuration" in output


class TestConfigPathCommand:
    """Tests for config_path_command function."""

    def test_config_path_json_mode(self, capsys):
        """JSON mode outputs paths as JSON."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        with patch.object(config_module, "config_dir", return_value=Path("/config")):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                with patch.object(config_module, "cache_dir", return_value=Path("/cache")):
                    with patch.object(
                        config_module, "credentials_dir", return_value=Path("/credentials")
                    ):
                        config_module.config_path_command(ctx, False, False)
                        captured = capsys.readouterr()
                        assert '"config_dir"' in captured.out
                        assert '"cache_dir"' in captured.out

    def test_config_path_json_cache_only(self, capsys):
        """JSON mode with --cache flag outputs only cache dir."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        with patch.object(config_module, "cache_dir", return_value=Path("/cache")):
            config_module.config_path_command(ctx, True, False)
            captured = capsys.readouterr()
            assert '"cache_dir"' in captured.out
            assert '"/cache"' in captured.out

    def test_config_path_json_credentials_only(self, capsys):
        """JSON mode with --credentials flag outputs only credentials dir."""
        ctx = MagicMock()
        ctx.meta = {"json": True, "verbose": False, "quiet": False}

        with patch.object(config_module, "credentials_dir", return_value=Path("/credentials")):
            config_module.config_path_command(ctx, False, True)
            captured = capsys.readouterr()
            assert '"credentials_dir"' in captured.out
            assert '"/credentials"' in captured.out

    def test_config_path_quiet_mode(self):
        """Quiet mode shows only path without labels."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        with patch.object(config_module, "config_dir", return_value=Path("/config")):
            console = Console(file=StringIO())
            with patch.object(config_module, "Console", return_value=console):
                config_module.config_path_command(ctx, False, False)
            output = console.file.getvalue()
            assert "/config" in output

    def test_config_path_quiet_cache(self):
        """Quiet mode with --cache shows only cache dir path."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": True}

        with patch.object(config_module, "cache_dir", return_value=Path("/cache")):
            console = Console(file=StringIO())
            with patch.object(config_module, "Console", return_value=console):
                config_module.config_path_command(ctx, True, False)
            output = console.file.getvalue()
            assert "/cache" in output

    def test_config_path_normal_mode(self):
        """Normal mode shows all paths with labels."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": False, "quiet": False}

        with patch.object(config_module, "config_dir", return_value=Path("/config")):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                with patch.object(config_module, "cache_dir", return_value=Path("/cache")):
                    with patch.object(
                        config_module, "credentials_dir", return_value=Path("/credentials")
                    ):
                        console = Console(file=StringIO())
                        with patch.object(config_module, "Console", return_value=console):
                            config_module.config_path_command(ctx, False, False)
                        output = console.file.getvalue()
                        assert "Config dir:" in output
                        assert "Config file:" in output
                        assert "Cache dir:" in output
                        assert "Credentials:" in output

    def test_config_path_verbose_mode(self):
        """Verbose mode shows directory existence info."""
        ctx = MagicMock()
        ctx.meta = {"json": False, "verbose": True, "quiet": False}

        config_dir = MagicMock(spec=Path)
        config_dir.exists.return_value = True
        cache_dir = MagicMock(spec=Path)
        cache_dir.exists.return_value = False
        creds_dir = MagicMock(spec=Path)
        creds_dir.exists.return_value = True

        with patch.object(config_module, "config_dir", return_value=config_dir):
            with patch.object(config_module, "config_file", return_value=Path("/config/config.toml")):
                with patch.object(config_module, "cache_dir", return_value=cache_dir):
                    with patch.object(config_module, "credentials_dir", return_value=creds_dir):
                        console = Console(file=StringIO())
                        with patch.object(config_module, "Console", return_value=console):
                            config_module.config_path_command(ctx, False, False)
                        output = console.file.getvalue()
                        assert "Directory status:" in output
                        assert "Config dir exists: True" in output
                        assert "Cache dir exists: False" in output


class TestConfigResetCommand:
    """Tests for config_reset_command function."""

    def test_config_reset_json_mode_auto_confirms(self, capsys):
        """JSON mode auto-confirms without prompting."""
        ctx = MagicMock()
        ctx.meta = {"json": True}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = True

        with patch.object(config_module, "config_file", return_value=config_path):
            config_module.config_reset_command(ctx, False)
            captured = capsys.readouterr()
            assert '"success"' in captured.out
            assert '"reset"' in captured.out
            config_path.unlink.assert_called_once()

    def test_config_reset_confirmed(self):
        """Confirmed reset deletes config file."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = True

        with patch.object(config_module, "config_file", return_value=config_path):
            with patch.object(config_module, "typer") as mock_typer:
                mock_typer.confirm.return_value = True
                config_module.config_reset_command(ctx, False)
                mock_typer.confirm.assert_called_once()
                config_path.unlink.assert_called_once()

    def test_config_reset_cancelled(self):
        """Cancelled reset exits without deleting."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        config_path = MagicMock(spec=Path)

        with patch.object(config_module, "config_file", return_value=config_path):
            with patch("typer.confirm", return_value=False):
                with pytest.raises(TyperExit):
                    config_module.config_reset_command(ctx, False)
                config_path.unlink.assert_not_called()

    def test_config_reset_no_file(self, capsys):
        """Reset with no custom file shows message."""
        ctx = MagicMock()
        ctx.meta = {"json": True}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = False

        with patch.object(config_module, "config_file", return_value=config_path):
            config_module.config_reset_command(ctx, True)
            captured = capsys.readouterr()
            assert '"success"' in captured.out
            assert '"reset"' in captured.out
            assert 'false' in captured.out.lower()

    def test_config_reset_with_confirm_flag(self):
        """--confirm flag skips prompt."""
        ctx = MagicMock()
        ctx.meta = {"json": False}

        config_path = MagicMock(spec=Path)
        config_path.exists.return_value = True

        with patch.object(config_module, "config_file", return_value=config_path):
            with patch.object(config_module, "typer") as mock_typer:
                config_module.config_reset_command(ctx, True)
                # Should not call confirm with --confirm flag
                mock_typer.confirm.assert_not_called()
                config_path.unlink.assert_called_once()


class TestConfigEditCommand:
    """Tests for config_edit_command function."""

    def test_config_edit_creates_default(self):
        """Creates default config if it doesn't exist."""
        cfg_path = MagicMock(spec=Path)
        cfg_path.exists.return_value = False
        cfg_dir = MagicMock(spec=Path)

        with patch.object(config_module, "config_file", return_value=cfg_path):
            with patch.object(config_module, "config_dir", return_value=cfg_dir):
                with patch.dict("os.environ", {"EDITOR": "vi"}):
                    with patch("subprocess.run") as mock_run:
                        config_module.config_edit_command()
                        cfg_dir.mkdir.assert_called_once_with(parents=True, exist_ok=True)
                        cfg_path.write_bytes.assert_called_once()
                        mock_run.assert_called_once_with(["vi", str(cfg_path)], check=True)

    def test_config_edit_editor_not_found(self):
        """Exits with CONFIG_ERROR when editor not found."""
        cfg_path = MagicMock(spec=Path)
        cfg_path.exists.return_value = True
        cfg_dir = MagicMock(spec=Path)

        with patch.object(config_module, "config_file", return_value=cfg_path):
            with patch.object(config_module, "config_dir", return_value=cfg_dir):
                with patch.dict("os.environ", {}, clear=True):
                    with patch("subprocess.run", side_effect=FileNotFoundError):
                        with pytest.raises(TyperExit) as exc_info:
                            config_module.config_edit_command()
                        assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_config_edit_editor_error(self):
        """Exits with GENERAL_ERROR when editor exits with error."""
        import subprocess

        cfg_path = MagicMock(spec=Path)
        cfg_path.exists.return_value = True
        cfg_dir = MagicMock(spec=Path)

        with patch.object(config_module, "config_file", return_value=cfg_path):
            with patch.object(config_module, "config_dir", return_value=cfg_dir):
                with patch.dict("os.environ", {"EDITOR": "vi"}):
                    with patch("subprocess.run", side_effect=subprocess.CalledProcessError(1, "vi")):
                        with pytest.raises(TyperExit) as exc_info:
                            config_module.config_edit_command()
                        assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR


class TestConfigCallback:
    """Tests for config_callback function."""

    def test_config_callback_stores_context(self):
        """Callback stores context for subcommands."""
        ctx = MagicMock()
        # Should not raise any errors
        config_module.config_callback(ctx)


class TestConfigAppRegistration:
    """Tests for config_app ATyper setup."""

    def test_config_app_exists(self):
        """config_app should be properly defined."""
        assert hasattr(config_module, "config_app")
        assert config_module.config_app.info.help == "Manage configuration settings."

    def test_config_show_command_exists(self):
        """config_show_command should be registered."""
        assert hasattr(config_module, "config_show_command")

    def test_config_path_command_exists(self):
        """config_path_command should be registered."""
        assert hasattr(config_module, "config_path_command")

    def test_config_reset_command_exists(self):
        """config_reset_command should be registered."""
        assert hasattr(config_module, "config_reset_command")

    def test_config_edit_command_exists(self):
        """config_edit_command should be registered."""
        assert hasattr(config_module, "config_edit_command")
