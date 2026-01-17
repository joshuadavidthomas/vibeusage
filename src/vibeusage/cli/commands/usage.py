"""Usage display commands for vibeusage."""

from __future__ import annotations

import time

import typer
from rich.console import Console
from rich.text import Text

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.core.http import cleanup
from vibeusage.providers import create_provider
from vibeusage.providers import list_provider_ids


@app.command("usage")
async def usage_command(
    ctx: typer.Context,
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
    json_output: bool = typer.Option(
        False,
        "--json",
        "-j",
        help="Output in JSON format",
    ),
) -> None:
    """Show usage statistics for all enabled providers or a specific provider."""

    # Get console, respecting no-color option
    console = Console()

    # Check for JSON mode (from global flag or local option)
    json_mode = json_output or ctx.meta.get("json", False)
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    try:
        if provider:
            # Single provider
            start_time = time.monotonic()
            result = await fetch_provider_usage(provider, refresh)
            duration_ms = (time.monotonic() - start_time) * 1000

            if json_mode:
                output_single_provider_json(result)
            elif result.success:
                display_snapshot(
                    console,
                    result.snapshot,
                    result.source,
                    result.cached,
                    verbose=verbose,
                    quiet=quiet,
                    duration_ms=duration_ms,
                )
            else:
                if not quiet:
                    console.print(f"[red]Error:[/red] {result.error}")
                raise typer.Exit(ExitCode.GENERAL_ERROR)
        else:
            # All enabled providers
            start_time = time.monotonic()
            results = await fetch_all_usage(refresh)
            duration_ms = (time.monotonic() - start_time) * 1000

            display_multiple_snapshots(
                console,
                results,
                ctx,
                json_mode,
                verbose=verbose,
                quiet=quiet,
                total_duration_ms=duration_ms,
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
            error=Exception(
                f"Unknown provider: {provider_id}. Available: {', '.join(available)}"
            ),
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


def display_snapshot(
    console,
    snapshot,
    source,
    cached,
    verbose: bool = False,
    quiet: bool = False,
    duration_ms: float = 0,
):
    """Display a single usage snapshot with spec-compliant format."""
    from vibeusage.cli.display import SingleProviderDisplay

    # In quiet mode, show minimal output
    if quiet:
        for period in snapshot.periods:
            console.print(f"{snapshot.provider} {period.name}: {period.utilization}%")
        return

    # Verbose: add timing and source info before the main display
    if verbose:
        if duration_ms > 0:
            console.print(f"Fetched in {duration_ms:.0f}ms", style="dim")
        if snapshot.identity and snapshot.identity.email:
            console.print(f"Account: {snapshot.identity.email}", style="dim")
        if source:
            console.print(f"Source: {source}", style="dim")
        if verbose:
            console.print()  # Blank line before main display

    # Use spec-compliant SingleProviderDisplay for single provider view
    display = SingleProviderDisplay(snapshot, cached=cached, source=source)
    console.print(display)


def output_single_provider_json(outcome) -> None:
    """Output single provider usage data in JSON format."""
    from vibeusage.display.json import from_vibeusage_error
    from vibeusage.display.json import output_json_pretty
    from vibeusage.errors.classify import classify_exception

    if outcome.success and outcome.snapshot:
        snapshot = outcome.snapshot
        data = {
            "provider": snapshot.provider,
            "identity": {
                "email": snapshot.identity.email,
                "organization": snapshot.identity.organization,
                "plan": snapshot.identity.plan,
            }
            if snapshot.identity
            else None,
            "periods": [
                {
                    "name": period.name,
                    "utilization": period.utilization,
                    "remaining": period.remaining(),
                    "resets_at": period.resets_at.isoformat()
                    if period.resets_at
                    else None,
                    "period_type": period.period_type.value,
                    "model": period.model,
                }
                for period in snapshot.periods
            ],
            "source": outcome.source,
            "cached": outcome.cached,
        }

        if snapshot.overage and snapshot.overage.is_enabled:
            data["overage"] = {
                "used": float(snapshot.overage.used),
                "limit": float(snapshot.overage.limit),
                "remaining": float(snapshot.overage.remaining()),
                "currency": snapshot.overage.currency,
            }

        output_json_pretty(data)
    else:
        # Use standardized error response format
        error = outcome.error
        if error:
            classified = classify_exception(error)
            # Use from_vibeusage_error if we have a VibeusageError, otherwise output_json_error
            if hasattr(classified, "category") and hasattr(classified, "severity"):
                error_response = from_vibeusage_error(classified)
                output_json_pretty(error_response.to_dict())
            else:
                output_json_pretty(
                    {
                        "error": {
                            "message": str(error),
                            "category": "unknown",
                            "severity": "recoverable",
                            "provider": outcome.provider_id,
                        }
                    }
                )
        else:
            output_json_pretty(
                {
                    "error": {
                        "message": "Unknown error occurred",
                        "category": "unknown",
                        "severity": "recoverable",
                        "provider": outcome.provider_id,
                    }
                }
            )


def output_json_usage(outcomes: dict) -> None:
    """Output usage data in JSON format."""
    from datetime import datetime

    from vibeusage.display.json import output_json_pretty

    data = {
        "providers": {},
        "errors": {},
        "fetched_at": datetime.now().astimezone().isoformat(),
    }

    for provider_id, outcome in outcomes.items():
        if outcome.success and outcome.snapshot:
            snapshot = outcome.snapshot
            provider_data = {
                "provider": snapshot.provider,
                "identity": {
                    "email": snapshot.identity.email,
                    "organization": snapshot.identity.organization,
                    "plan": snapshot.identity.plan,
                }
                if snapshot.identity
                else None,
                "periods": [
                    {
                        "name": period.name,
                        "utilization": period.utilization,
                        "remaining": period.remaining(),
                        "resets_at": period.resets_at.isoformat()
                        if period.resets_at
                        else None,
                        "period_type": period.period_type.value,
                        "model": period.model,
                    }
                    for period in snapshot.periods
                ],
                "source": outcome.source,
                "cached": outcome.cached,
            }

            if snapshot.overage and snapshot.overage.is_enabled:
                provider_data["overage"] = {
                    "used": float(snapshot.overage.used),
                    "limit": float(snapshot.overage.limit),
                    "remaining": float(snapshot.overage.remaining()),
                    "currency": snapshot.overage.currency,
                }

            data["providers"][provider_id] = provider_data
        else:
            data["errors"][provider_id] = (
                str(outcome.error) if outcome.error else "Unknown error"
            )

    output_json_pretty(data)


def display_multiple_snapshots(
    console,
    outcomes,
    ctx: typer.Context | None = None,
    json_mode: bool = False,
    verbose: bool = False,
    quiet: bool = False,
    total_duration_ms: float = 0,
):
    """Display multiple provider outcomes using panel-based layout per spec."""
    # Check for JSON mode (from parameter or context)
    if json_mode or (ctx and ctx.meta.get("json")):
        output_json_usage(outcomes)
        return

    # Check if any data
    has_data = any(o.success and o.snapshot for o in outcomes.values())

    if not has_data:
        if not quiet:
            console.print("[yellow]No usage data available[/yellow]")
            console.print("\nConfigure credentials with:")
            console.print("  vibeusage key <provider> set")
        return

    # Import ProviderPanel for spec-compliant panel-based display
    from vibeusage.cli.display import ProviderPanel

    errors = []

    for provider_id, outcome in outcomes.items():
        if outcome.success and outcome.snapshot:
            # Quiet mode: minimal output
            if quiet:
                for period in outcome.snapshot.periods:
                    console.print(f"{provider_id} {period.name}: {period.utilization}%")
            else:
                # Create and print provider panel for this provider
                panel = ProviderPanel(
                    outcome.snapshot,
                    cached=outcome.cached,
                )
                console.print(panel)

                # Verbose: add timing and source info per provider
                if verbose:
                    if outcome.attempts:
                        duration = sum(a.duration_ms for a in outcome.attempts)
                        console.print(
                            Text(
                                f"Fetched in {duration:.0f}ms via {outcome.source}",
                                style="dim",
                            )
                        )
        else:
            # Track errors for verbose output
            if outcome.error:
                errors.append((provider_id, outcome.error))

    # Verbose: show total fetch time and errors
    if verbose and not quiet:
        if total_duration_ms > 0:
            console.print(f"\n[dim]Total fetch time: {total_duration_ms:.0f}ms[/dim]")
        if errors:
            console.print("\n[red]Errors:[/red]")
            for provider_id, error in errors:
                console.print(f"  {provider_id}: {error}")
    # Show errors if all providers failed (even without verbose)
    elif not has_data and errors and not quiet:
        console.print("\n[red]Errors:[/red]")
        for provider_id, error in errors:
            console.print(f"  {provider_id}: {error}")


def format_period(period, verbose: bool = False):
    """Format a usage period for display."""
    from rich.text import Text

    text = Text()

    # Bar based on utilization - using integer division for 5% segments (width=20)
    bar_width = 20
    filled = period.utilization * bar_width // 100
    bar = "█" * filled + "░" * (bar_width - filled)

    # Color based on pace
    color = get_pace_color(period)

    text.append(f"{bar} ", style=color)
    text.append(f"{period.utilization}% ", style="bold")
    text.append(period.name, style="dim")

    # Verbose: show model info
    if verbose and period.model:
        text.append(f" ({period.model})", style="dim")

    # Reset time
    if period.resets_at:
        from vibeusage.models import format_reset_countdown

        time_until = (
            period.time_until_reset() if hasattr(period, "time_until_reset") else None
        )
        if time_until is not None:
            time_str = format_reset_countdown(time_until)
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

    pace_ratio = period.pace_ratio() if hasattr(period, "pace_ratio") else None
    color_name = pace_to_color(pace_ratio, period.utilization)
    return color_name
