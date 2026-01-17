"""Tests for vibeusage.models data structures."""
from __future__ import annotations

from datetime import datetime
from datetime import timedelta
from datetime import timezone
from decimal import Decimal

import pytest

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


class TestPeriodType:
    """Tests for PeriodType enum."""

    def test_period_type_values(self):
        """PeriodType has correct values."""
        assert PeriodType.SESSION == "session"
        assert PeriodType.DAILY == "daily"
        assert PeriodType.WEEKLY == "weekly"
        assert PeriodType.MONTHLY == "monthly"

    def test_session_duration(self):
        """SESSION period is 5 hours."""
        assert PeriodType.SESSION.hours == 5.0

    def test_daily_duration(self):
        """DAILY period is 24 hours."""
        assert PeriodType.DAILY.hours == 24.0

    def test_weekly_duration(self):
        """WEEKLY period is 7 days."""
        assert PeriodType.WEEKLY.hours == 7.0 * 24.0

    def test_monthly_duration(self):
        """MONTHLY period is 30 days."""
        assert PeriodType.MONTHLY.hours == 30.0 * 24.0


class TestUsagePeriod:
    """Tests for UsagePeriod."""

    def test_create_usage_period(self):
        """Can create a UsagePeriod with required fields."""
        now = datetime.now(timezone.utc)
        period = UsagePeriod(
            name="Session (5h)",
            utilization=50,
            period_type=PeriodType.SESSION,
            resets_at=now + timedelta(hours=3),
        )

        assert period.name == "Session (5h)"
        assert period.utilization == 50
        assert period.period_type == PeriodType.SESSION
        assert period.resets_at == now + timedelta(hours=3)
        assert period.model is None

    def test_create_with_model(self):
        """Can create a model-specific period."""
        period = UsagePeriod(
            name="Opus",
            utilization=75,
            period_type=PeriodType.DAILY,
            model="opus",
        )

        assert period.model == "opus"

    def test_remaining(self):
        """remaining() returns 100 - utilization."""
        period = UsagePeriod(name="Test", utilization=30, period_type=PeriodType.DAILY)
        assert period.remaining() == 70

    def test_remaining_zero(self):
        """remaining() is 0 when utilization is 100."""
        period = UsagePeriod(name="Test", utilization=100, period_type=PeriodType.DAILY)
        assert period.remaining() == 0

    def test_elapsed_ratio_with_reset_time(self):
        """elapsed_ratio() calculates time correctly."""
        now = datetime.now(timezone.utc)
        reset_time = now + timedelta(hours=12)
        period = UsagePeriod(
            name="Daily",
            utilization=50,
            period_type=PeriodType.DAILY,
            resets_at=reset_time,
        )

        ratio = period.elapsed_ratio()
        assert ratio is not None
        assert 0.45 <= ratio <= 0.55  # 12 hours into 24 hour period

    def test_elapsed_ratio_without_reset_time(self):
        """elapsed_ratio() returns None when no reset time."""
        period = UsagePeriod(
            name="Daily", utilization=50, period_type=PeriodType.DAILY, resets_at=None
        )
        assert period.elapsed_ratio() is None

    def test_elapsed_ratio_bounds(self):
        """elapsed_ratio() is bounded between 0 and 1."""
        now = datetime.now(timezone.utc)
        reset_time = now - timedelta(hours=1)
        period = UsagePeriod(
            name="Daily",
            utilization=50,
            period_type=PeriodType.DAILY,
            resets_at=reset_time,
        )

        ratio = period.elapsed_ratio()
        assert ratio == 1.0

    def test_pace_ratio_with_elapsed(self):
        """pace_ratio() calculates usage pace."""
        now = datetime.now(timezone.utc)
        reset_time = now + timedelta(hours=12)
        period = UsagePeriod(
            name="Daily",
            utilization=50,
            period_type=PeriodType.DAILY,
            resets_at=reset_time,
        )

        pace = period.pace_ratio()
        assert pace is not None
        assert 0.9 <= pace <= 1.1

    def test_pace_ratio_early_period(self):
        """pace_ratio() returns None for early period (<10% elapsed)."""
        now = datetime.now(timezone.utc)
        reset_time = now + timedelta(hours=23)
        period = UsagePeriod(
            name="Daily",
            utilization=5,
            period_type=PeriodType.DAILY,
            resets_at=reset_time,
        )

        pace = period.pace_ratio()
        assert pace is None

    def test_pace_ratio_without_reset(self):
        """pace_ratio() returns None when no reset time."""
        period = UsagePeriod(
            name="Daily", utilization=50, period_type=PeriodType.DAILY, resets_at=None
        )
        assert period.pace_ratio() is None

    def test_pace_ratio_over_pace(self):
        """pace_ratio() > 1.0 when using too fast."""
        now = datetime.now(timezone.utc)
        reset_time = now + timedelta(hours=6)
        period = UsagePeriod(
            name="Daily",
            utilization=80,
            period_type=PeriodType.DAILY,
            resets_at=reset_time,
        )

        pace = period.pace_ratio()
        assert pace is not None
        # With 80% used and 6 hours remaining in 24-hour period (75% elapsed),
        # pace ratio = 80 / 75 = 1.067
        assert pace > 1.0
        assert pace < 1.1

    def test_time_until_reset(self):
        """time_until_reset() returns correct timedelta."""
        now = datetime.now(timezone.utc)
        reset_time = now + timedelta(hours=3, minutes=30)
        period = UsagePeriod(
            name="Session",
            utilization=50,
            period_type=PeriodType.SESSION,
            resets_at=reset_time,
        )

        remaining = period.time_until_reset()
        assert remaining is not None
        assert 3.4 * 3600 <= remaining.total_seconds() <= 3.6 * 3600

    def test_time_until_reset_none(self):
        """time_until_reset() returns None when no reset time."""
        period = UsagePeriod(
            name="Session",
            utilization=50,
            period_type=PeriodType.SESSION,
            resets_at=None,
        )
        assert period.time_until_reset() is None

    def test_time_until_reset_past(self, utc_now):
        """time_until_reset() returns 0 timedelta when reset passed."""
        period = UsagePeriod(
            name="Session",
            utilization=50,
            period_type=PeriodType.SESSION,
            resets_at=utc_now - timedelta(hours=1),
        )

        remaining = period.time_until_reset()
        assert remaining == timedelta(0)

    def test_immutability(self):
        """UsagePeriod is immutable (frozen)."""
        period = UsagePeriod(name="Test", utilization=50, period_type=PeriodType.DAILY)
        with pytest.raises(AttributeError):
            period.utilization = 75


