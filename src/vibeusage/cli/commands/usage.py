"""Usage display commands for vibeusage."""

import asyncio

import typer
from rich.console import Console

from vibeusage.cli.app import app, ExitCode
from vibeusage.config.settings import get_config
from vibeusage.core.http import cleanup
from vibeusage.providers import create_provider, list_provider_ids


@app.command()
async def usage_command(
    provider: str = typer.Argument(
        None,
        help="Provider to show (default: all enabled)",
    ),
    refresh: bool = typer.Option(
        False,
        "--refresh",
        "-r",
        help="Bypass cache and fetch fresh data",
    ),
) -> None:
    """Show usage statistics for all enabled providers or a specific provider."""
    import sys

    from rich.console import Console

    # Get console, respecting no-color option
    console = Console()
    ctx = typer.get_context()

    try:
        if provider:
            # Single provider
            result = await fetch_provider_usage(provider, refresh)
            if result.success:
                display_snapshot(console, result.snapshot, result.source, result.cached)
            else:
                console.print(f"[red]Error:[/red] {result.error}")
                raise typer.Exit(ExitCode.GENERAL_ERROR)
        else:
            # All enabled providers
            results = await fetch_all_usage(refresh)
            display_multiple_snapshots(console, results)

    except KeyboardInterrupt:
        console.print("\n[yellow]Interrupted[/yellow]")
        raise typer.Exit(ExitCode.GENERAL_ERROR)
    except Exception as e:
        console.print(f"[red]Unexpected error:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR)
    finally:
        await cleanup()


async def fetch_provider_usage(provider_id: str, refresh: bool):
    """Fetch usage for a single provider."""
    from vibeusage.core.fetch import execute_fetch_pipeline
    from vibeusage.strategies.base import FetchOutcome

    # Check if provider exists
    available = list_provider_ids()
    if provider_id not in available:
        return FetchOutcome(
            provider_id=provider_id,
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            error=Exception(f"Unknown provider: {provider_id}. Available: {', '.join(available)}"),
        )

    # Create provider and get strategies
    provider = create_provider(provider_id)
    strategies = provider.fetch_strategies()

    # Check if cache should be used
    use_cache = not refresh

    # Fetch
    outcome = await execute_fetch_pipeline(provider_id, strategies, use_cache=use_cache)
    return outcome


async def fetch_all_usage(refresh: bool):
    """Fetch usage for all enabled providers."""
    from vibeusage.core.orchestrator import fetch_enabled_providers
    from vibeusage.providers import get_all_providers

    # Build provider map
    provider_map = {}
    for provider_id, provider_cls in get_all_providers().items():
        provider = provider_cls()
        provider_map[provider_id] = provider.fetch_strategies()

    # Fetch all
    outcomes = await fetch_enabled_providers(provider_map)
    return outcomes


def display_snapshot(console, snapshot, source, cached):
    """Display a single usage snapshot."""
    from rich.panel import Panel
    from rich.text import Text

    # Build output
    lines = []

    # Header
    header = Text()
    header.append(f"{snapshot.provider.upper()} ", style="bold")
    if cached:
        header.append("(cached) ", style="dim yellow")
    header.append(f"via {source}", style="dim")
    lines.append(header)

    # Periods
    for period in snapshot.periods:
        period_text = format_period(period)
        lines.append(period_text)

    # Overage
    if snapshot.overage and snapshot.overage.is_enabled:
        overage_text = format_overage(snapshot.overage)
        lines.append(overage_text)

    # Display
    content = "\n".join(str(line) for line in lines)
    console.print(Panel.fit(content, title=f"{snapshot.provider} Usage"))


def display_multiple_snapshots(console, outcomes):
    """Display multiple provider outcomes."""
    from rich.panel import Panel
    from rich.table import Table

    # Check if any data
    has_data = any(o.success and o.snapshot for o in outcomes.values())

    if not has_data:
        console.print("[yellow]No usage data available[/yellow]")
        console.print("\nConfigure credentials with:")
        console.print("  vibeusage key <provider> set")
        return

    # Create summary table
    table = Table(title="Usage Summary", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Usage", style="green")
    table.add_column("Status", style="yellow")

    for provider_id, outcome in outcomes.items():
        if outcome.success and outcome.snapshot:
            snapshot = outcome.snapshot
            primary = snapshot.primary_period() if snapshot.periods else None

            if primary:
                usage_text = f"{primary.utilization}%"
                if outcome.cached:
                    usage_text += " (cached)"
            else:
                usage_text = "N/A"

            status = "✓" if not outcome.cached else "⚠"
        else:
            usage_text = "Error"
            status = "✗"

        table.add_row(provider_id, usage_text, status)

    console.print(table)

    # Show errors if verbose
    ctx = typer.get_context()
    if ctx.meta.get("verbose"):
        errors = [(pid, o.error) for pid, o in outcomes.items() if o.error]
        if errors:
            console.print("\n[red]Errors:[/red]")
            for pid, error in errors:
                console.print(f"  {pid}: {error}")


def format_period(period):
    """Format a usage period for display."""
    from rich.text import Text

    text = Text()

    # Bar based on utilization
    bar_width = 20
    filled = int((period.utilization / 100) * bar_width)
    bar = "█" * filled + "░" * (bar_width - filled)

    # Color based on pace
    color = get_pace_color(period)

    text.append(f"{bar} ", style=color)
    text.append(f"{period.utilization}% ", style="bold")
    text.append(period.name, style="dim")

    # Reset time
    if period.resets_at:
        from vibeusage.models import format_reset_countdown

        time_str = format_reset_countdown(period.resets_at)
        text.append(f" • resets in {time_str}", style="dim")

    return text


def format_overage(overage):
    """Format overage usage for display."""
    from rich.text import Text

    text = Text()
    remaining = overage.limit - overage.used

    text.append(f"Overage: ${overage.used:.2f}", style="yellow")
    text.append(f" / ${overage.limit:.2f}", style="dim")
    text.append(f" (${remaining:.2f} remaining)", style="bold yellow")

    return text


def get_pace_color(period):
    """Get color for a period based on utilization pace."""
    from vibeusage.models import pace_to_color

    color_name = pace_to_color(period)
    return color_name
