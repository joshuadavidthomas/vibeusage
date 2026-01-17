"""Rich display components for vibeusage CLI."""

from __future__ import annotations

from datetime import datetime, timedelta

from rich.console import Console, RenderableType
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from vibeusage.display.rich import format_period, format_overage_used
from vibeusage.errors.types import ErrorCategory, ErrorSeverity, VibeusageError
from vibeusage.models import UsageSnapshot, format_reset_countdown


class UsageDisplay:
    """Rich renderable for displaying a single provider's usage."""

    def __init__(self, snapshot: UsageSnapshot, cached: bool = False):
        """Initialize display with usage snapshot.

        Args:
            snapshot: UsageSnapshot to display
            cached: Whether data is from cache
        """
        self.snapshot = snapshot
        self.cached = cached

    def __rich_console__(self, console: Console, options: dict) -> RenderableType:
        """Render the usage snapshot as a Rich console output."""
        lines = []

        # Header
        header = Text()
        header.append(f"{self.snapshot.provider.upper()} ", style="bold")
        if self.cached:
            header.append("(cached) ", style="dim yellow")
        if self.snapshot.source:
            header.append(f"via {self.snapshot.source}", style="dim")
        lines.append(header)

        # Periods
        for period in self.snapshot.periods:
            period_text = format_period(period)
            lines.append(period_text)

        # Overage
        if self.snapshot.overage and self.snapshot.overage.is_enabled:
            overage = self.snapshot.overage
            overage_text = format_overage_used(overage.used, overage.limit, overage.currency)
            lines.append(overage_text)

        # Identity
        if self.snapshot.identity:
            identity_parts = []
            if self.snapshot.identity.email:
                identity_parts.append(self.snapshot.identity.email)
            if self.snapshot.identity.plan:
                identity_parts.append(f"{self.snapshot.identity.plan} plan")
            if self.snapshot.identity.organization:
                identity_parts.append(self.snapshot.identity.organization)
            if identity_parts:
                identity_text = Text()
                identity_text.append(" • ".join(identity_parts), style="dim")
                lines.append(identity_text)

        yield Panel.fit(
            "\n".join(str(line) for line in lines),
            title=f"{self.snapshot.provider} Usage",
            border_style="cyan" if not self.cached else "yellow",
        )


class ProviderPanel:
    """Rich panel wrapper for multi-provider display."""

    def __init__(
        self,
        snapshot: UsageSnapshot,
        cached: bool = False,
        show_age: bool = False,
    ):
        """Initialize provider panel.

        Args:
            snapshot: UsageSnapshot to display
            cached: Whether data is from cache
            show_age: Whether to show data age
        """
        self.snapshot = snapshot
        self.cached = cached
        self.show_age = show_age

    def __rich_console__(self, console: Console, options: dict) -> RenderableType:
        """Render the provider panel."""
        lines = []

        # Title line
        title = Text()
        title.append(f"{self.snapshot.provider.upper()} ", style="bold cyan")

        if self.cached:
            title.append("(cached)", style="yellow")

        if self.show_age:
            age = self._get_age()
            if age:
                title.append(f" • {age} old", style="dim")

        lines.append(title)

        # Periods
        for period in self.snapshot.periods:
            period_text = format_period(period)
            lines.append(period_text)

        # Overage
        if self.snapshot.overage and self.snapshot.overage.is_enabled:
            overage = self.snapshot.overage
            overage_text = format_overage_used(overage.used, overage.limit, overage.currency)
            lines.append(overage_text)

        yield Panel.fit(
            "\n".join(str(line) for line in lines),
            border_style="cyan" if not self.cached else "yellow",
        )

    def _get_age(self) -> str | None:
        """Calculate human-readable age of snapshot."""
        age = datetime.now(self.snapshot.fetched_at.tzinfo) - self.snapshot.fetched_at
        return format_timedelta(age)


class ErrorDisplay:
    """Rich renderable for displaying structured errors."""

    def __init__(self, error: VibeusageError):
        """Initialize error display.

        Args:
            error: VibeusageError to display
        """
        self.error = error

    def __rich_console__(self, console: Console, options: dict) -> RenderableType:
        """Render the error as a Rich console output."""
        # Color by severity
        colors = {
            ErrorSeverity.FATAL: "red",
            ErrorSeverity.RECOVERABLE: "yellow",
            ErrorSeverity.TRANSIENT: "yellow",
            ErrorSeverity.WARNING: "dim",
        }
        color = colors.get(self.error.severity, "red")

        # Build message content
        content = Text()
        content.append(self.error.message, style=color)

        if self.error.remediation:
            content.append("\n\n")
            content.append(self.error.remediation, style="dim")

        # Title includes provider if available
        title = "Error"
        if self.error.provider:
            title = f"{self.error.provider.title()} Error"

        # Add category to title for debugging
        if self.error.category != ErrorCategory.UNKNOWN:
            title = f"{title} [{self.error.category}]"

        yield Panel(
            content,
            title=title,
            border_style=color,
            title_align="left",
        )