class TestOverageUsage:
    """Tests for OverageUsage."""

    def test_create_overage(self):
        """Can create OverageUsage."""
        overage = OverageUsage(
            used=Decimal("5.25"),
            limit=Decimal("20.00"),
            currency="USD",
            is_enabled=True,
        )

        assert overage.used == Decimal("5.25")
        assert overage.limit == Decimal("20.00")
        assert overage.currency == "USD"
        assert overage.is_enabled is True

    def test_remaining(self):
        """remaining() calculates correctly."""
        overage = OverageUsage(
            used=Decimal("5.00"),
            limit=Decimal("20.00"),
            currency="USD",
            is_enabled=True,
        )
        assert overage.remaining() == Decimal("15.00")

    def test_remaining_no_negative(self):
        """remaining() never goes below 0."""
        overage = OverageUsage(
            used=Decimal("25.00"),
            limit=Decimal("20.00"),
            currency="USD",
            is_enabled=True,
        )
        assert overage.remaining() == Decimal("0")

    def test_utilization_percentage(self):
        """utilization() returns percentage."""
        overage = OverageUsage(
            used=Decimal("10.00"),
            limit=Decimal("20.00"),
            currency="USD",
            is_enabled=True,
        )
        assert overage.utilization() == 50

    def test_utilization_caps_at_100(self):
        """utilization() caps at 100."""
        overage = OverageUsage(
            used=Decimal("30.00"),
            limit=Decimal("20.00"),
            currency="USD",
            is_enabled=True,
        )
        assert overage.utilization() == 100

    def test_utilization_zero_limit(self):
        """utilization() handles zero limit."""
        overage = OverageUsage(
            used=Decimal("10.00"), limit=Decimal("0"), currency="USD", is_enabled=True
        )
        # With zero limit but positive usage, returns 100
        assert overage.utilization() == 100

    def test_utilization_zero_limit_zero_usage(self):
        """utilization() returns 0 when both zero."""
        overage = OverageUsage(
            used=Decimal("0"), limit=Decimal("0"), currency="USD", is_enabled=True
        )
        assert overage.utilization() == 0

    def test_credits_currency(self):
        """Can use credits instead of currency."""
        overage = OverageUsage(
            used=Decimal("100"),
            limit=Decimal("500"),
            currency="credits",
            is_enabled=True,
        )
        assert overage.utilization() == 20


