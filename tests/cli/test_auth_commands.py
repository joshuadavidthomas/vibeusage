"""Tests for auth CLI commands."""

from __future__ import annotations

from io import StringIO
from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest
from rich.console import Console
from typer import Exit as TyperExit

from vibeusage.cli.app import ExitCode
from vibeusage.cli.commands import auth as auth_module


class TestAuthCommand:
    """Tests for auth_command function (main entry point)."""

    def test_auth_command_no_provider_shows_status(self):
        """No provider argument calls auth_status_command."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_status_command", return_value=None
        ) as mock_status:
            auth_module.auth_command(ctx, None, False, False, False)
            assert mock_status.called
            call_kwargs = mock_status.call_args.kwargs
            assert call_kwargs["show_all"] is False
            assert call_kwargs["verbose"] is False
            assert call_kwargs["quiet"] is False

    def test_auth_command_with_status_flag(self):
        """Deprecated --status flag calls status with show_all=True."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_status_command", return_value=None
        ) as mock_status:
            auth_module.auth_command(ctx, None, True, False, False)
            assert mock_status.called
            call_kwargs = mock_status.call_args.kwargs
            assert call_kwargs["show_all"] is True

    def test_auth_command_with_provider_claude(self):
        """Provider='claude' calls auth_claude_command."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_claude_command", return_value=None
        ) as mock_claude:
            auth_module.auth_command(ctx, "claude", False, False, False)
            mock_claude.assert_called_once_with(verbose=False, quiet=False)

    def test_auth_command_with_provider_generic(self):
        """Other providers call auth_generic_command."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_generic_command", return_value=None
        ) as mock_generic:
            auth_module.auth_command(ctx, "codex", False, False, False)
            mock_generic.assert_called_once_with("codex", verbose=False, quiet=False)

    def test_auth_command_invalid_provider(self):
        """Invalid provider prints error and exits with CONFIG_ERROR."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "list_provider_ids", return_value=["claude", "codex"]
        ):
            console = Console(file=StringIO())
            with patch.object(auth_module, "Console", return_value=console):
                with pytest.raises(TyperExit) as exc_info:
                    auth_module.auth_command(ctx, "invalid", False, False, False)
                assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_auth_command_context_json_mode(self):
        """Uses json from context meta."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": True}

        with patch.object(
            auth_module, "auth_status_command", return_value=None
        ) as mock_status:
            auth_module.auth_command(ctx, None, False, False, False)
            assert mock_status.called
            call_kwargs = mock_status.call_args.kwargs
            assert call_kwargs["json_mode"] is True

    def test_auth_command_json_option_overrides_context(self):
        """Local --json option overrides context.json."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_status_command", return_value=None
        ) as mock_status:
            auth_module.auth_command(ctx, None, False, False, True)
            assert mock_status.called
            call_kwargs = mock_status.call_args.kwargs
            assert call_kwargs["json_mode"] is True


class TestAuthStatusCommand:
    """Tests for auth_status_command function."""

    def test_auth_status_json_mode(self):
        """JSON output with provider data."""
        with patch.object(
            auth_module, "list_provider_ids", return_value=["claude", "codex"]
        ):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                side_effect=[(True, "vibeusage"), (False, None)],
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    side_effect=[
                        ("oauth", Path("/cred1"), Path("/cred1")),
                        (None, None, None),
                    ],
                ):
                    with patch(
                        "vibeusage.display.json.output_json_pretty"
                    ) as mock_json:
                        auth_module.auth_status_command(
                            show_all=False, json_mode=True, verbose=False, quiet=False
                        )
                        mock_json.assert_called_once()
                        result = mock_json.call_args[0][0]
                        assert "claude" in result
                        assert result["claude"]["authenticated"] is True
                        assert result["claude"]["source"] == "vibeusage storage"
                        assert "codex" in result
                        assert result["codex"]["authenticated"] is False

    def test_auth_status_json_mode_provider_cli_source(self):
        """JSON mode with provider_cli source."""
        with patch.object(auth_module, "list_provider_ids", return_value=["codex"]):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                return_value=(True, "provider_cli"),
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("provider_cli", Path("/cli/cred"), Path("/cli/cred")),
                ):
                    with patch(
                        "vibeusage.display.json.output_json_pretty"
                    ) as mock_json:
                        auth_module.auth_status_command(
                            show_all=False, json_mode=True, verbose=False, quiet=False
                        )
                        result = mock_json.call_args[0][0]
                        assert result["codex"]["source"] == "provider CLI"

    def test_auth_status_json_mode_env_source(self):
        """JSON mode with env source."""
        with patch.object(auth_module, "list_provider_ids", return_value=["gemini"]):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(True, "env")
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("env", Path("/env/cred"), Path("/env/cred")),
                ):
                    with patch(
                        "vibeusage.display.json.output_json_pretty"
                    ) as mock_json:
                        auth_module.auth_status_command(
                            show_all=False, json_mode=True, verbose=False, quiet=False
                        )
                        result = mock_json.call_args[0][0]
                        assert result["gemini"]["source"] == "environment variable"

    def test_auth_status_json_mode_unknown_source(self):
        """Unknown source value falls back to itself."""
        with patch.object(auth_module, "list_provider_ids", return_value=["test"]):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                return_value=(True, "unknown_source"),
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("unknown", Path("/cred"), Path("/cred")),
                ):
                    with patch(
                        "vibeusage.display.json.output_json_pretty"
                    ) as mock_json:
                        auth_module.auth_status_command(
                            show_all=False, json_mode=True, verbose=False, quiet=False
                        )
                        result = mock_json.call_args[0][0]
                        assert result["test"]["source"] == "unknown_source"

    def test_auth_status_quiet_mode(self):
        """Minimal quiet output."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude", "codex"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[(True, "vibeusage"), (False, None)],
                ):
                    auth_module.auth_status_command(
                        show_all=False, json_mode=False, verbose=False, quiet=True
                    )
                    output = console.file.getvalue()
                    assert "claude: authenticated" in output
                    assert "codex: not configured" in output

    def test_auth_status_normal_mode(self):
        """Rich table output."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude", "codex"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[
                        (True, "vibeusage"),
                        (False, None),
                        (False, None),
                        (False, None),
                    ],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            ("oauth", Path("/cred"), Path("/cred")),
                            (None, None, None),
                            (None, None, None),
                            (None, None, None),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "claude" in output
                        assert "codex" in output

    def test_auth_status_normal_mode_authenticated(self):
        """Authenticated provider shows correct status in table."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[
                        (True, "vibeusage"),
                        (True, "vibeusage"),
                        (True, "vibeusage"),
                    ],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            ("oauth", Path("/cred"), Path("/cred")),
                            ("oauth", Path("/cred"), Path("/cred")),
                            ("oauth", Path("/cred"), Path("/cred")),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert (
                            "Authenticated" in output or "vibeusage storage" in output
                        )

    def test_auth_status_normal_mode_unconfigured(self):
        """Unconfigured provider shows correct status in table."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["copilot"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[(False, None), (False, None), (False, None)],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            (None, None, None),
                            (None, None, None),
                            (None, None, None),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert (
                            "Not configured" in output
                            or "not configured" in output.lower()
                        )

    def test_auth_status_shows_unconfigured_instructions(self):
        """Shows setup instructions for unconfigured providers."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude", "codex"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[
                        (True, "vibeusage"),
                        (False, None),
                        (False, None),
                        (False, None),
                    ],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            ("oauth", Path("/cred"), Path("/cred")),
                            (None, None, None),
                            (None, None, None),
                            (None, None, None),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert ("configure" in output.lower()) and "codex" in output

    def test_auth_status_verbose_shows_paths(self):
        """Verbose shows credential paths."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude"]
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[
                        (True, "vibeusage"),
                        (True, "vibeusage"),
                        (True, "vibeusage"),
                    ],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            ("oauth", Path("/test/cred.json"), Path("/test/cred.json")),
                            ("oauth", Path("/test/cred.json"), Path("/test/cred.json")),
                            ("oauth", Path("/test/cred.json"), Path("/test/cred.json")),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=True, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "Credential Paths" in output
                        assert "/test/cred.json" in output

    def test_auth_status_verbose_none_paths(self):
        """Verbose handles None credential paths."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(auth_module, "list_provider_ids", return_value=["codex"]):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[(False, None), (False, None), (False, None)],
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[
                            (None, None, None),
                            (None, None, None),
                            (None, None, None),
                        ],
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=True, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "Credential Paths" in output
                        assert "none" in output

    def test_auth_status_all_providers_unconfigured(self):
        """All providers unconfigured shows instructions for all."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "list_provider_ids",
                return_value=["claude", "codex", "copilot"],
            ):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    side_effect=[(False, None)] * 9,
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        side_effect=[(None, None, None)] * 9,
                    ):
                        auth_module.auth_status_command(
                            show_all=False, json_mode=False, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "claude" in output
                        assert "codex" in output
                        assert "copilot" in output


class TestAuthClaudeCommand:
    """Tests for auth_claude_command function."""

    def test_auth_claude_with_session_key(self):
        """Valid session key saves successfully."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_claude_command(
                        session_key="sk-ant-sid01-test123", verbose=False, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "Success" in output

    def test_auth_claude_no_session_key_prompts(self):
        """No session key shows instructions and prompts when browser extraction fails."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "_try_browser_cookie_extraction",
                return_value=None,
            ):
                with patch("typer.prompt", return_value="sk-ant-sid01-test123"):
                    with patch.object(
                        auth_module, "credential_path", return_value=Path("/cred.json")
                    ):
                        with patch.object(auth_module, "write_credential"):
                            auth_module.auth_claude_command(
                                session_key=None, verbose=False, quiet=False
                            )
                            output = console.file.getvalue()
                            assert (
                                "Instructions" in output
                                or "Claude Authentication" in output
                            )

    def test_auth_claude_browser_extraction_success(self):
        """Browser extraction skips prompt when cookie found."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "_try_browser_cookie_extraction",
                return_value="sk-ant-sid01-extracted",
            ):
                with patch.object(
                    auth_module, "credential_path", return_value=Path("/cred.json")
                ):
                    with patch.object(auth_module, "write_credential"):
                        auth_module.auth_claude_command(
                            session_key=None, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "Success" in output
                        # Should NOT show instructions since browser extraction worked
                        assert "Instructions" not in output

    def test_auth_claude_invalid_format_warns(self):
        """Invalid format shows warning."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch("typer.confirm", return_value=False):
                with pytest.raises(TyperExit) as exc_info:
                    auth_module.auth_claude_command(
                        session_key="invalid-key", verbose=False, quiet=False
                    )
                assert exc_info.value.exit_code == ExitCode.AUTH_ERROR
                output = console.file.getvalue()
                assert "Warning" in output

    def test_auth_claude_invalid_format_confirmed(self):
        """Invalid format with user confirmation saves."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch("typer.confirm", return_value=True):
                with patch.object(
                    auth_module, "credential_path", return_value=Path("/cred.json")
                ):
                    with patch.object(auth_module, "write_credential"):
                        auth_module.auth_claude_command(
                            session_key="invalid-format-key", verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "Success" in output

    def test_auth_claude_verbose_shows_prefix(self):
        """Verbose mode shows session key prefix."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_claude_command(
                        session_key="sk-ant-sid01-1234567890abcdefghijklmnop",
                        verbose=True,
                        quiet=False,
                    )
                    output = console.file.getvalue()
                    # The code shows first 20 chars: session_key[:20] + "..."
                    assert "sk-ant-sid01-1234567..." in output

    def test_auth_claude_quiet_no_output(self):
        """Quiet mode suppresses success output."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_claude_command(
                        session_key="sk-ant-sid01-test", verbose=False, quiet=True
                    )
                    output = console.file.getvalue()
                    assert "Success" not in output

    def test_auth_claude_write_error(self):
        """Exception during write is handled."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(
                    auth_module, "write_credential", side_effect=IOError("Write failed")
                ):
                    with pytest.raises(TyperExit) as exc_info:
                        auth_module.auth_claude_command(
                            session_key="sk-ant-sid01-test", verbose=False, quiet=False
                        )
                    assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR
                    output = console.file.getvalue()
                    assert "Error saving credential" in output

    def test_auth_claude_quiet_write_error(self):
        """Exception in quiet mode still exits."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(
                    auth_module, "write_credential", side_effect=IOError("Write failed")
                ):
                    with pytest.raises(TyperExit) as exc_info:
                        auth_module.auth_claude_command(
                            session_key="sk-ant-sid01-test", verbose=False, quiet=True
                        )
                    assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR


