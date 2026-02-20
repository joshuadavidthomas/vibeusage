"""Authentication commands for vibeusage."""

from __future__ import annotations

import json

import typer
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.config.credentials import check_provider_credentials
from vibeusage.config.credentials import credential_path
from vibeusage.config.credentials import find_provider_credential
from vibeusage.config.credentials import write_credential
from vibeusage.providers import list_provider_ids


@app.command("auth")
async def auth_command(
    ctx: typer.Context,
    provider: str = typer.Argument(
        None,
        help="Provider to authenticate with",
    ),
    status: bool = typer.Option(
        False,
        "--status",
        help="Show authentication status",
    ),
    show_all: bool = typer.Option(
        False,
        "--all",
        "-a",
        help="Show detailed status for all providers",
    ),
    json_output: bool = typer.Option(
        False,
        "--json",
        "-j",
        help="Output in JSON format",
    ),
) -> None:
    """Authenticate with a provider or show auth status.

    Without arguments, shows auth status for all providers.
    With provider name, starts provider-specific auth flow.
    """
    console = Console()

    # Get verbose/quiet from context
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    # Check for JSON mode (from global flag or local option)
    json_mode = json_output or ctx.meta.get("json", False)

    # Handle --status flag (deprecated, use --all instead)
    if status:
        auth_status_command(
            show_all=True, json_mode=json_mode, verbose=verbose, quiet=quiet
        )
        return

    # No provider - show status
    if provider is None:
        auth_status_command(
            show_all=show_all, json_mode=json_mode, verbose=verbose, quiet=quiet
        )
        return

    # Validate provider
    if provider not in list_provider_ids():
        if not quiet:
            console.print(f"[red]Unknown provider:[/red] {provider}")
            console.print(
                f"Available providers: {', '.join(sorted(list_provider_ids()))}"
            )
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    # Provider-specific auth flows
    if provider == "claude":
        auth_claude_command(verbose=verbose, quiet=quiet)
    elif provider == "cursor":
        auth_cursor_command(verbose=verbose, quiet=quiet)
    elif provider == "copilot":
        await auth_copilot_command(verbose=verbose, quiet=quiet)
    else:
        auth_generic_command(provider, verbose=verbose, quiet=quiet)


