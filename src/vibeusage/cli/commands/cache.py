"""Cache management commands for vibeusage."""

from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import app, ExitCode
from vibeusage.cli.atyper import ATyper
from vibeusage.config.cache import (
    cache_org_id,
    cache_snapshot,
    clear_all_cache,
    clear_org_id_cache,
    clear_provider_cache,
    is_snapshot_fresh,
    load_cached_snapshot,
    org_id_path,
    snapshot_path,
)
from vibeusage.config.paths import cache_dir, snapshots_dir
from vibeusage.config.settings import get_config
from vibeusage.providers import list_provider_ids

# Create cache group
cache_app = ATyper(help="Manage cached usage data.")


@cache_app.callback(invoke_without_command=True)
def cache_callback(
    ctx: typer.Context,
) -> None:
    """Cache management commands.

    If no subcommand is provided, shows cache status.
    """
    # Store context for subcommands
    pass


@cache_app.command("show")
def cache_show_command(
    ctx: typer.Context,
) -> None:
    """Show cache status per provider."""
    console = Console()

    config = get_config()
    stale_threshold = config.fetch.stale_threshold_minutes
    json_mode = ctx.meta.get("json", False)
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    # Build cache status data
    cache_data = {}
    for provider_id in list_provider_ids():
        snap_path = snapshot_path(provider_id)
        org_path = org_id_path(provider_id)

        snap_status = "none"
        snap_age_minutes = None
        org_status = False

        if snap_path.exists():
            snapshot = load_cached_snapshot(provider_id)
            if snapshot:
                if is_snapshot_fresh(snapshot, stale_threshold):
                    snap_status = "fresh"
                else:
                    snap_status = "stale"

                from datetime import datetime, timezone

                now = datetime.now(timezone.utc)
                if snapshot.fetched_at.tzinfo is None:
                    fetched = snapshot.fetched_at.replace(tzinfo=timezone.utc)
                else:
                    fetched = snapshot.fetched_at

                delta = now - fetched
                snap_age_minutes = int(delta.total_seconds() / 60)
            else:
                snap_status = "error"

        if org_path.exists():
            org_status = True

        cache_data[provider_id] = {
            "snapshot": snap_status,
            "org_id_cached": org_status,
            "age_minutes": snap_age_minutes,
        }

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        output_json_pretty(cache_data)
        return

    # Quiet mode: minimal output
    if quiet:
        for provider_id, data in cache_data.items():
            console.print(f"{provider_id}: {data['snapshot']}")
        return

    # Display table
    table = Table(title="Cache Status", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Snapshot", style="yellow")
    table.add_column("Org ID", style="dim")
    table.add_column("Age", style="dim")

    for provider_id, data in cache_data.items():
        snap_status_map = {
            "fresh": "[green]✓ Fresh[/green]",
            "stale": "[yellow]⚠ Stale[/yellow]",
            "error": "[red]✗ Error[/red]",
            "none": "—",
        }
        snap_status = snap_status_map.get(data["snapshot"], "—")

        org_status = "[green]✓[/green]" if data["org_id_cached"] else "—"

        age = data["age_minutes"]
        if age is None:
            snap_age = "—"
        elif age >= 1440:
            snap_age = f"{age // 1440}d"
        elif age >= 60:
            snap_age = f"{age // 60}h"
        elif age >= 1:
            snap_age = f"{age}m"
        else:
            snap_age = "<1m"

        table.add_row(provider_id, snap_status, org_status, snap_age)

    console.print(table)
    console.print(f"\nCache directory: {cache_dir()}")

    # Verbose: show stale threshold
    if verbose:
        console.print(f"\n[dim]Stale threshold: {stale_threshold} minutes[/dim]")
        console.print(f"[dim]Snapshots older than this are considered stale.[/dim]")


@cache_app.command("clear")
def cache_clear_command(
    ctx: typer.Context,
    provider: str = typer.Argument(
        None,
        help="Provider to clear cache for (default: all)",
    ),
    org_only: bool = typer.Option(
        False,
        "--org",
        "-o",
        help="Only clear org ID cache",
    ),
) -> None:
    """Clear cache data."""
    console = Console()
    json_mode = ctx.meta.get("json", False)

    result = {"success": True, "cleared": []}

    if provider is None:
        # Clear all cache
        if org_only:
            clear_all_cache(org_ids_only=True)
            result["message"] = "Cleared all org ID cache"
        else:
            clear_all_cache()
            result["message"] = "Cleared all cache"
        result["provider"] = "all"
        result["org_only"] = org_only
    else:
        if provider not in list_provider_ids():
            result["success"] = False
            result["error"] = f"Unknown provider: {provider}"
            if json_mode:
                from vibeusage.display.json import output_json_pretty
                output_json_pretty(result)
            console.print(f"[red]Unknown provider:[/red] {provider}")
            console.print(f"Available providers: {', '.join(list_provider_ids())}")
            raise typer.Exit(ExitCode.CONFIG_ERROR)

        result["provider"] = provider
        result["org_only"] = org_only
        result["cleared"].append(provider)

        if org_only:
            clear_org_id_cache(provider)
            result["message"] = f"Cleared org ID cache for {provider}"
        else:
            clear_provider_cache(provider)
            result["message"] = f"Cleared cache for {provider}"

    if json_mode:
        from vibeusage.display.json import output_json_pretty
        output_json_pretty(result)
        return

    if result["success"]:
        console.print(f"[green]✓[/green] {result['message']}")


# Register the cache group with the main app
app.add_typer(cache_app, name="cache")
