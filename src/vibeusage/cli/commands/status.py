"""Provider status command for vibeusage."""

import asyncio

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import app, ExitCode
from vibeusage.core.http import cleanup
from vibeusage.providers import get_all_providers, list_provider_ids


@app.command("status")
async def status_command(
    json: bool = typer.Option(
        False,
        "--json",
        "-j",
        help="Output in JSON format",
    ),
) -> None:
    """Show health status for all providers."""
    console = Console()

    try:
        # Fetch all provider statuses
        statuses = await fetch_all_statuses()

        if json:
            output_json_status(console, statuses)
        else:
            display_status_table(console, statuses)

    except KeyboardInterrupt:
        console.print("\n[yellow]Interrupted[/yellow]")
        raise typer.Exit(ExitCode.GENERAL_ERROR)
    except Exception as e:
        console.print(f"[red]Unexpected error:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR)
    finally:
        await cleanup()


async def fetch_all_statuses():
    """Fetch status from all providers."""
    from vibeusage.models import ProviderStatus, StatusLevel

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


def display_status_table(console, statuses):
    """Display provider statuses in a table."""
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


def output_json_status(console, statuses):
    """Output statuses in JSON format."""
    import msgspec.json

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

    from datetime import datetime, timezone

    now = datetime.now(timezone.utc)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)

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