class TestProviderIdentity:
    """Tests for ProviderIdentity."""

    def test_create_full_identity(self):
        """Can create identity with all fields."""
        identity = ProviderIdentity(
            email="user@example.com", organization="Acme", plan="pro"
        )

        assert identity.email == "user@example.com"
        assert identity.organization == "Acme"
        assert identity.plan == "pro"

    def test_create_partial_identity(self):
        """Can create identity with partial data."""
        identity = ProviderIdentity(email="user@example.com")

        assert identity.email == "user@example.com"
        assert identity.organization is None
        assert identity.plan is None

    def test_create_empty_identity(self):
        """Can create empty identity."""
        identity = ProviderIdentity()

        assert identity.email is None
        assert identity.organization is None
        assert identity.plan is None


class TestProviderStatus:
    """Tests for ProviderStatus."""

    def test_create_status(self):
        """Can create status."""
        now = datetime.now(timezone.utc)
        status = ProviderStatus(
            level=StatusLevel.OPERATIONAL,
            description="All systems normal",
            updated_at=now,
        )

        assert status.level == StatusLevel.OPERATIONAL
        assert status.description == "All systems normal"
        assert status.updated_at == now

    def test_operational_factory(self):
        """operational() factory creates operational status."""
        status = ProviderStatus.operational()
        assert status.level == StatusLevel.OPERATIONAL

    def test_unknown_factory(self):
        """unknown() factory creates unknown status."""
        status = ProviderStatus.unknown()
        assert status.level == StatusLevel.UNKNOWN


class TestStatusLevel:
    """Tests for StatusLevel enum."""

    def test_status_level_values(self):
        """StatusLevel has correct values."""
        assert StatusLevel.OPERATIONAL == "operational"
        assert StatusLevel.DEGRADED == "degraded"
        assert StatusLevel.PARTIAL_OUTAGE == "partial_outage"
        assert StatusLevel.MAJOR_OUTAGE == "major_outage"
        assert StatusLevel.UNKNOWN == "unknown"