class TestAuthCursorCommand:
    """Tests for auth_cursor_command function."""

    def test_auth_cursor_with_session_token(self):
        """Valid session token saves successfully."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_cursor_command(
                        session_token="cursor-test-token", verbose=False, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "Success" in output

    def test_auth_cursor_no_token_browser_extraction_success(self):
        """Browser extraction skips prompt when cookie found."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "_try_browser_cookie_extraction",
                return_value="extracted-cursor-token",
            ):
                with patch.object(
                    auth_module, "credential_path", return_value=Path("/cred.json")
                ):
                    with patch.object(auth_module, "write_credential"):
                        auth_module.auth_cursor_command(
                            session_token=None, verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "Success" in output
                        assert "Instructions" not in output

    def test_auth_cursor_no_token_prompts_on_extraction_fail(self):
        """Falls back to manual prompt when browser extraction fails."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "_try_browser_cookie_extraction",
                return_value=None,
            ):
                with patch("typer.prompt", return_value="manual-cursor-token"):
                    with patch.object(
                        auth_module, "credential_path", return_value=Path("/cred.json")
                    ):
                        with patch.object(auth_module, "write_credential"):
                            auth_module.auth_cursor_command(
                                session_token=None, verbose=False, quiet=False
                            )
                            output = console.file.getvalue()
                            assert "Cursor Authentication" in output
                            assert "Success" in output

    def test_auth_cursor_verbose_shows_prefix(self):
        """Verbose mode shows session token prefix."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_cursor_command(
                        session_token="cursor-session-1234567890abcdef",
                        verbose=True,
                        quiet=False,
                    )
                    output = console.file.getvalue()
                    assert "cursor-session-12345..." in output

    def test_auth_cursor_quiet_no_output(self):
        """Quiet mode suppresses success output."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(auth_module, "write_credential"):
                    auth_module.auth_cursor_command(
                        session_token="test-token", verbose=False, quiet=True
                    )
                    output = console.file.getvalue()
                    assert "Success" not in output

    def test_auth_cursor_write_error(self):
        """Exception during write is handled."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "credential_path", return_value=Path("/cred.json")
            ):
                with patch.object(
                    auth_module,
                    "write_credential",
                    side_effect=OSError("Write failed"),
                ):
                    with pytest.raises(TyperExit) as exc_info:
                        auth_module.auth_cursor_command(
                            session_token="test-token", verbose=False, quiet=False
                        )
                    assert exc_info.value.exit_code == ExitCode.GENERAL_ERROR
                    output = console.file.getvalue()
                    assert "Error saving credential" in output


