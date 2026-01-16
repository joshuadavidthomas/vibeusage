# Spec 02: Data Models

**Status**: Draft
**Dependencies**: None (foundation spec)
**Dependents**: 03-authentication, 01-architecture, 04-providers, 05-cli-interface

## Overview

This specification defines the normalized data structures that all providers must produce. These models abstract provider-specific API responses into a consistent format for display and caching.

## Design Goals

1. **Provider Agnostic**: Models work across all 12+ supported providers
2. **Rich Display**: Support pace-based coloring and countdown formatting
3. **Serializable**: Native JSON serialization via msgspec for caching and `--json` output
4. **Immutable**: Structs are frozen by default to prevent accidental mutation
5. **Type Safe**: Full type annotations with runtime validation
6. **Performance**: msgspec provides 10-75x faster serialization than stdlib json

## Core Data Models

### Dependencies

```toml
# pyproject.toml
dependencies = [
    "msgspec>=0.18",
]
```

### PeriodType

Enumeration of rate window types with associated durations.

```python
from enum import StrEnum

class PeriodType(StrEnum):
    """Usage period types with their durations."""

    SESSION = "session"      # Short-term window (typically 5 hours)
    DAILY = "daily"          # 24-hour window
    WEEKLY = "weekly"        # 7-day window
    MONTHLY = "monthly"      # 30-day window

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
```

**Rationale**: Different providers use different window sizes. SESSION covers Claude's 5-hour window, DAILY covers providers with 24-hour resets, WEEKLY is common across most providers, and MONTHLY supports billing-cycle aligned quotas.

### UsagePeriod

A single rate window with utilization and reset information.

```python
import msgspec
from datetime import datetime, timedelta

class UsagePeriod(msgspec.Struct, frozen=True):
    """A usage rate window (e.g., 5-hour session, 7-day weekly)."""

    name: str                           # Display name (e.g., "Session (5h)", "Weekly")
    utilization: int                    # 0-100 percentage used
    period_type: PeriodType             # Type determines duration
    resets_at: datetime | None = None   # When the window resets (UTC)
    model: str | None = None            # Model-specific window (e.g., "opus", "sonnet")

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
```

**Key Design Decisions**:

1. **Utilization as percentage**: Normalized to 0-100 regardless of how providers report (some use fractions, some use remaining counts).

2. **Pace calculation**: The `pace_ratio()` method enables smarter coloring than fixed thresholds. At 50% through the period, 50% usage = pace 1.0. At 10% through with 20% used = pace 2.0 (concerning).

3. **Model-specific windows**: Some providers (Claude, Antigravity) have per-model quotas. The `model` field captures this without requiring separate classes.

4. **Frozen dataclass**: Immutability prevents bugs from accidental mutation.

### OverageUsage

Extra usage / cost tracking beyond included quota.

```python
from decimal import Decimal

class OverageUsage(msgspec.Struct, frozen=True):
    """Extra usage / overage cost tracking."""

    used: Decimal              # Amount used (credits or currency)
    limit: Decimal             # Monthly limit
    currency: str              # Currency code (e.g., "USD", "credits")
    is_enabled: bool           # Whether overage is enabled for this account

    def remaining(self) -> Decimal:
        """Return remaining limit."""
        return max(Decimal(0), self.limit - self.used)

    def utilization(self) -> int:
        """Return usage as 0-100 percentage."""
        if self.limit <= 0:
            return 100 if self.used > 0 else 0
        return min(100, int((self.used / self.limit) * 100))
```

**Rationale**: Decimal is used instead of float for financial data precision. The currency field is a string to support both currency codes ("USD") and unit types ("credits").

### ProviderIdentity

Account information and plan details.

```python
class ProviderIdentity(msgspec.Struct, frozen=True):
    """Account and plan information."""

    email: str | None = None           # Account email
    organization: str | None = None    # Organization name
    plan: str | None = None            # Plan tier (e.g., "free", "pro", "max")
```

### ProviderStatus

Provider health and incident information.

```python
from enum import StrEnum

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
    description: str | None = None     # Current incident description
    updated_at: datetime | None = None # When status was last checked

    @classmethod
    def operational(cls) -> "ProviderStatus":
        """Factory for operational status."""
        return cls(level=StatusLevel.OPERATIONAL)

    @classmethod
    def unknown(cls) -> "ProviderStatus":
        """Factory for unknown status."""
        return cls(level=StatusLevel.UNKNOWN)
```

### UsageSnapshot

Complete usage data from a provider at a point in time.

```python
from datetime import datetime

class UsageSnapshot(msgspec.Struct, frozen=True):
    """Complete usage snapshot from a provider."""

    provider: str                                # Provider identifier (e.g., "claude", "codex")
    fetched_at: datetime                         # When this data was fetched
    periods: tuple[UsagePeriod, ...]             # Rate windows (session, weekly, model-specific)
    overage: OverageUsage | None = None          # Extra usage if applicable
    identity: ProviderIdentity | None = None     # Account info
    status: ProviderStatus | None = None         # Provider health
    source: str | None = None                    # How data was fetched ("oauth", "web", "cli")

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
```

**Key Design Decisions**:

1. **Tuple for periods**: Immutable collection that preserves order. Providers may have 1-4 rate windows.

2. **Primary/secondary helpers**: Simplifies display logic. Primary is typically the session window, secondary is the weekly window.

3. **Source tracking**: Records which fetch strategy succeeded (oauth/web/cli) for debugging and user information.

4. **Staleness check**: Enables displaying cached data with a "stale" indicator when fresh data can't be fetched.

## Provider-Specific Mapping

How provider API responses map to these models:

### Claude

