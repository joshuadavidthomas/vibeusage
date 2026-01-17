"""Failure gate to prevent retry flapping for vibeusage."""

from __future__ import annotations

from dataclasses import dataclass
from dataclasses import field
from datetime import datetime
from datetime import timedelta

from vibeusage.config.paths import gate_dir
from vibeusage.errors.types import ErrorCategory


@dataclass(frozen=True)
class FailureRecord:
    """A record of a single failure."""

    timestamp: datetime
    error_category: ErrorCategory
    message: str


# Gate constants
MAX_CONSECUTIVE_FAILURES = 3
GATE_DURATION = timedelta(minutes=5)
WINDOW_DURATION = timedelta(minutes=10)


@dataclass
class FailureGate:
    """Tracks failures and implements backoff gating.

    After MAX_CONSECUTIVE_FAILURES within WINDOW_DURATION,
    the gate closes for GATE_DURATION.
    """

    provider_id: str
    failures: list[FailureRecord] = field(default_factory=list)
    gated_until: datetime | None = None
    consecutive_count: int = 0

    def record_failure(self, error_category: ErrorCategory, message: str) -> None:
        """Record a failure and update gate state."""
        now = datetime.now()
        record = FailureRecord(
            timestamp=now, error_category=error_category, message=message
        )

        # Clean old failures outside the window
        cutoff = now - WINDOW_DURATION
        self.failures = [f for f in self.failures if f.timestamp > cutoff]

        self.failures.append(record)
        self.consecutive_count += 1

        # Check if we should gate
        if self.consecutive_count >= MAX_CONSECUTIVE_FAILURES:
            self.gated_until = now + GATE_DURATION

    def record_success(self) -> None:
        """Record a success and reset consecutive failure count."""
        self.consecutive_count = 0

    def is_gated(self) -> bool:
        """Check if the provider is currently gated."""
        if self.gated_until is None:
            return False

        if datetime.now() > self.gated_until:
            # Gate has expired
            self.gated_until = None
            return False

        return True

    def gate_remaining(self) -> timedelta | None:
        """Get time remaining until gate opens."""
        if self.gated_until is None:
            return None

        remaining = self.gated_until - datetime.now()
        if remaining.total_seconds() <= 0:
            return None
        return remaining

    def recent_failures(self, limit: int = 5) -> list[FailureRecord]:
        """Get recent failure records for diagnostics."""
        return self.failures[-limit:]

    def clear(self) -> None:
        """Clear all failure state."""
        self.failures.clear()
        self.gated_until = None
        self.consecutive_count = 0


# Global gate storage
_gates: dict[str, FailureGate] = {}


def get_failure_gate(provider_id: str) -> FailureGate:
    """Get or create a failure gate for a provider."""
    if provider_id not in _gates:
        # Try to load from disk
        _gates[provider_id] = load_gate(provider_id) or FailureGate(provider_id)
    return _gates[provider_id]


def gate_path(provider_id: str) -> str:
    """Get the file path for a gate's state."""
    return str(gate_dir() / f"{provider_id}.json")


def load_gate(provider_id: str) -> FailureGate | None:
    """Load gate state from disk."""
    from vibeusage.config.cache import load_cached_gate_state

    state = load_cached_gate_state(provider_id)
    if not state:
        return None

    gate = FailureGate(provider_id)
    gate.gated_until = state.get("gated_until")

    # Reconstruct failure records
    failures = state.get("failures", [])
    for f in failures:
        gate.failures.append(
            FailureRecord(
                timestamp=datetime.fromisoformat(f["timestamp"]),
                error_category=ErrorCategory[f["error_category"]],
                message=f["message"],
            )
        )

    return gate


def save_gate(gate: FailureGate) -> None:
    """Save gate state to disk."""
    from vibeusage.config.cache import cache_gate_state

    failures_data = [
        {
            "timestamp": f.timestamp.isoformat(),
            "error_category": f.error_category.name,
            "message": f.message,
        }
        for f in gate.failures
    ]

    cache_gate_state(gate.provider_id, failures_data, gate.gated_until)


def clear_gate(provider_id: str) -> None:
    """Clear a provider's gate state."""
    gate = get_failure_gate(provider_id)
    gate.clear()
    save_gate(gate)
