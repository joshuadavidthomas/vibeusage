"""Provider status command for vibeusage."""

from __future__ import annotations

import time

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.core.http import cleanup
from vibeusage.providers import get_all_providers


@app.command("status")
async def status_command(
    ctx: typer.Context,
    json_output: bool = typer.Option(
        False,
        "--json",
        "-j",
        help="Output in JSON format",
    ),
) -> None:
    """Show health status for all providers."""
    console = Console()

    # Get verbose/quiet from context
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    try:
        # Fetch all provider statuses with timing
        start_time = time.monotonic()
        statuses = await fetch_all_statuses()
        duration_ms = (time.monotonic() - start_time) * 1000

        # Check for JSON mode (from global flag or local option)
        json_mode = json_output or ctx.meta.get("json", False)
        if json_mode:
            output_json_status(statuses)
        else:
            display_status_table(
                console, statuses, verbose=verbose, quiet=quiet, duration_ms=duration_ms
            )

    except KeyboardInterrupt:
        if not quiet:
            console.print("\n[yellow]Interrupted[/yellow]")
        raise typer.Exit(ExitCode.GENERAL_ERROR) from None
    except Exception as e:
        if not quiet:
            console.print(f"[red]Unexpected error:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR) from e
    finally:
        await cleanup()


async def fetch_all_statuses():
    """Fetch status from all providers."""
    from vibeusage.models import ProviderStatus
    from vibeusage.models import StatusLevel

    statuses = {}

    for provider_id, provider_cls in get_all_providers().items():
        provider = provider_cls()
        try:
            status = await provider.fetch_status()
            statuses[provider_id] = status
        except Exception:
            # Default to unknown on error
            statuses[provider_id] = ProviderStatus(
                level=StatusLevel.UNKNOWN,
                description="Unable to fetch status",
                updated_at=None,
            )

    return statuses


def display_status_table(
    console,
    statuses,
    verbose: bool = False,
    quiet: bool = False,
    duration_ms: float = 0,
):
    """Display provider statuses in a table."""

    # Quiet mode: minimal output
    if quiet:
        for provider_id, status in statuses.items():
            symbol = status_symbol(status.level)
            console.print(f"{provider_id}: {symbol} {status.level.value}")
        return

    table = Table(title="Provider Status", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Status", style="bold")
    table.add_column("Description", style="dim")
    table.add_column("Updated", style="dim")

    for provider_id, status in statuses.items():
        # Get symbol for status level
        symbol = status_symbol(status.level)
        color = status_color(status.level)

        table.add_row(
            provider_id,
            f"[{color}]{symbol}[/{color}]",
            status.description or "",
            format_status_updated(status.updated_at),
        )

    console.print(table)

    # Verbose: show timing
    if verbose and duration_ms > 0:
        console.print(f"\n[dim]Fetched in {duration_ms:.0f}ms[/dim]")


def output_json_status(statuses):
    """Output statuses in JSON format."""
    from vibeusage.display.json import output_json_pretty

    data = {
        pid: {
            "level": status.level.value,
            "description": status.description,
            "updated_at": status.updated_at.isoformat() if status.updated_at else None,
        }
        for pid, status in statuses.items()
    }

    output_json_pretty(data)


def status_symbol(level):
    """Get Unicode symbol for status level."""
    from vibeusage.models import StatusLevel

    return {
        StatusLevel.OPERATIONAL: "●",
        StatusLevel.DEGRADED: "◐",
        StatusLevel.PARTIAL_OUTAGE: "◑",
        StatusLevel.MAJOR_OUTAGE: "○",
        StatusLevel.UNKNOWN: "?",
    }.get(level, "?")


def status_color(level):
    """Get color for status level."""
    from vibeusage.models import StatusLevel

    return {
        StatusLevel.OPERATIONAL: "green",
        StatusLevel.DEGRADED: "yellow",
        StatusLevel.PARTIAL_OUTAGE: "orange",
        StatusLevel.MAJOR_OUTAGE: "red",
        StatusLevel.UNKNOWN: "dim",
    }.get(level, "dim")


def format_status_updated(dt):
    """Format status updated time."""
    if dt is None:
        return "unknown"

    from datetime import UTC
    from datetime import datetime

    now = datetime.now(UTC)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)

    delta = now - dt

    if delta.days > 0:
        return f"{delta.days}d ago"
    hours = delta.seconds // 3600
    if hours > 0:
        return f"{hours}h ago"
    minutes = delta.seconds // 60
    if minutes > 0:
        return f"{minutes}m ago"
    return "just now"
