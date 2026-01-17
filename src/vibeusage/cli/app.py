"""Main CLI application for vibeusage."""

from __future__ import annotations

import asyncio
from enum import IntEnum

import typer
from rich.console import Console

from vibeusage.cli.atyper import ATyper

# Create the main app
app = ATyper(
    name="vibeusage",
    help="Track usage across agentic LLM providers",
    add_completion=True,
    no_args_is_help=False,
    invoke_without_command=True,
)


class ExitCode(IntEnum):
    """Exit codes for vibeusage."""

    SUCCESS = 0
    GENERAL_ERROR = 1
    AUTH_ERROR = 2
    NETWORK_ERROR = 3
    CONFIG_ERROR = 4
    PARTIAL_FAILURE = 5


@app.callback()
def main(
    ctx: typer.Context,
    json: bool = typer.Option(False, "--json", "-j", help="Enable JSON output mode"),
    no_color: bool = typer.Option(False, "--no-color", help="Disable colored output"),
    verbose: bool = typer.Option(
        False, "--verbose", "-v", help="Enable verbose output"
    ),
    quiet: bool = typer.Option(False, "--quiet", "-q", help="Enable quiet mode"),
    version: bool = typer.Option(False, "--version", help="Show version and exit"),
) -> None:
    """Vibeusage - Track usage across agentic LLM providers."""
    if version:
        from vibeusage import __version__

        typer.echo(f"vibeusage {__version__}")
        raise typer.Exit()

    # Resolve conflicts: verbose and quiet are mutually exclusive, quiet takes precedence
    if verbose and quiet:
        quiet = True
        verbose = False

    # Store options in context
    ctx.meta["json"] = json
    ctx.meta["no_color"] = no_color
    ctx.meta["verbose"] = verbose
    ctx.meta["quiet"] = quiet

    # If no command provided, run default usage command
    if ctx.invoked_subcommand is None:
        asyncio.run(run_default_usage(ctx))


async def run_default_usage(ctx: typer.Context) -> None:
    """Run the default usage command."""
    import time

    from vibeusage.cli.commands.usage import display_multiple_snapshots
    from vibeusage.cli.commands.usage import fetch_all_usage
    from vibeusage.config.credentials import is_first_run
    from vibeusage.core.http import cleanup

    console = Console()
    refresh = False  # Default to not refresh

    # Get verbose/quiet from context
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)
    json_mode = ctx.meta.get("json", False)

    # Check for first run and show helpful message
    if is_first_run() and not json_mode and not quiet:
        _show_first_run_message(console)
        raise typer.Exit(ExitCode.SUCCESS)

    # Fetch with timing
    start_time = time.monotonic()
    outcomes = await fetch_all_usage(refresh)
    duration_ms = (time.monotonic() - start_time) * 1000

    display_multiple_snapshots(
        console,
        outcomes,
        ctx,
        verbose=verbose,
        quiet=quiet,
        total_duration_ms=duration_ms,
    )

    # Cleanup HTTP client
    await cleanup()


def _show_first_run_message(console: Console) -> None:
    """Show first-run welcome message."""
    from rich.panel import Panel

    from vibeusage.providers import list_provider_ids

    console.print()
    welcome = Panel(
        """[bold cyan]Welcome to vibeusage![/bold cyan]

[dim]No providers are configured yet.[/dim]

[dim]Track your usage across AI providers in one place.[/dim]""",
        title="✨ First-Time Setup",
        border_style="cyan",
        padding=(1, 2),
    )
    console.print(welcome)
    console.print()

    console.print("[bold]Quick start:[/bold]")
    console.print("  [cyan]vibeusage init[/cyan] - Run the setup wizard")
    console.print("  [cyan]vibeusage init --quick[/cyan] - Quick setup with Claude")
    console.print()
    console.print("[bold]Or set up a provider directly:[/bold]")

    for provider_id in sorted(list_provider_ids())[:3]:  # Show first 3
        console.print(f"  [dim]vibeusage auth {provider_id}[/dim]")

    console.print()
    console.print("[dim]Run 'vibeusage init' to see all available providers.[/dim]")
    console.print()


def run_app() -> None:
    """Run the CLI app."""
    app()


# Import commands to register them with the app
# These imports must come after app is defined
# Note: key, config, and cache groups register themselves via add_typer()

# Import command modules - they register themselves via @app.command() decorators
from vibeusage.cli.commands import auth  # noqa: F401 (registers auth command)
from vibeusage.cli.commands import init  # noqa: F401 (registers init command)
from vibeusage.cli.commands import status  # noqa: F401 (registers status command)
from vibeusage.cli.commands import usage  # noqa: F401 (registers usage command)

# Import key, config, cache modules and register their typer groups
from vibeusage.cli.commands import cache as cache_cmd  # noqa: F401
from vibeusage.cli.commands import config as config_cmd  # noqa: F401
from vibeusage.cli.commands import key as key_cmd  # noqa: F401

# Register typer groups for key, cache, config commands
app.add_typer(key_cmd.key_app, name="key")
app.add_typer(cache_cmd.cache_app, name="cache")
app.add_typer(config_cmd.config_app, name="config")


# Provider command aliases - these provide top-level shortcuts like `vibeusage claude`
# Each alias behaves identically to `vibeusage usage <provider>`
def _create_provider_command(provider_id: str):
    """Create a provider-specific command that delegates to usage command."""

    @app.command(provider_id)
    async def provider_command(
        ctx: typer.Context,
        refresh: bool = typer.Option(
            False,
            "--refresh",
            "-r",
            help="Bypass cache and fetch fresh data",
        ),
        json_output: bool = typer.Option(
            False,
            "--json",
            "-j",
            help="Output in JSON format",
        ),
    ) -> None:
        """Show usage statistics for this provider."""
        # Import here to avoid circular imports
        from vibeusage.cli.commands.usage import usage_command

        # Create a new context with provider set
        # This simulates calling `vibeusage usage <provider>`
        return await usage_command(
            ctx,
            provider=provider_id,
            refresh=refresh,
            json_output=json_output,
        )

    return provider_command


# Register provider commands for all known providers
# These must come after app is fully initialized
_create_provider_command("claude")
_create_provider_command("codex")
_create_provider_command("copilot")
_create_provider_command("cursor")
_create_provider_command("gemini")