def show_error(error: VibeusageError | Exception, console: Console | None = None) -> None:
    """Display a formatted error message.

    Args:
        error: VibeusageError or Exception to display
        console: Rich console (uses default if None)
    """
    from vibeusage.errors.classify import classify_exception

    if console is None:
        console = Console()

    # Convert plain Exception to VibeusageError
    if not isinstance(error, VibeusageError):
        vibe_error = classify_exception(error)
    else:
        vibe_error = error

    console.print(ErrorDisplay(vibe_error))


def show_partial_failures(
    failures: dict[str, VibeusageError | Exception],
    console: Console | None = None,
) -> None:
    """Show summary of partial provider failures.

    Args:
        failures: Dict of provider_id -> error
        console: Rich console (uses default if None)
    """
    if not failures:
        return

    from vibeusage.errors.classify import classify_exception

    if console is None:
        console = Console()

    console.print()
    console.print("[dim]─── Errors ───[/dim]")

    for provider_id, error in failures.items():
        # Convert to VibeusageError if needed
        if not isinstance(error, VibeusageError):
            vibe_error = classify_exception(error, provider_id)
        else:
            vibe_error = error

        console.print(f"[red]{provider_id}[/red]: {vibe_error.message}")
        if vibe_error.remediation:
            # Remove color codes for simpler display
            remediation = vibe_error.remediation.replace("[cyan]", "").replace("[/cyan]", "")
            console.print(f"  [dim]{remediation}[/dim]")


def show_stale_warning(
    snapshot: UsageSnapshot,
    max_age_minutes: int = 10,
    console: Console | None = None,
) -> None:
    """Display a warning for stale cached data.

    Args:
        snapshot: UsageSnapshot to check
        max_age_minutes: Maximum age before showing warning
        console: Rich console (uses default if None)
    """
    if console is None:
        console = Console()

    age = datetime.now(snapshot.fetched_at.tzinfo) - snapshot.fetched_at
    age_minutes = int(age.total_seconds() / 60)

    if age_minutes >= max_age_minutes:
        if age_minutes < 60:
            age_str = f"{age_minutes} minute{'s' if age_minutes != 1 else ''}"
        else:
            hours = age_minutes // 60
            age_str = f"{hours} hour{'s' if hours != 1 else ''}"

        console.print(f"[yellow]⚠ Showing cached data from {age_str} ago[/yellow]")
        console.print(
            f"[dim]Run with [cyan]--refresh[/cyan] to fetch fresh data[/dim]"
        )


def show_diagnostic_info(console: Console | None = None) -> None:
    """Show diagnostic information for troubleshooting.

    Args:
        console: Rich console (uses default if None)
    """
    import sys
    import platform

    from vibeusage import __version__
    from vibeusage.config.paths import get_cache_dir, get_config_dir
    from vibeusage.core.gate import get_failure_gate
    from vibeusage.providers import list_provider_ids

    if console is None:
        console = Console()

    console.print("[bold]Diagnostic Information[/bold]")
    console.print()

    console.print(f"vibeusage version: {__version__}")
    console.print(f"Python version: {sys.version.split()[0]}")
    console.print(f"Platform: {platform.platform()}")
    console.print()

    console.print(f"Config directory: {get_config_dir()}")
    console.print(f"Cache directory: {get_cache_dir()}")
    console.print()

    # Show credential status
    from vibeusage.config.credentials import check_provider_credentials

    console.print("[dim]Credential Status:[/dim]")
    for provider_id in list_provider_ids():
        status, source = check_provider_credentials(provider_id)
        source_str = f" ({source})" if source else ""
        console.print(f"  {provider_id}: {status}{source_str}")

    console.print()

    # Show failure gate status
    gate = get_failure_gate()
    console.print("[dim]Failure Gate Status:[/dim]")
    for provider_id in list_provider_ids():
        provider_gate = get_failure_gate(provider_id)
        if provider_gate.is_gated():
            remaining = provider_gate.gate_remaining()
            if remaining:
                console.print(
                    f"  {provider_id}: [red]gated[/red] ({format_timedelta(remaining)} remaining)"
                )
        else:
            failures = provider_gate.recent_failures()
            if failures:
                console.print(f"  {provider_id}: {len(failures)} recent failure(s)")
            else:
                console.print(f"  {provider_id}: [green]ok[/green]")


def format_timedelta(delta: timedelta) -> str:
    """Format a timedelta as a human-readable string.

    Args:
        delta: Timedelta to format

    Returns:
        Formatted string (e.g., "5m", "2h 30m", "1d 4h")
    """
    total_seconds = int(delta.total_seconds())
    if total_seconds <= 0:
        return "now"

    days, remainder = divmod(total_seconds, 86400)
    hours, remainder = divmod(remainder, 3600)
    minutes = remainder // 60

    if days > 0:
        return f"{days}d {hours}h"
    elif hours > 0:
        return f"{hours}h {minutes}m"
    else:
        return f"{minutes}m"


def calculate_age_minutes(snapshot: UsageSnapshot) -> int:
    """Calculate the age of a snapshot in minutes.

    Args:
        snapshot: UsageSnapshot to check

    Returns:
        Age in minutes
    """
    age = datetime.now(snapshot.fetched_at.tzinfo) - snapshot.fetched_at
    return int(age.total_seconds() / 60)
