"""First-run setup wizard for vibeusage."""

from __future__ import annotations

import typer
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.config.credentials import check_provider_credentials
from vibeusage.config.credentials import count_configured_providers
from vibeusage.config.credentials import is_first_run
from vibeusage.providers import list_provider_ids

# Provider descriptions for the setup wizard
_PROVIDER_DESCRIPTIONS: dict[str, str] = {
    "claude": "Anthropic's Claude AI assistant (claude.ai)",
    "codex": "OpenAI's Codex/ChatGPT (platform.openai.com)",
    "copilot": "GitHub Copilot (github.com)",
    "cursor": "Cursor AI code editor (cursor.com)",
    "gemini": "Google's Gemini AI (gemini.google.com)",
}

# Quick setup commands for each provider
_QUICK_SETUP_COMMANDS: dict[str, str] = {
    "claude": "vibeusage auth claude",
    "codex": "vibeusage auth codex",
    "copilot": "vibeusage auth copilot",
    "cursor": "vibeusage auth cursor",
    "gemini": "vibeusage auth gemini",
}


@app.command("init")
def init_command(
    ctx: typer.Context,
    quick: bool = typer.Option(
        False,
        "--quick",
        "-q",
        help="Quick setup with default provider (Claude)",
    ),
    skip: bool = typer.Option(
        False,
        "--skip",
        help="Skip setup and exit",
    ),
    json_output: bool = typer.Option(
        False,
        "--json",
        "-j",
        help="Output in JSON format",
    ),
) -> None:
    """Run first-time setup wizard.

    This command helps you configure vibeusage for the first time.
    You can select which providers to set up and get guided through authentication.
    """
    console = Console()

    # Get verbose/quiet from context
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    # Check for JSON mode
    json_mode = json_output or ctx.meta.get("json", False)

    # JSON output for first-run status
    if json_mode:
        from vibeusage.display.json import output_json_pretty

        output_json_pretty(
            {
                "first_run": is_first_run(),
                "configured_providers": count_configured_providers(),
                "available_providers": sorted(list_provider_ids()),
            }
        )
        return

    # Skip flag - just show status
    if skip:
        if is_first_run():
            if not quiet:
                console.print("[yellow]First run detected.[/yellow]")
                console.print("[dim]Run 'vibeusage init' to set up a provider.[/dim]")
            raise typer.Exit(ExitCode.SUCCESS)
        else:
            if not quiet:
                console.print(
                    f"[green]Already configured[/green] ({count_configured_providers()} providers)"
                )
            raise typer.Exit(ExitCode.SUCCESS)

    # Check if already configured
    if not is_first_run() and not quick:
        _show_already_configured(console, verbose=verbose, quiet=quiet)
        try:
            response = typer.prompt(
                "Run setup again?",
                default=False,
                show_default=False,
            )
        except (KeyboardInterrupt, EOFError):
            console.print()
            raise typer.Exit(ExitCode.SUCCESS) from None

        if not response:
            raise typer.Exit(ExitCode.SUCCESS)

    # Show welcome
    if not quiet:
        _show_welcome(console)

    # Quick setup mode
    if quick:
        _quick_setup(console, verbose=verbose, quiet=quiet)
        raise typer.Exit(ExitCode.SUCCESS)

    # Interactive wizard
    _run_interactive_wizard(console, verbose=verbose, quiet=quiet)


def _show_welcome(console: Console) -> None:
    """Show welcome message."""
    welcome = Panel(
        """[bold cyan]Welcome to vibeusage![/bold cyan]

Track your usage across AI providers in one place.

[dim]This wizard will help you:[/dim]
[dim]• Choose which providers to set up[/dim]
[dim]• Configure authentication[/dim]
[dim]• Start tracking your AI usage[/dim]

[dim]You can always add more providers later with 'vibeusage auth <provider>'[/dim]""",
        title="✨ First-Time Setup",
        border_style="cyan",
        padding=(1, 2),
    )
    console.print(welcome)
    console.print()


def _show_already_configured(
    console: Console, verbose: bool = False, quiet: bool = False
) -> None:
    """Show message for already configured users."""
    count = count_configured_providers()

    if quiet:
        console.print(f"Already configured ({count} providers)")
        return

    table = Table(title="Current Configuration", show_header=True)
    table.add_column("Provider", style="cyan")
    table.add_column("Status", style="bold")

    for provider_id in sorted(list_provider_ids()):
        has_creds, source = check_provider_credentials(provider_id)
        if has_creds:
            status = "[green]Configured[/green]"
        else:
            status = "[dim]Not configured[/dim]"
        table.add_row(provider_id, status)

    console.print(table)
    console.print()


