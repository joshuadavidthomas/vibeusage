"""Pytest configuration and shared fixtures for vibeusage tests."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from decimal import Decimal
from pathlib import Path
from typing import Generator
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import httpx

from vibeusage.models import (
    OverageUsage,
    PeriodType,
    ProviderIdentity,
    ProviderStatus,
    StatusLevel,
    UsagePeriod,
    UsageSnapshot,
)
from vibeusage.errors.types import (
    ErrorCategory,
    ErrorSeverity,
    HTTPErrorMapping,
    VibeusageError,
)
from vibeusage.core.gate import FailureGate, FailureRecord


@pytest.fixture
def utc_now() -> datetime:
    """Fixed UTC datetime for consistent testing."""
    return datetime(2025, 1, 15, 12, 0, 0, tzinfo=timezone.utc)


@pytest.fixture
def future_time(utc_now: datetime) -> datetime:
    """Time 2 hours in the future."""
    return utc_now + timedelta(hours=2)


@pytest.fixture
def past_time(utc_now: datetime) -> datetime:
    """Time 2 hours in the past."""
    return utc_now - timedelta(hours=2)


@pytest.fixture
def sample_period(utc_now: datetime) -> UsagePeriod:
    """Sample usage period with known values."""
    return UsagePeriod(
        name="Session (5h)",
        utilization=65,
        period_type=PeriodType.SESSION,
        resets_at=utc_now + timedelta(hours=3),
    )


@pytest.fixture
def sample_period_daily(utc_now: datetime) -> UsagePeriod:
    """Sample daily usage period."""
    return UsagePeriod(
        name="Daily",
        utilization=45,
        period_type=PeriodType.DAILY,
        resets_at=utc_now + timedelta(hours=12),
    )


@pytest.fixture
def sample_period_weekly(utc_now: datetime) -> UsagePeriod:
    """Sample weekly usage period."""
    return UsagePeriod(
        name="Weekly",
        utilization=30,
        period_type=PeriodType.WEEKLY,
        resets_at=utc_now + timedelta(days=3),
    )


@pytest.fixture
def sample_period_with_model(utc_now: datetime) -> UsagePeriod:
    """Sample model-specific usage period."""
    return UsagePeriod(
        name="Opus",
        utilization=80,
        period_type=PeriodType.DAILY,
        model="opus",
    )


@pytest.fixture
def sample_overage() -> OverageUsage:
    """Sample overage usage."""
    return OverageUsage(
        used=Decimal("2.50"),
        limit=Decimal("15.00"),
        currency="USD",
        is_enabled=True,
    )


@pytest.fixture
def sample_identity() -> ProviderIdentity:
    """Sample provider identity."""
    return ProviderIdentity(
        email="user@example.com",
        organization="Acme Corp",
        plan="pro",
    )


@pytest.fixture
def sample_status() -> ProviderStatus:
    """Sample operational provider status."""
    return ProviderStatus(
        level=StatusLevel.OPERATIONAL,
        description=None,
        updated_at=datetime.now(timezone.utc),
    )


@pytest.fixture
def sample_snapshot(
    utc_now: datetime,
    sample_period: UsagePeriod,
    sample_overage: OverageUsage,
    sample_identity: ProviderIdentity,
    sample_status: ProviderStatus,
) -> UsageSnapshot:
    """Sample complete usage snapshot."""
    return UsageSnapshot(
        provider="claude",
        fetched_at=utc_now,
        periods=(sample_period,),
        overage=sample_overage,
        identity=sample_identity,
        status=sample_status,
        source="oauth",
    )


@pytest.fixture
def sample_multi_period_snapshot(
    utc_now: datetime,
    sample_period: UsagePeriod,
    sample_period_daily: UsagePeriod,
    sample_period_with_model: UsagePeriod,
) -> UsageSnapshot:
    """Sample snapshot with multiple periods."""
    return UsageSnapshot(
        provider="claude",
        fetched_at=utc_now,
        periods=(sample_period, sample_period_daily, sample_period_with_model),
        overage=None,
        identity=None,
        status=None,
        source="web",
    )


@pytest.fixture
def sample_error() -> VibeusageError:
    """Sample VibeusageError."""
    return VibeusageError(
        message="Authentication failed",
        category=ErrorCategory.AUTHENTICATION,
        severity=ErrorSeverity.FATAL,
        provider="claude",
        remediation="Run 'vibeusage auth claude' to re-authenticate",
        details={"status_code": 401},
    )


@pytest.fixture
def http_error_mapping_401() -> HTTPErrorMapping:
    """HTTP error mapping for 401."""
    return HTTPErrorMapping(
        category=ErrorCategory.AUTHENTICATION,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    )


@pytest.fixture
def http_error_mapping_429() -> HTTPErrorMapping:
    """HTTP error mapping for 429 rate limit."""
    return HTTPErrorMapping(
        category=ErrorCategory.RATE_LIMITED,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=False,
        retry_after_header=True,
    )


@pytest.fixture
def http_error_mapping_500() -> HTTPErrorMapping:
    """HTTP error mapping for 500 server error."""
    return HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    )


@pytest.fixture
def sample_failure_gate() -> FailureGate:
    """Sample failure gate."""
    return FailureGate(provider_id="claude")


@pytest.fixture
def gated_failure_gate() -> FailureGate:
    """Failure gate that is currently gated."""
    gate = FailureGate(provider_id="codex")
    gate.consecutive_count = 3
    gate.gated_until = datetime.now() + timedelta(minutes=3)
    gate.failures = [
        FailureRecord(
            timestamp=datetime.now() - timedelta(minutes=2),
            error_category=ErrorCategory.NETWORK,
            message="Connection timeout",
        ),
        FailureRecord(
            timestamp=datetime.now() - timedelta(minutes=1),
            error_category=ErrorCategory.NETWORK,
            message="Connection timeout",
        ),
        FailureRecord(
            timestamp=datetime.now(),
            error_category=ErrorCategory.NETWORK,
            message="Connection timeout",
        ),
    ]
    return gate


@pytest.fixture
def mock_httpx_client() -> httpx.AsyncClient:
    """Mock httpx.AsyncClient."""
    client = MagicMock(spec=httpx.AsyncClient)
    client.get = AsyncMock()
    client.post = AsyncMock()
    client.__aenter__ = AsyncMock(return_value=client)
    client.__aexit__ = AsyncMock()
    client.is_closed = False
    client.aclose = AsyncMock()
    return client


@pytest.fixture
def mock_response() -> MagicMock:
    """Mock httpx.Response."""
    response = MagicMock(spec=httpx.Response)
    response.status_code = 200
    response.json = MagicMock(return_value={})
    response.text = ""
    response.headers = httpx.Headers({})
    return response


@pytest.fixture
def temp_config_dir(tmp_path: Path) -> Generator[Path, None, None]:
    """Temporary config directory for testing."""
    config_dir = tmp_path / "config"
    cache_dir = tmp_path / "cache"
    config_dir.mkdir()
    cache_dir.mkdir()

    with patch("vibeusage.config.paths.config_dir", return_value=config_dir), patch(
        "vibeusage.config.paths.cache_dir", return_value=cache_dir
    ):
        yield config_dir


@pytest.fixture
def sample_config_dict() -> dict:
    """Sample configuration dictionary."""
    return {
        "enabled_providers": ["claude", "codex"],
        "display": {
            "show_remaining": True,
            "pace_colors": True,
            "reset_format": "countdown",
        },
        "fetch": {
            "timeout": 30,
            "max_concurrent": 5,
            "stale_threshold": 10,
        },
        "credentials": {
            "use_keyring": False,
            "reuse_provider_credentials": True,
        },
        "providers": {
            "claude": {
                "enabled": True,
                "timeout": 30,
            },
            "codex": {
                "enabled": True,
                "timeout": 30,
            },
        },
    }


@pytest.fixture
def auth_credentials_dict() -> dict:
    """Sample OAuth credentials dictionary."""
    return {
        "access_token": "test_access_token",
        "refresh_token": "test_refresh_token",
        "token_type": "Bearer",
        "expires_at": (datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(),
        "scope": "usage.read",
    }


@pytest.fixture
def sample_usage_response() -> dict:
    """Sample Claude API usage response."""
    return {
        "organization_id": "org_123",
        "usage": {
            "session_usage": {
                "input_tokens": 45000,
                "output_tokens": 15000,
            },
            "period_start": "2025-01-15T07:00:00Z",
            "period_end": "2025-01-15T12:00:00Z",
            "tier": "claude_pro",
        },
        "rate_limits": {
            "seven_day": {
                "remaining": 35000,
                "limit": 50000,
            },
            "monthly": {
                "remaining": 150000,
                "limit": 200000,
            },
        },
    }


@pytest.fixture
def sample_statuspage_response() -> dict:
    """Sample statuspage.io API response."""
    return {
        "page": {
            "id": "abc123",
            "name": "Claude API",
            "url": "https://status.anthropic.com",
            "updated_at": "2025-01-15T12:00:00Z",
        },
        "indicators": [
            {
                "id": "def456",
                "name": "API Systems",
                "status": "operational",
            }
        ],
    }


@pytest.fixture
def mock_async_mock() -> AsyncMock:
    """Factory for creating AsyncMock instances."""
    return AsyncMock()


# Test data for edge cases
@pytest.fixture
def edge_case_snapshots(utc_now: datetime) -> dict[str, UsageSnapshot]:
    """Dictionary of snapshots for edge case testing."""
    return {
        "zero_utilization": UsageSnapshot(
            provider="claude",
            fetched_at=utc_now,
            periods=(
                UsagePeriod(
                    name="Empty", utilization=0, period_type=PeriodType.DAILY
                ),
            ),
        ),
        "full_utilization": UsageSnapshot(
            provider="claude",
            fetched_at=utc_now,
            periods=(
                UsagePeriod(
                    name="Full", utilization=100, period_type=PeriodType.DAILY
                ),
            ),
        ),
        "no_periods": UsageSnapshot(
            provider="claude", fetched_at=utc_now, periods=()
        ),
        "stale": UsageSnapshot(
            provider="claude",
            fetched_at=utc_now - timedelta(minutes=15),
            periods=(
                UsagePeriod(
                    name="Old", utilization=50, period_type=PeriodType.DAILY
                ),
            ),
        ),
        "future_reset": UsageSnapshot(
            provider="claude",
            fetched_at=utc_now,
            periods=(
                UsagePeriod(
                    name="Future",
                    utilization=50,
                    period_type=PeriodType.DAILY,
                    resets_at=utc_now + timedelta(days=7),
                ),
            ),
        ),
    }
