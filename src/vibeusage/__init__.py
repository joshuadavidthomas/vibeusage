"""vibeusage: Track usage across agentic LLM providers."""

__version__ = "0.1.0"

from vibeusage.models import (
    PeriodType,
    UsagePeriod,
    OverageUsage,
    ProviderIdentity,
    StatusLevel,
    ProviderStatus,
    UsageSnapshot,
    validate_usage_period,
    validate_snapshot,
    format_reset_countdown,
    pace_to_color,
)

__all__ = [
    "__version__",
    "PeriodType",
    "UsagePeriod",
    "OverageUsage",
    "ProviderIdentity",
    "StatusLevel",
    "ProviderStatus",
    "UsageSnapshot",
    "validate_usage_period",
    "validate_snapshot",
    "format_reset_countdown",
    "pace_to_color",
]


def main() -> None:
    """Entry point for the vibeusage CLI."""
    from vibeusage.cli.app import run_app

    run_app()
