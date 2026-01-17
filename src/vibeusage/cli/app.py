"""Main CLI application for vibeusage."""
from __future__ import annotations

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

    from rich.console import Console

    from vibeusage.cli.commands.usage import display_multiple_snapshots
    from vibeusage.cli.commands.usage import fetch_all_usage
    from vibeusage.core.http import cleanup

    console = Console()
    refresh = False  # Default to not refresh

    # Fetch with timing
    start_time = time.monotonic()
    outcomes = await fetch_all_usage(refresh)
    duration_ms = (time.monotonic() - start_time) * 1000

    # Get verbose/quiet from context
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

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


def run_app() -> None:
    """Run the CLI app."""
    app()


# Import commands to register them with the app
# These imports must come after app is defined
# Note: key, config, and cache groups register themselves via add_typer()

# Import key, config, cache modules to trigger their self-registration


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