class TestUsageSnapshot:
    """Tests for UsageSnapshot."""

    def test_create_snapshot(self, utc_now, sample_period):
        """Can create a snapshot."""
        snapshot = UsageSnapshot(
            provider="claude", fetched_at=utc_now, periods=(sample_period,)
        )

        assert snapshot.provider == "claude"
        assert snapshot.fetched_at == utc_now
        assert len(snapshot.periods) == 1
        assert snapshot.periods[0] == sample_period

    def test_full_snapshot(
        self, utc_now, sample_period, sample_overage, sample_identity
    ):
        """Can create a complete snapshot with all fields."""
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=utc_now,
            periods=(sample_period,),
            overage=sample_overage,
            identity=sample_identity,
            status=ProviderStatus.operational(),
            source="oauth",
        )

        assert snapshot.provider == "claude"
        assert snapshot.overage == sample_overage
        assert snapshot.identity == sample_identity
        assert snapshot.status.level == StatusLevel.OPERATIONAL
        assert snapshot.source == "oauth"

    def test_primary_period_single(self, sample_multi_period_snapshot):
        """primary_period() returns shortest period."""
        primary = sample_multi_period_snapshot.primary_period()
        assert primary.period_type == PeriodType.SESSION

    def test_primary_period_none(self):
        """primary_period() returns None when no periods."""
        snapshot = UsageSnapshot(
            provider="claude", fetched_at=datetime.now(timezone.utc)
        )
        assert snapshot.primary_period() is None

    def test_secondary_period(self, sample_multi_period_snapshot):
        """secondary_period() returns second non-model period."""
        secondary = sample_multi_period_snapshot.secondary_period()
        assert secondary.period_type == PeriodType.DAILY
        assert secondary.model is None

    def test_secondary_period_none(self, sample_period):
        """secondary_period() returns None with only one period."""
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            periods=(sample_period,),
        )
        assert snapshot.secondary_period() is None

    def test_model_periods(self, sample_multi_period_snapshot):
        """model_periods() returns only model-specific periods."""
        model_periods = sample_multi_period_snapshot.model_periods()
        assert len(model_periods) == 1
        assert model_periods[0].model == "opus"

    def test_model_periods_empty(self, sample_period):
        """model_periods() returns empty tuple when none."""
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            periods=(sample_period,),
        )
        assert snapshot.model_periods() == ()

    def test_is_stale_fresh(self):
        """is_stale() returns False for fresh data."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=now - timedelta(minutes=5),
            periods=(
                UsagePeriod(name="Test", utilization=50, period_type=PeriodType.DAILY),
            ),
        )
        assert not snapshot.is_stale()

    def test_is_stale_old(self):
        """is_stale() returns True for old data."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=now - timedelta(minutes=15),
            periods=(
                UsagePeriod(name="Test", utilization=50, period_type=PeriodType.DAILY),
            ),
        )
        assert snapshot.is_stale()

    def test_is_stale_custom_threshold(self):
        """is_stale() respects custom threshold."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=now - timedelta(minutes=5),
            periods=(
                UsagePeriod(name="Test", utilization=50, period_type=PeriodType.DAILY),
            ),
        )
        assert snapshot.is_stale(max_age_minutes=3)


class TestValidateFunctions:
    """Tests for validation functions."""

    def test_validate_usage_period_valid(self):
        """Valid period passes validation."""
        period = UsagePeriod(name="Test", utilization=50, period_type=PeriodType.DAILY)
        errors = validate_usage_period(period)
        assert errors == []

    def test_validate_usage_period_negative(self):
        """Negative utilization fails validation."""
        period = UsagePeriod(name="Test", utilization=-10, period_type=PeriodType.DAILY)
        errors = validate_usage_period(period)
        assert len(errors) == 1
        assert "out of range" in errors[0]

    def test_validate_usage_period_over_100(self):
        """Utilization > 100 fails validation."""
        period = UsagePeriod(name="Test", utilization=150, period_type=PeriodType.DAILY)
        errors = validate_usage_period(period)
        assert len(errors) == 1
        assert "out of range" in errors[0]

    def test_validate_snapshot_valid(self, sample_snapshot):
        """Valid snapshot passes validation."""
        errors = validate_snapshot(sample_snapshot)
        assert errors == []

    def test_validate_snapshot_no_periods(self, utc_now):
        """Snapshot without periods fails validation."""
        snapshot = UsageSnapshot(provider="claude", fetched_at=utc_now, periods=())
        errors = validate_snapshot(snapshot)
        assert len(errors) == 1
        assert "at least one period" in errors[0]

    def test_validate_snapshot_invalid_period(self, utc_now):
        """Snapshot with invalid period fails."""
        bad_period = UsagePeriod(
            name="Bad", utilization=150, period_type=PeriodType.DAILY
        )
        snapshot = UsageSnapshot(
            provider="claude", fetched_at=utc_now, periods=(bad_period,)
        )
        errors = validate_snapshot(snapshot)
        assert len(errors) == 1
        assert "out of range" in errors[0]


class TestFormatResetCountdown:
    """Tests for format_reset_countdown."""

    def test_format_none(self):
        """None returns empty string."""
        assert format_reset_countdown(None) == ""

    def test_format_negative(self):
        """Negative timedelta returns 'now'."""
        assert format_reset_countdown(timedelta(seconds=-10)) == "now"
        assert format_reset_countdown(timedelta(seconds=0)) == "now"

    def test_format_minutes(self):
        """Format minutes only."""
        assert format_reset_countdown(timedelta(minutes=5)) == "5m"
        assert format_reset_countdown(timedelta(minutes=45)) == "45m"

    def test_format_hours(self):
        """Format hours and minutes."""
        assert format_reset_countdown(timedelta(hours=2, minutes=30)) == "2h 30m"
        assert format_reset_countdown(timedelta(hours=1, minutes=5)) == "1h 5m"

    def test_format_days(self):
        """Format days and hours."""
        assert format_reset_countdown(timedelta(days=2, hours=6)) == "2d 6h"
        assert format_reset_countdown(timedelta(days=7, hours=12)) == "7d 12h"

    def test_format_complex(self):
        """Format complex timedelta."""
        delta = timedelta(days=1, hours=2, minutes=34)
        result = format_reset_countdown(delta)
        assert result == "1d 2h"  # Minutes not shown when days present


class TestPaceToColor:
    """Tests for pace_to_color."""

    def test_pace_none_low_utilization(self):
        """None pace with low utilization returns green."""
        assert pace_to_color(None, 30) == "green"

    def test_pace_none_medium_utilization(self):
        """None pace with medium utilization returns yellow."""
        assert pace_to_color(None, 65) == "yellow"

    def test_pace_none_high_utilization(self):
        """None pace with high utilization returns red."""
        assert pace_to_color(None, 85) == "red"

    def test_pace_good(self):
        """Pace at or below 1.15 returns green."""
        assert pace_to_color(1.0, 50) == "green"
        assert pace_to_color(1.15, 50) == "green"

    def test_pace_warning(self):
        """Pace between 1.15 and 1.30 returns yellow."""
        assert pace_to_color(1.20, 50) == "yellow"
        assert pace_to_color(1.30, 50) == "yellow"

    def test_pace_danger(self):
        """Pace above 1.30 returns red."""
        assert pace_to_color(1.31, 50) == "red"
        assert pace_to_color(2.0, 50) == "red"

    def test_pace_zero_utilization(self):
        """Zero utilization edge case."""
        # With pace at 1.0, zero utilization is fine
        assert pace_to_color(1.0, 0) == "green"