class TestAuthCommandRoutesCursor:
    """Tests for auth_command routing cursor to auth_cursor_command."""

    def test_auth_command_with_provider_cursor(self):
        """Provider='cursor' calls auth_cursor_command."""
        ctx = MagicMock()
        ctx.meta = {"verbose": False, "quiet": False, "json": False}

        with patch.object(
            auth_module, "auth_cursor_command", return_value=None
        ) as mock_cursor:
            auth_module.auth_command(ctx, "cursor", False, False, False)
            mock_cursor.assert_called_once_with(verbose=False, quiet=False)


class TestTryBrowserCookieExtraction:
    """Tests for _try_browser_cookie_extraction helper."""

    def test_returns_none_when_no_library(self):
        """Returns None when browser_cookie3 not importable."""
        import builtins

        original_import = builtins.__import__

        def mock_import(name, *args, **kwargs):
            if name in ("browser_cookie3", "pycookiecheat"):
                raise ImportError(f"No module named '{name}'")
            return original_import(name, *args, **kwargs)

        console = Console(file=StringIO())
        with patch("builtins.__import__", side_effect=mock_import):
            result = auth_module._try_browser_cookie_extraction(
                console,
                provider="test",
                cookie_domains=[".test.com"],
                cookie_names=["testCookie"],
                verbose=True,
                quiet=False,
            )

        assert result is None
        output = console.file.getvalue()
        assert "not available" in output

    def test_returns_cookie_value_on_success(self):
        """Returns cookie value when found."""
        mock_cookie = MagicMock()
        mock_cookie.name = "sessionKey"
        mock_cookie.value = "extracted-value"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.chrome.return_value = [mock_cookie]

        console = Console(file=StringIO())
        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = auth_module._try_browser_cookie_extraction(
                console,
                provider="claude",
                cookie_domains=[".claude.ai"],
                cookie_names=["sessionKey"],
                verbose=False,
                quiet=False,
            )

        assert result == "extracted-value"
        output = console.file.getvalue()
        assert "Found" in output

    def test_returns_none_when_no_cookie_found(self):
        """Returns None when no matching cookie found."""
        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.safari = None
        mock_browser_cookie3.chrome.return_value = []
        mock_browser_cookie3.firefox.return_value = []
        mock_browser_cookie3.brave = None
        mock_browser_cookie3.edge = None
        mock_browser_cookie3.arc = None

        console = Console(file=StringIO())
        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = auth_module._try_browser_cookie_extraction(
                console,
                provider="test",
                cookie_domains=[".test.com"],
                cookie_names=["testCookie"],
                verbose=True,
                quiet=False,
            )

        assert result is None
        output = console.file.getvalue()
        assert "No session cookie found" in output

    def test_quiet_suppresses_output(self):
        """Quiet mode suppresses extraction messages."""
        mock_cookie = MagicMock()
        mock_cookie.name = "sessionKey"
        mock_cookie.value = "extracted-value"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.chrome.return_value = [mock_cookie]

        console = Console(file=StringIO())
        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = auth_module._try_browser_cookie_extraction(
                console,
                provider="claude",
                cookie_domains=[".claude.ai"],
                cookie_names=["sessionKey"],
                verbose=False,
                quiet=True,
            )

        assert result == "extracted-value"
        output = console.file.getvalue()
        assert output == ""

    def test_handles_browser_exception_gracefully(self):
        """Continues to next browser when one throws."""
        mock_cookie = MagicMock()
        mock_cookie.name = "testCookie"
        mock_cookie.value = "from-firefox"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.safari = None
        mock_browser_cookie3.chrome.side_effect = Exception("DB locked")
        mock_browser_cookie3.firefox.return_value = [mock_cookie]

        console = Console(file=StringIO())
        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = auth_module._try_browser_cookie_extraction(
                console,
                provider="test",
                cookie_domains=[".test.com"],
                cookie_names=["testCookie"],
                verbose=False,
                quiet=False,
            )

        assert result == "from-firefox"


