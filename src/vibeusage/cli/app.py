"""Main CLI application for vibeusage."""

import asyncio
from enum import IntEnum

import typer

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
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
    quiet: bool = typer.Option(False, "--quiet", "-q", help="Enable quiet mode"),
    version: bool = typer.Option(False, "--version", help="Show version and exit"),
) -> None:
    """Vibeusage - Track usage across agentic LLM providers."""
    if version:
        from vibeusage import __version__

        typer.echo(f"vibeusage {__version__}")
        raise typer.Exit()

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
    from rich.console import Console
    from vibeusage.cli.commands.usage import fetch_all_usage, display_multiple_snapshots
    from vibeusage.core.http import cleanup

    console = Console()
    refresh = False  # Default to not refresh
    outcomes = await fetch_all_usage(refresh)
    display_multiple_snapshots(console, outcomes)

    # Cleanup HTTP client
    await cleanup()


def run_app() -> None:
    """Run the CLI app."""
    app()