def auth_status_command(
    show_all: bool = False,
    json_mode: bool = False,
    verbose: bool = False,
    quiet: bool = False,
) -> None:
    """Show authentication status for all providers."""
    console = Console()

    all_providers = list_provider_ids()

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        data = {}
        for provider_id in sorted(all_providers):
            has_creds, source = check_provider_credentials(provider_id)
            _, _, cred_path = find_provider_credential(provider_id)

            source_label = {
                "vibeusage": "vibeusage storage",
                "provider_cli": "provider CLI",
                "env": "environment variable",
            }.get(source or "", source or "unknown")

            data[provider_id] = {
                "authenticated": has_creds,
                "source": source_label if has_creds else None,
                "credential_path": str(cred_path) if cred_path else None,
            }

        output_json_pretty(data)
        return

    # Quiet mode: minimal output
    if quiet:
        for provider_id in sorted(all_providers):
            has_creds, source = check_provider_credentials(provider_id)
            status = "authenticated" if has_creds else "not configured"
            console.print(f"{provider_id}: {status}")
        return

    table = Table(title="Authentication Status", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Status", style="bold")
    table.add_column("Source", style="dim")
    table.add_column("Details", style="dim")

    for provider_id in sorted(all_providers):
        has_creds, source = check_provider_credentials(provider_id)

        if has_creds:
            status_text = "[green]Authenticated[/green]"
            source_label = {
                "vibeusage": "vibeusage storage",
                "provider_cli": "provider CLI",
                "env": "environment variable",
            }.get(source or "", source or "unknown")

            # Get credential path for details
            _, _, cred_path = find_provider_credential(provider_id)
            details = cred_path or "—"
        else:
            status_text = "[yellow]Not configured[/yellow]"
            source_label = "—"
            details = "—"

        table.add_row(provider_id, status_text, source_label, str(details))

    console.print(table)

    # Show setup instructions for unconfigured providers
    unconfigured = [p for p in all_providers if not check_provider_credentials(p)[0]]
    if unconfigured:
        console.print("\n[dim]To configure a provider, run:[/dim]")
        for provider_id in unconfigured:
            console.print(f"  [dim]vibeusage auth {provider_id}[/dim]")

    # Verbose: show credential paths
    if verbose:
        console.print("\n[bold]Credential Paths:[/bold]")
        for provider_id in sorted(all_providers):
            _, _, cred_path = find_provider_credential(provider_id)
            if cred_path:
                console.print(f"  {provider_id}: {cred_path}")
            else:
                console.print(f"  {provider_id}: [dim]none[/dim]")


def auth_claude_command(
    session_key: str | None = None, verbose: bool = False, quiet: bool = False
) -> None:
    """Authenticate with Claude using a session key.

    Tries automatic browser cookie extraction first, then falls back
    to manual session key entry.
    """
    console = Console()

    # If no session key provided, try automatic extraction first
    if session_key is None:
        session_key = _try_browser_cookie_extraction(
            console,
            provider="claude",
            cookie_domains=[".claude.ai", "claude.ai"],
            cookie_names=["sessionKey"],
            verbose=verbose,
            quiet=quiet,
        )

    # If still no session key, prompt for manual entry
    if session_key is None:
        _show_claude_auth_instructions(console, quiet=quiet)
        session_key = typer.prompt("Session key", hide_input=True)

    # Validate session key format
    if not session_key.startswith("sk-ant-sid01-"):
        if not quiet:
            console.print(
                "[yellow]Warning:[/yellow] Session key doesn't match expected format (sk-ant-sid01-...)"
            )
        if not typer.confirm("Save anyway?"):
            raise typer.Exit(ExitCode.AUTH_ERROR)

    # Save session key
    cred_path = credential_path("claude", "session")
    cred_data = {"session_key": session_key}
    content = json.dumps(cred_data).encode()

    try:
        write_credential(cred_path, content)
        if not quiet:
            console.print("[green]Success:[/green] Claude session key saved")
            console.print(f"  Location: {cred_path}")
        if verbose:
            console.print(f"[dim]Session key prefix: {session_key[:20]}...[/dim]")
    except Exception as e:
        if not quiet:
            console.print(f"[red]Error saving credential:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR) from e


def auth_cursor_command(
    session_token: str | None = None, verbose: bool = False, quiet: bool = False
) -> None:
    """Authenticate with Cursor using a session token.

    Tries automatic browser cookie extraction first, then falls back
    to manual session token entry.
    """
    console = Console()

    # If no session token provided, try automatic extraction first
    if session_token is None:
        session_token = _try_browser_cookie_extraction(
            console,
            provider="cursor",
            cookie_domains=[".cursor.com", "cursor.com", ".cursor.sh", "cursor.sh"],
            cookie_names=[
                "WorkosCursorSessionToken",
                "__Secure-next-auth.session-token",
                "next-auth.session-token",
            ],
            verbose=verbose,
            quiet=quiet,
        )

    # If still no session token, prompt for manual entry
    if session_token is None:
        _show_cursor_auth_instructions(console, quiet=quiet)
        session_token = typer.prompt("Session token", hide_input=True)

    # Save session token
    cred_path = credential_path("cursor", "session")
    cred_data = {"session_token": session_token}
    content = json.dumps(cred_data).encode()

    try:
        write_credential(cred_path, content)
        if not quiet:
            console.print("[green]Success:[/green] Cursor session token saved")
            console.print(f"  Location: {cred_path}")
        if verbose:
            console.print(f"[dim]Session token prefix: {session_token[:20]}...[/dim]")
    except Exception as e:
        if not quiet:
            console.print(f"[red]Error saving credential:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR) from e


async def auth_copilot_command(verbose: bool = False, quiet: bool = False) -> None:
    """Authenticate with Copilot using GitHub device flow OAuth.

    This interactive flow:
    1. Requests a device code from GitHub
    2. Displays the verification URL and user code
    3. Opens a browser for the user to authorize
    4. Polls for the access token
    5. Saves credentials to vibeusage storage
    """
    console = Console()

    # Check if already authenticated
    has_creds, source = check_provider_credentials("copilot")

    if has_creds:
        source_label = {
            "vibeusage": "vibeusage storage",
            "provider_cli": "provider CLI",
            "env": "environment variable",
        }.get(source or "", source or "unknown")
        if not quiet:
            console.print(
                f"[green]✓[/green] Copilot is already authenticated ({source_label})"
            )

        if verbose:
            from vibeusage.config.credentials import find_provider_credential

            _, _, cred_path = find_provider_credential("copilot")
            if cred_path:
                console.print(f"  Location: {cred_path}")

        # Ask if user wants to re-authenticate
        if not quiet:
            if not typer.confirm("Re-authenticate?"):
                raise typer.Exit(ExitCode.SUCCESS)

    # Import and run device flow
    from vibeusage.providers.copilot.device_flow import CopilotDeviceFlowStrategy

    strategy = CopilotDeviceFlowStrategy()
    success = await strategy.device_flow(console=console, quiet=quiet)

    if not success:
        raise typer.Exit(ExitCode.AUTH_ERROR)


def auth_generic_command(
    provider: str, verbose: bool = False, quiet: bool = False
) -> None:
    """Generic auth handler for providers without specific auth flows."""
    console = Console()

    if provider not in list_provider_ids():
        if not quiet:
            console.print(f"[red]Unknown provider:[/red] {provider}")
            console.print(
                f"Available providers: {', '.join(sorted(list_provider_ids()))}"
            )
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    # Check if already authenticated
    has_creds, source = check_provider_credentials(provider)

    if has_creds:
        source_label = {
            "vibeusage": "vibeusage storage",
            "provider_cli": "provider CLI",
            "env": "environment variable",
        }.get(source or "", source or "unknown")
        if not quiet:
            console.print(
                f"[green]✓[/green] {provider} is already authenticated ({source_label})"
            )

        if verbose:
            _, _, cred_path = find_provider_credential(provider)
            if cred_path:
                console.print(f"  Location: {cred_path}")
        return

    # Show provider-specific instructions
    _show_provider_auth_instructions(console, provider, quiet=quiet)


def _show_claude_auth_instructions(console: Console, quiet: bool = False) -> None:
    """Display instructions for getting Claude session key."""
    if quiet:
        return
    instructions = Panel(
        """[bold cyan]Claude Authentication[/bold cyan]

Get your session key from claude.ai:

1. Open https://claude.ai in your browser
2. Open browser DevTools (F12 or Cmd+Option+I)
3. Go to Application/Storage → Cookies → https://claude.ai
4. Find the [bold]sessionKey[/bold] cookie
5. Copy its value (starts with [bold]sk-ant-sid01-[/bold])

[bold]Expected format:[/bold]
  sk-ant-sid01-<long alphanumeric string>
  The key is typically 100+ characters long.

[bold]Note:[/bold] Session keys expire periodically (usually after a few days
to a week). You'll need to re-run [bold]vibeusage auth claude[/bold] when
the key expires.

[bold]Alternative:[/bold] If you have the Claude CLI installed, vibeusage can
also read credentials from [bold]~/.claude/.credentials.json[/bold] automatically.

[dim]The session key allows vibeusage to fetch your usage data.[/dim]""",
        title="Instructions",
        border_style="cyan",
    )
    console.print(instructions)


def _show_provider_auth_instructions(
    console: Console, provider: str, quiet: bool = False
) -> None:
    """Display auth instructions for providers without specific flows."""
    if quiet:
        return
    instructions_map = {
        "codex": """[bold cyan]Codex (ChatGPT) Authentication[/bold cyan]

[bold]Option 1: Codex CLI (recommended)[/bold]
Run the official Codex CLI to authenticate:
  [dim]codex auth login[/dim]

vibeusage will automatically read credentials from
[bold]~/.codex/auth.json[/bold]

[bold]Option 2: Environment variable[/bold]
Set the [bold]OPENAI_API_KEY[/bold] environment variable:
  [dim]export OPENAI_API_KEY="sk-..."[/dim]

[bold]Option 3: Manual credential[/bold]
  [dim]vibeusage key codex set[/dim]

[bold]Credential file format[/bold] (~/.codex/auth.json):
  {
    "tokens": {
      "access_token": "<OAuth access token>",
      "refresh_token": "<OAuth refresh token>",
      "expires_at": "<ISO 8601 timestamp>"
    }
  }""",
        "copilot": """[bold cyan]GitHub Copilot Authentication[/bold cyan]

[bold]Option 1: Device flow (recommended)[/bold]
Run the interactive device flow to authenticate:
  [dim]vibeusage auth copilot[/dim]

This opens a browser where you authorize vibeusage with GitHub.
The OAuth token (prefixed [bold]gho_[/bold]) is saved automatically.

[bold]Option 2: VS Code / GitHub CLI credentials[/bold]
If you have GitHub Copilot configured in VS Code or the GitHub CLI,
vibeusage can read credentials from:
  [bold]~/.config/github-copilot/hosts.json[/bold]

[bold]Option 3: Environment variable[/bold]
Set the [bold]GITHUB_TOKEN[/bold] environment variable.

[bold]Option 4: Manual credential[/bold]
  [dim]vibeusage key copilot set[/dim]

[bold]Token format:[/bold]
  GitHub OAuth tokens start with [bold]gho_[/bold] (OAuth) or [bold]ghu_[/bold] (user).
  Example: gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx""",
        "cursor": """[bold cyan]Cursor Authentication[/bold cyan]

Cursor uses session cookies from the browser.

[dim]1. Open https://cursor.com in your browser
2. Extract session cookies manually
3. Set credential:[/dim]
  [dim]vibeusage key cursor set[/dim]""",
        "gemini": """[bold cyan]Gemini Authentication[/bold cyan]

[bold]Option 1: Gemini CLI (recommended)[/bold]
Run the official Gemini CLI to authenticate:
  [dim]gemini auth login[/dim]

vibeusage will automatically read credentials from
[bold]~/.gemini/oauth_creds.json[/bold]

[bold]Option 2: API key[/bold]
Set the [bold]GEMINI_API_KEY[/bold] environment variable:
  [dim]export GEMINI_API_KEY="AIza..."[/dim]

Or save an API key file:
  [dim]vibeusage key gemini set[/dim]

[bold]Option 3: Manual OAuth credential[/bold]
  [dim]vibeusage key gemini set[/dim]

[bold]Credential file format[/bold] (~/.gemini/oauth_creds.json):
  {
    "access_token": "<Google OAuth access token>",
    "refresh_token": "<Google OAuth refresh token>",
    "expires_at": "<ISO 8601 timestamp>"
  }

[dim]Tokens are automatically refreshed when they expire.[/dim]""",
    }

    instructions = instructions_map.get(
        provider,
        f"[bold cyan]{provider.title()} Authentication[/bold cyan]\n\n[dim]Set credentials manually:[/dim]\n  [dim]vibeusage key {provider} set[/dim]",
    )

    console.print(Panel(instructions, title="Instructions", border_style="cyan"))


def _show_cursor_auth_instructions(console: Console, quiet: bool = False) -> None:
    """Display instructions for getting Cursor session token."""
    if quiet:
        return
    instructions = Panel(
        """[bold cyan]Cursor Authentication[/bold cyan]

Get your session token from cursor.com:

1. Open https://cursor.com in your browser
2. Open browser DevTools (F12 or Cmd+Option+I)
3. Go to Application/Storage → Cookies → https://cursor.com
4. Look for one of these cookies:
   • [bold]WorkosCursorSessionToken[/bold]
   • [bold]__Secure-next-auth.session-token[/bold]
   • [bold]next-auth.session-token[/bold]
5. Copy its value

[bold]Expected format:[/bold]
  The token is a long encoded string, typically a JWT starting
  with [bold]eyJ[/bold] or a session identifier. It can be 100+ characters.

[dim]The session token allows vibeusage to fetch your usage data.[/dim]""",
        title="Instructions",
        border_style="cyan",
    )
    console.print(instructions)


def _try_browser_cookie_extraction(
    console: Console,
    *,
    provider: str,
    cookie_domains: list[str],
    cookie_names: list[str],
    verbose: bool = False,
    quiet: bool = False,
) -> str | None:
    """Try to extract a session cookie from installed browsers.

    Returns the cookie value if found, None otherwise.
    """
    try:
        import browser_cookie3
    except ImportError:
        try:
            import pycookiecheat as browser_cookie3  # type: ignore[no-redef]
        except ImportError:
            if verbose:
                console.print(
                    "[dim]Browser cookie extraction not available "
                    "(install browser_cookie3)[/dim]"
                )
            return None

    if not quiet:
        console.print(
            f"[dim]Attempting to extract {provider} session from browser cookies...[/dim]"
        )

    browsers = ["safari", "chrome", "firefox", "brave", "edge", "arc"]

    for browser_name in browsers:
        browser_fn = getattr(browser_cookie3, browser_name, None)
        if browser_fn is None:
            continue

        for domain in cookie_domains:
            try:
                cookies = browser_fn(domain_name=domain)
                for cookie in cookies:
                    if cookie.name in cookie_names:
                        if not quiet:
                            console.print(
                                f"[green]Found[/green] {provider} session cookie "
                                f"from {browser_name} ({cookie.name})"
                            )
                        return cookie.value
            except Exception:
                continue

    if verbose:
        console.print("[dim]No session cookie found in any browser[/dim]")
    return None