class TestShowCursorAuthInstructions:
    """Tests for _show_cursor_auth_instructions function."""

    def test_show_cursor_instructions_normal(self):
        """Instructions are displayed."""
        console = Console(file=StringIO())
        auth_module._show_cursor_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert "Cursor Authentication" in output
        assert "cursor.com" in output
        assert "WorkosCursorSessionToken" in output

    def test_show_cursor_instructions_quiet(self):
        """Quiet mode suppresses output."""
        console = Console(file=StringIO())
        auth_module._show_cursor_auth_instructions(console, quiet=True)
        output = console.file.getvalue()
        assert output == ""


class TestAuthGenericCommand:
    """Tests for auth_generic_command function."""

    def test_auth_generic_authenticated_vibeusage(self):
        """Already authenticated from vibeusage storage."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                return_value=(True, "vibeusage"),
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("oauth", Path("/cred"), Path("/cred")),
                ):
                    auth_module.auth_generic_command(
                        "codex", verbose=False, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "codex is already authenticated" in output
                    assert "vibeusage storage" in output

    def test_auth_generic_authenticated_provider_cli(self):
        """Already authenticated from provider CLI."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                return_value=(True, "provider_cli"),
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("provider_cli", Path("/cli/cred"), Path("/cli/cred")),
                ):
                    auth_module.auth_generic_command(
                        "codex", verbose=False, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "codex is already authenticated" in output
                    assert "provider CLI" in output

    def test_auth_generic_authenticated_env(self):
        """Already authenticated from env var."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(True, "env")
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=("env", Path("/env"), Path("/env")),
                ):
                    auth_module.auth_generic_command(
                        "gemini", verbose=False, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "gemini is already authenticated" in output
                    assert "environment variable" in output

    def test_auth_generic_authenticated_unknown_source(self):
        """Unknown source value handling."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(auth_module, "list_provider_ids", return_value=["test"]):
                with patch.object(
                    auth_module,
                    "check_provider_credentials",
                    return_value=(True, "unknown_source"),
                ):
                    with patch.object(
                        auth_module,
                        "find_provider_credential",
                        return_value=("unknown", Path("/cred"), Path("/cred")),
                    ):
                        auth_module.auth_generic_command(
                            "test", verbose=False, quiet=False
                        )
                        output = console.file.getvalue()
                        assert "test is already authenticated" in output

    def test_auth_generic_authenticated_verbose(self):
        """Verbose shows credential path."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module,
                "check_provider_credentials",
                return_value=(True, "vibeusage"),
            ):
                with patch.object(
                    auth_module,
                    "find_provider_credential",
                    return_value=(
                        "oauth",
                        Path("/test/cred.json"),
                        Path("/test/cred.json"),
                    ),
                ):
                    auth_module.auth_generic_command(
                        "cursor", verbose=True, quiet=False
                    )
                    output = console.file.getvalue()
                    assert "Location:" in output
                    assert "/test/cred.json" in output

    def test_auth_generic_not_authenticated_shows_instructions(self):
        """Not authenticated shows instructions."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(False, None)
            ):
                auth_module.auth_generic_command("copilot", verbose=False, quiet=False)
                output = console.file.getvalue()
                assert "Instructions" in output
                assert "GitHub Copilot Authentication" in output

    def test_auth_generic_invalid_provider(self):
        """Invalid provider check (duplicate validation)."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "list_provider_ids", return_value=["claude", "codex"]
            ):
                with pytest.raises(TyperExit) as exc_info:
                    auth_module.auth_generic_command(
                        "invalid", verbose=False, quiet=False
                    )
                assert exc_info.value.exit_code == ExitCode.CONFIG_ERROR

    def test_auth_generic_codex_instructions(self):
        """Codex shows specific instructions."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(False, None)
            ):
                auth_module.auth_generic_command("codex", verbose=False, quiet=False)
                output = console.file.getvalue()
                assert "Codex" in output

    def test_auth_generic_copilot_instructions(self):
        """Copilot shows specific instructions."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(False, None)
            ):
                auth_module.auth_generic_command("copilot", verbose=False, quiet=False)
                output = console.file.getvalue()
                assert "GitHub Copilot Authentication" in output

    def test_auth_generic_cursor_instructions(self):
        """Cursor shows specific instructions."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(False, None)
            ):
                auth_module.auth_generic_command("cursor", verbose=False, quiet=False)
                output = console.file.getvalue()
                assert "Cursor Authentication" in output
                assert "cursor.com" in output

    def test_auth_generic_gemini_instructions(self):
        """Gemini shows specific instructions."""
        console = Console(file=StringIO())
        with patch.object(auth_module, "Console", return_value=console):
            with patch.object(
                auth_module, "check_provider_credentials", return_value=(False, None)
            ):
                auth_module.auth_generic_command("gemini", verbose=False, quiet=False)
                output = console.file.getvalue()
                assert "Gemini Authentication" in output