| API Field | Model Field |
|-----------|-------------|
| `five_hour.utilization` | `UsagePeriod(name="Session", period_type=SESSION)` |
| `seven_day.utilization` | `UsagePeriod(name="Weekly", period_type=WEEKLY)` |
| `seven_day_opus.utilization` | `UsagePeriod(name="Opus", period_type=WEEKLY, model="opus")` |
| `seven_day_sonnet.utilization` | `UsagePeriod(name="Sonnet", period_type=WEEKLY, model="sonnet")` |
| `overage_spend_limit.*` | `OverageUsage` |

### Codex (OpenAI)

| API Field | Model Field |
|-----------|-------------|
| `rate_limits.primary.used_percent` | `UsagePeriod(period_type=SESSION)` |
| `rate_limits.secondary.used_percent` | `UsagePeriod(period_type=WEEKLY)` |
| `credits.balance` | `OverageUsage` (if credits enabled) |

### Copilot

| API Field | Model Field |
|-----------|-------------|
| `premium_interactions.percent_remaining` | `UsagePeriod` (inverted to utilization) |
| `chat.percent_remaining` | `UsagePeriod(model="chat")` |
| `quota_reset_date` | All periods' `resets_at` |

### Cursor

| API Field | Model Field |
|-----------|-------------|
| `premium_requests.used / available` | `UsagePeriod` |
| `on_demand_spend.used_cents / limit_cents` | `OverageUsage` |

### Gemini

| API Field | Model Field |
|-----------|-------------|
| `quota_buckets[].remaining_fraction` | `UsagePeriod` per model (inverted) |
| `user_tier` | `ProviderIdentity.plan` |

## Serialization with msgspec

msgspec handles serialization natively. No manual `to_dict()`/`from_dict()` methods needed.

### Encoder/Decoder Setup

```python
import msgspec

# Create encoder/decoder with custom handling for Decimal
encoder = msgspec.json.Encoder()
decoder = msgspec.json.Decoder(UsageSnapshot)

# Serialize
json_bytes: bytes = encoder.encode(snapshot)
json_str: str = json_bytes.decode("utf-8")

# Deserialize
snapshot: UsageSnapshot = decoder.decode(json_bytes)

# Or use the simpler functions
json_bytes = msgspec.json.encode(snapshot)
snapshot = msgspec.json.decode(json_bytes, type=UsageSnapshot)
```

### JSON Output Format

msgspec produces JSON with ISO 8601 datetimes and string representation for Decimals:

```json
{
  "provider": "claude",
  "fetched_at": "2026-01-16T10:30:00+00:00",
  "periods": [
    {
      "name": "Session (5h)",
      "utilization": 45,
      "period_type": "session",
      "resets_at": "2026-01-16T15:30:00+00:00",
      "model": null
    },
    {
      "name": "Weekly",
      "utilization": 23,
      "period_type": "weekly",
      "resets_at": "2026-01-20T00:00:00+00:00",
      "model": null
    }
  ],
  "overage": {
    "used": "5.50",
    "limit": "100.00",
    "currency": "USD",
    "is_enabled": true
  },
  "identity": {
    "email": "user@example.com",
    "organization": "Personal",
    "plan": "max"
  },
  "status": {
    "level": "operational",
    "description": null,
    "updated_at": "2026-01-16T10:00:00+00:00"
  },
  "source": "oauth"
}
```

### Type Coercion and Validation

msgspec validates types during deserialization:

```python
# This will raise msgspec.ValidationError if the JSON doesn't match the schema
try:
    snapshot = msgspec.json.decode(data, type=UsageSnapshot)
except msgspec.ValidationError as e:
    print(f"Invalid data: {e}")
```

## Cache Structure

Cached snapshots are stored per-provider:

```
~/.cache/vibeusage/
├── snapshots/
│   ├── claude.json
│   ├── codex.json
│   └── cursor.json
└── org-ids/
    ├── claude
    └── codex
```

The snapshot files contain msgspec-serialized JSON:

```python
def cache_snapshot(snapshot: UsageSnapshot, path: Path) -> None:
    """Cache a snapshot to disk."""
    path.write_bytes(msgspec.json.encode(snapshot))

def load_snapshot(path: Path) -> UsageSnapshot | None:
    """Load a cached snapshot."""
    try:
        return msgspec.json.decode(path.read_bytes(), type=UsageSnapshot)
    except (FileNotFoundError, msgspec.ValidationError):
        return None
```

## Validation Rules

1. **utilization**: Must be 0-100 inclusive
2. **resets_at**: Must be in the future or within the current period
3. **currency**: Non-empty string
4. **periods**: At least one period required for a valid snapshot

```python
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
```

## Display Helpers

Utility functions for CLI rendering (used by spec 05):

```python
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
        return "green"   # On or under pace
    elif pace_ratio <= 1.30:
        return "yellow"  # Slightly over pace
    else:
        return "red"     # Significantly over pace
```

## Open Questions

1. **Token cost tracking**: Should we include a separate `TokenUsage` model for providers that expose token counts and per-token pricing? CodexBar has this via JSONL log scanning.

2. **Historical data**: Should snapshots include historical utilization for trend display? This would require breaking immutability or a separate `UsageHistory` model.

3. **Multi-org support**: Some providers (Claude, Codex) support multiple organizations. Should `UsageSnapshot` include org ID, or should there be multiple snapshots?

## Implementation Notes

- All models should be in `vibeusage/models.py`
- Use `from __future__ import annotations` for forward references
- Use `msgspec.Struct` with `frozen=True` for all data models
- msgspec provides native support for `datetime`, `Decimal`, and `Enum` types
- msgspec validates types during deserialization - no separate validation needed
- For optional fields with defaults, put them after required fields in the struct definition
- Use `tuple[T, ...]` instead of `list[T]` for immutable collections
