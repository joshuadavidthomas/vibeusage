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
def auth_command(
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

    The session key can be found in browser cookies at claude.ai.
    Look for the 'sessionKey' cookie.
    """
    console = Console()

    # Show instructions if no session key provided
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
5. Copy its value (starts with sk-ant-sid01-)

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

Codex authentication uses OAuth credentials.

[dim]Run the official Codex CLI to authenticate:[/dim]
  [dim]codex auth login[/dim]

[dim]Or set credentials manually with your OAuth token:[/dim]
  [dim]vibeusage key codex set[/dim]""",
        "copilot": """[bold cyan]GitHub Copilot Authentication[/bold cyan]

GitHub Copilot uses GitHub device flow OAuth.

[dim]Run the official GitHub CLI to authenticate:[/dim]
  [dim]gh auth login[/dim]

[dim]Or set credentials manually with your OAuth token:[/dim]
  [dim]vibeusage key copilot set[/dim]

[dim]Note: [italic]gh auth login[/italic] authenticates the GitHub CLI, not Copilot.[/dim]""",
        "cursor": """[bold cyan]Cursor Authentication[/bold cyan]

Cursor uses session cookies from the browser.

[dim]1. Open https://cursor.com in your browser
2. Extract session cookies manually
3. Set credential:[/dim]
  [dim]vibeusage key cursor set[/dim]""",
        "gemini": """[bold cyan]Gemini Authentication[/bold cyan]

Gemini uses Google OAuth credentials.

[dim]Run the official Gemini CLI to authenticate:[/dim]
  [dim]gemini auth login[/dim]

[dim]Or set credentials manually with your OAuth token:[/dim]
  [dim]vibeusage key gemini set[/dim]""",
    }

    instructions = instructions_map.get(
        provider,
        f"[bold cyan]{provider.title()} Authentication[/bold cyan]\n\n[dim]Set credentials manually:[/dim]\n  [dim]vibeusage key {provider} set[/dim]",
    )

    console.print(Panel(instructions, title="Instructions", border_style="cyan"))