class TestShowClaudeAuthInstructions:
    """Tests for _show_claude_auth_instructions function."""

    def test_show_claude_instructions_normal(self):
        """Instructions are displayed."""
        console = Console(file=StringIO())
        auth_module._show_claude_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert "Claude Authentication" in output
        assert "claude.ai" in output
        assert "sessionKey" in output

    def test_show_claude_instructions_quiet(self):
        """Quiet mode suppresses output."""
        console = Console(file=StringIO())
        auth_module._show_claude_auth_instructions(console, quiet=True)
        output = console.file.getvalue()
        assert output == ""


class TestShowProviderAuthInstructions:
    """Tests for _show_provider_auth_instructions function."""

    def test_show_instructions_codex(self):
        """Codex instructions displayed."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=False)
        output = console.file.getvalue()
        assert "Codex" in output

    def test_show_instructions_copilot(self):
        """Copilot instructions displayed."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "copilot", quiet=False)
        output = console.file.getvalue()
        assert "GitHub Copilot Authentication" in output

    def test_show_instructions_cursor(self):
        """Cursor instructions displayed."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "cursor", quiet=False)
        output = console.file.getvalue()
        assert "Cursor Authentication" in output
        assert "cursor.com" in output

    def test_show_instructions_gemini(self):
        """Gemini instructions displayed."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "Gemini Authentication" in output

    def test_show_instructions_unknown_provider(self):
        """Generic template for unknown provider."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(
            console, "unknownprovider", quiet=False
        )
        output = console.file.getvalue()
        assert "authentication" in output.lower()

    def test_show_instructions_quiet(self):
        """Quiet mode suppresses output."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=True)
        output = console.file.getvalue()
        assert output == ""


