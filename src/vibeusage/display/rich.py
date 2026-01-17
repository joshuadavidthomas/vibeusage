"""Rich-based rendering utilities for vibeusage."""

from __future__ import annotations

from rich.text import Text

from vibeusage.models import UsagePeriod, format_reset_countdown, pace_to_color


def render_usage_bar(
    utilization: int,
    width: int = 20,
    color: str | None = None,
) -> Text:
    """Render a usage progress bar.

    Args:
        utilization: Usage percentage (0-100)
        width: Bar width in characters
        color: Optional color override

    Returns:
        Rich Text with the progress bar
    """
    # Use integer division for consistent bar segments (5% per segment for width=20)
    # For width=20: 100% = 20 blocks, 5% = 1 block (utilization // 5)
    # For general width: scale the calculation proportionally
    filled = utilization * width // 100
    bar = "█" * filled + "░" * (width - filled)

    text = Text()
    text.append(bar, style=color or "default")
    return text


def format_period(period: UsagePeriod) -> Text:
    """Format a usage period for display.

    Args:
        period: UsagePeriod to format

    Returns:
        Rich Text with formatted period info
    """
    text = Text()

    # Progress bar colored by pace
    pace_ratio = period.pace_ratio() if hasattr(period, 'pace_ratio') else None
    color = pace_to_color(pace_ratio, period.utilization)
    bar = render_usage_bar(period.utilization, color=color)

    text.append_text(bar)
    text.append(" ")
    text.append(f"{period.utilization}%", style="bold")
    text.append(f" {period.name}", style="dim")

    # Reset time
    time_until = period.time_until_reset() if hasattr(period, 'time_until_reset') else None
    if time_until is not None:
        time_str = format_reset_countdown(time_until)
        text.append(f" • resets in {time_str}", style="dim")

    return text


def format_overage_used(used: float, limit: float, currency: str = "USD") -> Text:
    """Format overage usage for display.

    Args:
        used: Amount used
        limit: Total limit
        currency: Currency code

    Returns:
        Rich Text with formatted overage info
    """
    text = Text()
    remaining = limit - used

    symbol = "$" if currency == "USD" else ""

    text.append(f"Overage: {symbol}{used:.2f}", style="yellow")
    text.append(f" / {symbol}{limit:.2f}", style="dim")
    text.append(f" ({symbol}{remaining:.2f} remaining)", style="bold yellow")

    return text


def format_period_line(
    name: str,
    utilization: int,
    resets_at_str: str | None = None,
    indent: int = 0,
) -> Text:
    """Format a simple period line for compact display.

    Args:
        name: Period name
        utilization: Usage percentage
        resets_at_str: Pre-formatted reset time string
        indent: Number of spaces to indent

    Returns:
        Rich Text with formatted line
    """
    text = Text()

    if indent > 0:
        text.append(" " * indent)

    # Get color based on utilization (simple threshold for compact view)
    if utilization < 50:
        color = "green"
    elif utilization < 80:
        color = "yellow"
    else:
        color = "red"

    bar_width = 12
    filled = int((utilization / 100) * bar_width)
    bar = "█" * filled + "░" * (bar_width - filled)

    text.append(f"{name:<16} ", style="dim")
    text.append(f"{bar} ", style=color)
    text.append(f"{utilization:>3}%", style="bold")

    if resets_at_str:
        text.append(f"   resets in {resets_at_str}", style="dim")

    return text
