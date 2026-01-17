"""vibeusage: Track usage across agentic LLM providers."""

from __future__ import annotations

__version__ = "0.1.0"

from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import ProviderIdentity
from vibeusage.models import ProviderStatus
from vibeusage.models import StatusLevel
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.models import format_reset_countdown
from vibeusage.models import pace_to_color
from vibeusage.models import validate_snapshot
from vibeusage.models import validate_usage_period

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
