"""Data models for vibeusage.

Defines normalized data structures that all providers must produce.
These models abstract provider-specific API responses into a consistent format.
"""

from __future__ import annotations

from datetime import datetime
from datetime import timedelta
from decimal import Decimal
from enum import StrEnum

import msgspec


class PeriodType(StrEnum):
    """Usage period types with their durations."""

    SESSION = "session"  # Short-term window (typically 5 hours)
    DAILY = "daily"  # 24-hour window
    WEEKLY = "weekly"  # 7-day window
    MONTHLY = "monthly"  # 30-day window

    @property
    def hours(self) -> float:
        """Return the duration in hours."""
        match self:
            case PeriodType.SESSION:
                return 5.0
            case PeriodType.DAILY:
                return 24.0
            case PeriodType.WEEKLY:
                return 7.0 * 24.0
            case PeriodType.MONTHLY:
                return 30.0 * 24.0


class UsagePeriod(msgspec.Struct, frozen=True):
    """A usage rate window (e.g., 5-hour session, 7-day weekly)."""

    name: str  # Display name (e.g., "Session (5h)", "Weekly")
    utilization: int  # 0-100 percentage used
    period_type: PeriodType  # Type determines duration
    resets_at: datetime | None = None  # When the window resets (UTC)
    model: str | None = None  # Model-specific window (e.g., "opus", "sonnet")

    def remaining(self) -> int:
        """Return percentage remaining (100 - utilization)."""
        return 100 - self.utilization

    def elapsed_ratio(self) -> float | None:
        """
        Calculate ratio of time elapsed in current period (0.0 to 1.0).
        Returns None if reset time unknown.
        """
        if self.resets_at is None:
            return None

        now = datetime.now(self.resets_at.tzinfo)
        total_hours = self.period_type.hours
        start_time = self.resets_at - timedelta(hours=total_hours)
        elapsed = (now - start_time).total_seconds() / 3600.0
        return max(0.0, min(elapsed / total_hours, 1.0))

    def pace_ratio(self) -> float | None:
        """
        Calculate usage pace ratio.

        Returns the ratio of actual usage to expected usage based on elapsed time.
        - 1.0 = exactly on pace
        - <1.0 = under pace (good)
        - >1.0 = over pace (concerning)

        Returns None if elapsed time is too small (<10%) for meaningful calculation.
        """
        elapsed = self.elapsed_ratio()
        if elapsed is None or elapsed < 0.10:
            return None

        expected_utilization = elapsed * 100.0
        if expected_utilization <= 0:
            return None

        return self.utilization / expected_utilization

    def time_until_reset(self) -> timedelta | None:
        """Return time remaining until reset."""
        if self.resets_at is None:
            return None
        now = datetime.now(self.resets_at.tzinfo)
        return max(timedelta(0), self.resets_at - now)


class OverageUsage(msgspec.Struct, frozen=True):
    """Extra usage / overage cost tracking."""

    used: Decimal  # Amount used (credits or currency)
    limit: Decimal  # Monthly limit
    currency: str  # Currency code (e.g., "USD", "credits")
    is_enabled: bool  # Whether overage is enabled for this account

    def remaining(self) -> Decimal:
        """Return remaining limit."""
        return max(Decimal(0), self.limit - self.used)

    def utilization(self) -> int:
        """Return usage as 0-100 percentage."""
        if self.limit <= 0:
            return 100 if self.used > 0 else 0
        return min(100, int((self.used / self.limit) * 100))


class ProviderIdentity(msgspec.Struct, frozen=True):
    """Account and plan information."""

    email: str | None = None  # Account email
    organization: str | None = None  # Organization name
    plan: str | None = None  # Plan tier (e.g., "free", "pro", "max")


class StatusLevel(StrEnum):
    """Provider operational status levels."""

    OPERATIONAL = "operational"
    DEGRADED = "degraded"
    PARTIAL_OUTAGE = "partial_outage"
    MAJOR_OUTAGE = "major_outage"
    UNKNOWN = "unknown"


class ProviderStatus(msgspec.Struct, frozen=True):
    """Provider health status."""

    level: StatusLevel
    description: str | None = None  # Current incident description
    updated_at: datetime | None = None  # When status was last checked

    @classmethod
    def operational(cls) -> type[ProviderStatus]:
        """Factory for operational status."""
        return cls(level=StatusLevel.OPERATIONAL)

    @classmethod
    def unknown(cls) -> type[ProviderStatus]:
        """Factory for unknown status."""
        return cls(level=StatusLevel.UNKNOWN)


class UsageSnapshot(msgspec.Struct, frozen=True):
    """Complete usage snapshot from a provider."""

    provider: str  # Provider identifier (e.g., "claude", "codex")
    fetched_at: datetime  # When this data was fetched
    periods: tuple[
        UsagePeriod, ...
    ] = ()  # Rate windows (session, weekly, model-specific)
    overage: OverageUsage | None = None  # Extra usage if applicable
    identity: ProviderIdentity | None = None  # Account info
    status: ProviderStatus | None = None  # Provider health
    source: str | None = None  # How data was fetched ("oauth", "web", "cli")

    def primary_period(self) -> UsagePeriod | None:
        """Return the primary (shortest) rate window."""
        if not self.periods:
            return None
        # Prefer session > daily > weekly > monthly
        priority = {
            PeriodType.SESSION: 0,
            PeriodType.DAILY: 1,
            PeriodType.WEEKLY: 2,
            PeriodType.MONTHLY: 3,
        }
        return min(self.periods, key=lambda p: priority.get(p.period_type, 99))

    def secondary_period(self) -> UsagePeriod | None:
        """Return the secondary (longer) rate window."""
        if len(self.periods) < 2:
            return None
        primary = self.primary_period()
        for period in self.periods:
            if period != primary and period.model is None:
                return period
        return None

    def model_periods(self) -> tuple[UsagePeriod, ...]:
        """Return model-specific rate windows."""
        return tuple(p for p in self.periods if p.model is not None)

    def is_stale(self, max_age_minutes: int = 10) -> bool:
        """Check if snapshot is older than max_age_minutes."""
        age = datetime.now(self.fetched_at.tzinfo) - self.fetched_at
        return age.total_seconds() > max_age_minutes * 60


def validate_usage_period(period: UsagePeriod) -> list[str]:
    """Return list of validation errors, empty if valid."""
    errors = []
    if not 0 <= period.utilization <= 100:
        errors.append(f"utilization {period.utilization} out of range [0, 100]")
    return errors


def validate_snapshot(snapshot: UsageSnapshot) -> list[str]:
    """Return list of validation errors, empty if valid."""
    errors = []
    if not snapshot.periods:
        errors.append("at least one period required")
    for period in snapshot.periods:
        errors.extend(validate_usage_period(period))
    return errors


def format_reset_countdown(delta: timedelta | None) -> str:
    """Format reset time as countdown string."""
    if delta is None:
        return ""

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


def pace_to_color(pace_ratio: float | None, utilization: int) -> str:
    """
    Determine display color based on pace ratio.

    Falls back to threshold-based coloring if pace unavailable.
    """
    if pace_ratio is None:
        # Threshold fallback for early periods or missing data
        if utilization < 50:
            return "green"
        elif utilization < 80:
            return "yellow"
        else:
            return "red"

    # Pace-based coloring
    if pace_ratio <= 1.15:
        return "green"  # On or under pace
    elif pace_ratio <= 1.30:
        return "yellow"  # Slightly over pace
    else:
        return "red"  # Significantly over pace