class TestImprovedClaudeAuthInstructions:
    """Tests for improved Claude auth instructions (Phase 9.3)."""

    def test_claude_instructions_show_cookie_format(self):
        """Claude instructions mention expected cookie format prefix."""
        console = Console(file=StringIO())
        auth_module._show_claude_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert "sk-ant-sid01-" in output

    def test_claude_instructions_mention_expiration(self):
        """Claude instructions mention cookie expiration behavior."""
        console = Console(file=StringIO())
        auth_module._show_claude_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert "expir" in output.lower()

    def test_claude_instructions_mention_alternative_credential_path(self):
        """Claude instructions mention the CLI credential path as alternative."""
        console = Console(file=StringIO())
        auth_module._show_claude_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert ".claude" in output


class TestImprovedCodexAuthInstructions:
    """Tests for improved Codex auth instructions (Phase 9.3)."""

    def test_codex_instructions_show_file_format(self):
        """Codex instructions show the auth.json file format."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=False)
        output = console.file.getvalue()
        assert "auth.json" in output

    def test_codex_instructions_show_credential_path(self):
        """Codex instructions show the ~/.codex/ credential path."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=False)
        output = console.file.getvalue()
        assert "~/.codex/" in output

    def test_codex_instructions_show_token_structure(self):
        """Codex instructions explain token JSON structure."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=False)
        output = console.file.getvalue()
        assert "access_token" in output

    def test_codex_instructions_mention_env_var(self):
        """Codex instructions mention the OPENAI_API_KEY env var."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "codex", quiet=False)
        output = console.file.getvalue()
        assert "OPENAI_API_KEY" in output


