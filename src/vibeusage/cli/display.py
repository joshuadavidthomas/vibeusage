"""Rich display components for vibeusage CLI."""

from __future__ import annotations

from datetime import datetime, timedelta

from rich.console import Console, RenderableType
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from vibeusage.display.rich import format_period, format_overage_used
from vibeusage.errors.types import ErrorCategory, ErrorSeverity, VibeusageError
from vibeusage.models import UsageSnapshot, format_reset_countdown, PeriodType


class SingleProviderDisplay:
    """Rich renderable for displaying a single provider's usage with title+separator format.

    This produces the spec-compliant single provider output:
    - Provider name as title (e.g., "Claude")
    - Separator line with ━ characters
    - Session periods standalone (not indented)
    - Blank line separator
    - Weekly/Daily/Monthly section header (bold)
    - Model-specific periods indented with 2 spaces
    - Overage in a Panel (only part wrapped in a panel)
    """

    def __init__(
        self,
        snapshot: UsageSnapshot,
        cached: bool = False,
        source: str | None = None,
    ):
        """Initialize display with usage snapshot.

        Args:
            snapshot: UsageSnapshot to display
            cached: Whether data is from cache
            source: Data source (e.g., "oauth", "web", "cli")
        """
        self.snapshot = snapshot
        self.cached = cached
        self.source = source

    def __rich_console__(self, console: Console, options: dict) -> RenderableType:
        """Render the single provider usage display."""
        # Title line with provider name (capitalized per spec)
        title = Text(self.snapshot.provider.title(), style="bold")
        yield title

        # Separator line (80 dashes)
        separator = "━" * 80
        yield Text(separator, style="dim")

        # Create grid for period display
        grid = Table.grid(padding=(0, 2))
        grid.add_column(min_width=12, justify="left")  # Period name
        grid.add_column(min_width=22, justify="left")  # Bar + percentage
        grid.add_column(justify="right")  # Reset time

        # Group periods by type for proper display per spec
        session_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.SESSION]
        weekly_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.WEEKLY]
        daily_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.DAILY]
        monthly_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.MONTHLY]

        # Display session periods first (typically "Session (5h)") - not indented
        if session_periods:
            for period in session_periods:
                grid.add_row(
                    Text(period.name, style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )
            # Add separator if we have other periods to show
            if weekly_periods or daily_periods or monthly_periods:
                yield grid
                yield Text()  # Blank line after session
                grid = Table.grid(padding=(0, 2))
                grid.add_column(min_width=12, justify="left")
                grid.add_column(min_width=22, justify="left")
                grid.add_column(justify="right")

        # Display longer periods with header and indented model-specific periods
        longer_periods = weekly_periods or daily_periods or monthly_periods
        if longer_periods:
            # Determine the header name based on period type
            if weekly_periods:
                header_name = "Weekly"
                periods_to_show = weekly_periods
            elif daily_periods:
                header_name = "Daily"
                periods_to_show = daily_periods
            else:  # monthly_periods
                header_name = "Monthly"
                periods_to_show = monthly_periods

            # Separate general periods from model-specific periods
            general_periods = [p for p in periods_to_show if p.model is None]
            model_periods = [p for p in periods_to_show if p.model is not None]

            # Add header row
            yield Text(header_name, style="bold")

            # Add general periods - show "All Models" for non-model-specific periods per spec
            for period in general_periods:
                grid.add_row(
                    Text("  All Models", style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )

            # Add model-specific periods (e.g., "Opus", "Sonnet") - indented
            for period in model_periods:
                grid.add_row(
                    Text(f"  {period.name}"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )

        # Add any remaining periods (not session/weekly/daily/monthly)
        handled_types = {PeriodType.SESSION, PeriodType.WEEKLY, PeriodType.DAILY, PeriodType.MONTHLY}
        remaining_periods = [p for p in self.snapshot.periods if p.period_type not in handled_types]
        for period in remaining_periods:
            grid.add_row(
                Text(period.name, style="bold"),
                self._format_bar_and_percentage(period),
                self._format_reset_time(period),
            )

        yield grid

        # Add overage in a panel if enabled
        if self.snapshot.overage and self.snapshot.overage.is_enabled:
            overage = self.snapshot.overage
            symbol = "$" if overage.currency == "USD" else ""
            overage_text = Text(f"Extra Usage: {symbol}{overage.used:.2f} / {symbol}{overage.limit:.2f} {overage.currency}")
            yield Panel(
                overage_text,
                title="Overage",
                border_style="cyan",
                padding=(0, 1),
            )

    def _format_bar_and_percentage(self, period) -> Text:
        """Format the progress bar and percentage column."""
        from vibeusage.display.rich import render_usage_bar
        from vibeusage.models import pace_to_color

        text = Text()
        pace_ratio = period.pace_ratio() if hasattr(period, 'pace_ratio') else None
        color = pace_to_color(pace_ratio, period.utilization)
        bar = render_usage_bar(period.utilization, color=color)
        text.append_text(bar)
        text.append(f" {period.utilization}%", style=color)
        return text

    def _format_reset_time(self, period) -> Text:
        """Format the reset time column."""
        text = Text()
        time_until = period.time_until_reset() if hasattr(period, 'time_until_reset') else None
        if time_until is not None:
            time_str = format_reset_countdown(time_until)
            text.append(f"resets in {time_str}", style="dim")
        return text


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
        """Render the provider panel using grid layout per spec."""
        from vibeusage.models import PeriodType

        # Create a grid with 3 columns: period name, bar+percentage, reset time
        grid = Table.grid(padding=(0, 2))
        grid.add_column(min_width=12, justify="left")  # Period name
        grid.add_column(min_width=22, justify="left")  # Bar + percentage
        grid.add_column(justify="right")  # Reset time

        # In multi-provider (compact) view, skip model-specific periods
        # Only show general periods where model is None
        session_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.SESSION and p.model is None]
        weekly_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.WEEKLY and p.model is None]
        daily_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.DAILY and p.model is None]
        monthly_periods = [p for p in self.snapshot.periods if p.period_type == PeriodType.MONTHLY and p.model is None]

        # Display session periods first - use their specific name per spec 05
        if session_periods:
            for period in session_periods:
                grid.add_row(
                    Text(period.name, style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )

        # Display longer periods (compact view - no model-specific breakdown)
        # Use period type name (e.g., "Weekly") instead of period name per spec 05
        if weekly_periods:
            for period in weekly_periods:
                grid.add_row(
                    Text("Weekly", style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )
        if daily_periods:
            for period in daily_periods:
                grid.add_row(
                    Text("Daily", style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )
        if monthly_periods:
            for period in monthly_periods:
                grid.add_row(
                    Text("Monthly", style="bold"),
                    self._format_bar_and_percentage(period),
                    self._format_reset_time(period),
                )

        # Add any remaining periods (not session/weekly/daily/monthly) - skip model-specific
        handled_types = {PeriodType.SESSION, PeriodType.WEEKLY, PeriodType.DAILY, PeriodType.MONTHLY}
        remaining_periods = [p for p in self.snapshot.periods if p.period_type not in handled_types and p.model is None]
        for period in remaining_periods:
            grid.add_row(
                Text(period.name, style="bold"),
                self._format_bar_and_percentage(period),
                self._format_reset_time(period),
            )

        # Add overage if enabled
        if self.snapshot.overage and self.snapshot.overage.is_enabled:
            overage = self.snapshot.overage
            overage_text = Text()
            symbol = "$" if overage.currency == "USD" else ""
            overage_text.append(f"Extra Usage: {symbol}{overage.used:.2f} / {symbol}{overage.limit:.2f} {overage.currency}")
            grid.add_row(
                overage_text,
                Text(),
                Text(),
            )

        # Create panel with provider name as title (e.g., "Claude", "Codex")
        yield Panel(
            grid,
            title=self.snapshot.provider.title(),
            border_style="dim",
            padding=(0, 1),
        )

    def _format_bar_and_percentage(self, period) -> Text:
        """Format the progress bar and percentage column."""
        from vibeusage.display.rich import render_usage_bar
        from vibeusage.models import pace_to_color

        text = Text()
        pace_ratio = period.pace_ratio() if hasattr(period, 'pace_ratio') else None
        color = pace_to_color(pace_ratio, period.utilization)
        bar = render_usage_bar(period.utilization, color=color)
        text.append_text(bar)
        text.append(f" {period.utilization}%", style=color)
        return text

    def _format_reset_time(self, period) -> Text:
        """Format the reset time column."""
        text = Text()
        time_until = period.time_until_reset() if hasattr(period, 'time_until_reset') else None
        if time_until is not None:
            time_str = format_reset_countdown(time_until)
            text.append(f"resets in {time_str}", style="dim")
        return text


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
