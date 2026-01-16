"""Cache management commands for vibeusage."""

from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import app, ExitCode
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


@app.command("cache-show")
def cache_show_command() -> None:
    """Show cache status per provider."""
    console = Console()

    table = Table(title="Cache Status", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Snapshot", style="yellow")
    table.add_column("Org ID", style="dim")
    table.add_column("Age", style="dim")

    config = get_config()
    stale_threshold = config.fetch.stale_threshold_minutes

    for provider_id in list_provider_ids():
        snap_path = snapshot_path(provider_id)
        org_path = org_id_path(provider_id)

        snap_status = "—"
        snap_age = "—"
        org_status = "—"

        if snap_path.exists():
            snapshot = load_cached_snapshot(provider_id)
            if snapshot:
                if is_snapshot_fresh(snapshot, stale_threshold):
                    snap_status = "[green]✓ Fresh[/green]"
                else:
                    snap_status = "[yellow]⚠ Stale[/yellow]"

                from datetime import datetime, timezone

                now = datetime.now(timezone.utc)
                if snapshot.fetched_at.tzinfo is None:
                    fetched = snapshot.fetched_at.replace(tzinfo=timezone.utc)
                else:
                    fetched = snapshot.fetched_at

                delta = now - fetched
                if delta.days > 0:
                    snap_age = f"{delta.days}d"
                elif delta.seconds // 3600 > 0:
                    snap_age = f"{delta.seconds // 3600}h"
                elif delta.seconds // 60 > 0:
                    snap_age = f"{delta.seconds // 60}m"
                else:
                    snap_age = "<1m"
            else:
                snap_status = "[red]✗ Error[/red]"

        if org_path.exists():
            org_status = "[green]✓[/green]"

        table.add_row(provider_id, snap_status, org_status, snap_age)

    console.print(table)
    console.print(f"\nCache directory: {cache_dir()}")


@app.command("cache-clear")
def cache_clear_command(
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

    if provider is None:
        # Clear all cache
        if org_only:
            clear_all_cache(org_ids_only=True)
            console.print("[green]✓[/green] Cleared all org ID cache")
        else:
            clear_all_cache()
            console.print("[green]✓[/green] Cleared all cache")
    else:
        if provider not in list_provider_ids():
            console.print(f"[red]Unknown provider:[/red] {provider}")
            console.print(f"Available providers: {', '.join(list_provider_ids())}")
            raise typer.Exit(ExitCode.CONFIG_ERROR)

        if org_only:
            clear_org_id_cache(provider)
            console.print(f"[green]✓[/green] Cleared org ID cache for {provider}")
        else:
            clear_provider_cache(provider)
            console.print(f"[green]✓[/green] Cleared cache for {provider}")