class TestImprovedCursorAuthInstructions:
    """Tests for improved Cursor auth instructions (Phase 9.3)."""

    def test_cursor_instructions_mention_format_hint(self):
        """Cursor instructions mention expected token format."""
        console = Console(file=StringIO())
        auth_module._show_cursor_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        # Should mention it's a long encoded string
        assert (
            "long" in output.lower()
            or "encoded" in output.lower()
            or "JWT" in output
            or "eyJ" in output
        )

    def test_cursor_instructions_mention_all_cookie_names(self):
        """Cursor instructions list all possible cookie names."""
        console = Console(file=StringIO())
        auth_module._show_cursor_auth_instructions(console, quiet=False)
        output = console.file.getvalue()
        assert "WorkosCursorSessionToken" in output
        assert "__Secure-next-auth.session-token" in output
        assert "next-auth.session-token" in output


class TestImprovedGeminiAuthInstructions:
    """Tests for improved Gemini auth instructions (Phase 9.3)."""

    def test_gemini_instructions_show_credential_path(self):
        """Gemini instructions show the ~/.gemini/ credential path."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "~/.gemini/" in output

    def test_gemini_instructions_show_file_format(self):
        """Gemini instructions show the oauth_creds.json file format."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "oauth_creds.json" in output

    def test_gemini_instructions_show_token_structure(self):
        """Gemini instructions explain token JSON structure."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "access_token" in output
        assert "refresh_token" in output

    def test_gemini_instructions_mention_env_var(self):
        """Gemini instructions mention the GEMINI_API_KEY env var."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "GEMINI_API_KEY" in output

    def test_gemini_instructions_mention_api_key_alternative(self):
        """Gemini instructions mention API key as alternative auth method."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "gemini", quiet=False)
        output = console.file.getvalue()
        assert "API key" in output or "api_key" in output


class TestImprovedCopilotAuthInstructions:
    """Tests for improved Copilot auth instructions (Phase 9.3)."""

    def test_copilot_instructions_mention_hosts_json(self):
        """Copilot instructions mention VS Code hosts.json location."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "copilot", quiet=False)
        output = console.file.getvalue()
        assert "hosts.json" in output

    def test_copilot_instructions_show_credential_path(self):
        """Copilot instructions show the github-copilot config path."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "copilot", quiet=False)
        output = console.file.getvalue()
        assert "github-copilot" in output

    def test_copilot_instructions_explain_token_format(self):
        """Copilot instructions explain token format."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "copilot", quiet=False)
        output = console.file.getvalue()
        assert "gho_" in output or "ghu_" in output

    def test_copilot_instructions_mention_device_flow(self):
        """Copilot instructions mention the preferred device flow auth."""
        console = Console(file=StringIO())
        auth_module._show_provider_auth_instructions(console, "copilot", quiet=False)
        output = console.file.getvalue()
        assert "vibeusage auth copilot" in output