def _quick_setup(
    console: Console, verbose: bool = False, quiet: bool = False
) -> None:
    """Quick setup with Claude as default provider."""
    if quiet:
        console.print("Run 'vibeusage auth claude' to set up Claude")
        raise typer.Exit(ExitCode.SUCCESS)

    console.print("[bold]Quick Setup: Claude[/bold]")
    console.print("[dim]Claude is the most popular AI assistant for agentic workflows.[/dim]")
    console.print()

    # Check if already configured
    has_creds, _ = check_provider_credentials("claude")
    if has_creds:
        console.print("[green]✓ Claude is already configured![/green]")
        console.print()
        console.print("[dim]You can run 'vibeusage' to see your usage.[/dim]")
        raise typer.Exit(ExitCode.SUCCESS)

    console.print("[cyan]To set up Claude, run:[/cyan]")
    console.print("  [bold]vibeusage auth claude[/bold]")
    console.print()
    console.print("[dim]This will prompt you for your session key from claude.ai[/dim]")
    console.print()
    console.print("[dim]After setup, run 'vibeusage' to see your usage.[/dim]")


def _run_interactive_wizard(
    console: Console, verbose: bool = False, quiet: bool = False
) -> None:
    """Run the interactive setup wizard."""
    if quiet:
        console.print("Use 'vibeusage auth <provider>' to set up providers")
        raise typer.Exit(ExitCode.SUCCESS)

    all_providers = sorted(list_provider_ids())

    # Step 1: Show available providers
    console.print("[bold]Step 1: Choose providers to set up[/bold]")
    console.print()

    table = Table(show_header=True, header_style="bold cyan")
    table.add_column("#", style="dim", width=3)
    table.add_column("Provider", style="cyan")
    table.add_column("Description", style="white")
    table.add_column("Status", style="bold")

    for idx, provider_id in enumerate(all_providers, 1):
        has_creds, _ = check_provider_credentials(provider_id)
        status = "[green]✓ Configured[/green]" if has_creds else "[dim]Not configured[/dim]"
        description = _PROVIDER_DESCRIPTIONS.get(provider_id, f"{provider_id.title()} AI")
        table.add_row(str(idx), provider_id, description, status)

    console.print(table)
    console.print()

    # Step 2: Get provider selection
    console.print("[dim]Enter provider numbers separated by spaces (e.g., '1 3 5')[/dim]")
    console.print("[dim]Press Enter to skip setup and use vibeusage later[/dim]")

    try:
        selection_input = typer.prompt("Providers", default="")
    except (KeyboardInterrupt, EOFError):
        console.print("\n[dim]Setup cancelled. Run 'vibeusage init' anytime to set up.[/dim]")
        raise typer.Exit(ExitCode.SUCCESS) from None

    if not selection_input.strip():
        console.print()
        console.print("[dim]No providers selected. You can set up providers later:[/dim]")
        for provider_id in all_providers:
            console.print(f"  [dim]vibeusage auth {provider_id}[/dim]")
        raise typer.Exit(ExitCode.SUCCESS)

    # Parse selection
    try:
        selected_indices = [int(x.strip()) for x in selection_input.split()]
        selected_providers = [
            all_providers[i - 1] for i in selected_indices if 1 <= i <= len(all_providers)
        ]
    except ValueError:
        console.print("[red]Invalid selection. Please enter numbers separated by spaces.[/red]")
        raise typer.Exit(ExitCode.CONFIG_ERROR) from None

    if not selected_providers:
        console.print("[red]No valid providers selected.[/red]")
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    # Step 3: Show setup commands for selected providers
    console.print()
    console.print(f"[bold]Step 2: Set up {len(selected_providers)} provider(s)[/bold]")
    console.print()

    unconfigured_count = 0
    first_unconfigured = None

    for provider_id in selected_providers:
        has_creds, _ = check_provider_credentials(provider_id)
        if has_creds:
            console.print(f"[green]✓ {provider_id}[/green] already configured")
        else:
            setup_cmd = _QUICK_SETUP_COMMANDS.get(
                provider_id, f"vibeusage auth {provider_id}"
            )
            console.print(f"[cyan]→ {provider_id}:[/cyan] {setup_cmd}")
            unconfigured_count += 1
            if first_unconfigured is None:
                first_unconfigured = provider_id

    console.print()

    # Step 4: Ask if user wants to set up now
    if unconfigured_count > 0:
        console.print("[bold]Run the commands above to authenticate each provider.[/bold]")
        console.print("[dim]After setup, run 'vibeusage' to see your usage.[/dim]")
        console.print()

        try:
            setup_now = typer.prompt(
                "Set up first provider now?",
                default=False,
                show_default=False,
            )
        except (KeyboardInterrupt, EOFError):
            setup_now = False

        if setup_now and first_unconfigured:
            console.print()
            console.print(f"[bold]Running: vibeusage auth {first_unconfigured}[/bold]")
            console.print()
            _run_auth_for_provider(first_unconfigured, verbose=verbose, quiet=quiet)
    else:
        console.print("[bold]All selected providers are already configured![/bold]")
        console.print()
        console.print("[dim]Run 'vibeusage' to see your usage.[/dim]")


def _run_auth_for_provider(
    provider_id: str, verbose: bool = False, quiet: bool = False
) -> None:
    """Run auth command for a specific provider."""
    if provider_id == "claude":
        from vibeusage.cli.commands.auth import auth_claude_command

        auth_claude_command(verbose=verbose, quiet=quiet)
    else:
        from vibeusage.cli.commands.auth import auth_generic_command

        auth_generic_command(provider_id, verbose=verbose, quiet=quiet)
