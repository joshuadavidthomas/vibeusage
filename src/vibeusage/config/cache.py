"""Snapshot and org ID caching for vibeusage."""
from __future__ import annotations

import json
from datetime import datetime
from datetime import timedelta
from pathlib import Path

import msgspec

from vibeusage.config.paths import gate_dir
from vibeusage.config.paths import org_ids_dir
from vibeusage.config.paths import snapshots_dir
from vibeusage.models import UsageSnapshot


def snapshot_path(provider_id: str) -> Path:
    """Get path for provider's cached snapshot."""
    return snapshots_dir() / f"{provider_id}.msgpack"


def cache_snapshot(snapshot: UsageSnapshot) -> None:
    """Save usage snapshot to cache."""
    path = snapshot_path(snapshot.provider)
    path.parent.mkdir(parents=True, exist_ok=True)

    data = msgspec.json.encode(snapshot)
    path.write_bytes(data)


def load_cached_snapshot(provider_id: str) -> UsageSnapshot | None:
    """Load cached snapshot for provider."""
    path = snapshot_path(provider_id)
    if not path.exists():
        return None

    try:
        data = path.read_bytes()
        return msgspec.json.decode(data, type=UsageSnapshot)
    except (msgspec.DecodeError, json.JSONDecodeError, OSError):
        return None


def is_snapshot_fresh(
    provider_id: str,
    stale_threshold_minutes: int = 60,
) -> bool:
    """Check if cached snapshot is within staleness threshold."""
    snapshot = load_cached_snapshot(provider_id)
    if snapshot is None:
        return False

    age = datetime.now(tz=snapshot.fetched_at.tzinfo) - snapshot.fetched_at
    return age < timedelta(minutes=stale_threshold_minutes)


def get_snapshot_age_minutes(provider_id: str) -> int | None:
    """Get age of cached snapshot in minutes."""
    snapshot = load_cached_snapshot(provider_id)
    if snapshot is None:
        return None

    age = datetime.now(tz=snapshot.fetched_at.tzinfo) - snapshot.fetched_at
    return int(age.total_seconds() / 60)


# Org ID caching
def org_id_path(provider_id: str) -> Path:
    """Get path for provider's cached org ID."""
    return org_ids_dir() / f"{provider_id}.txt"


def cache_org_id(provider_id: str, org_id: str) -> None:
    """Save org ID to cache."""
    path = org_id_path(provider_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(org_id)


def load_cached_org_id(provider_id: str) -> str | None:
    """Load cached org ID for provider."""
    path = org_id_path(provider_id)
    if not path.exists():
        return None

    try:
        return path.read_text().strip()
    except OSError:
        return None


def clear_org_id_cache(provider_id: str | None = None) -> None:
    """Clear org ID cache for a provider or all providers."""
    if provider_id:
        path = org_id_path(provider_id)
        if path.exists():
            path.unlink()
    else:
        for path in org_ids_dir().iterdir():
            if path.is_file():
                path.unlink()


# Snapshot cache clearing
def clear_provider_cache(provider_id: str) -> None:
    """Clear all cached data for a provider."""
    snapshot_path(provider_id).unlink(missing_ok=True)
    org_id_path(provider_id).unlink(missing_ok=True)


def clear_snapshot_cache(provider_id: str | None = None) -> None:
    """Clear snapshot cache for a provider or all providers."""
    if provider_id:
        snapshot_path(provider_id).unlink(missing_ok=True)
    else:
        for path in snapshots_dir().iterdir():
            if path.is_file():
                path.unlink()


def clear_all_cache(provider_id: str | None = None) -> None:
    """Clear all cached data."""
    if provider_id:
        clear_provider_cache(provider_id)
    else:
        for path in snapshots_dir().iterdir():
            if path.is_file():
                path.unlink()
        for path in org_ids_dir().iterdir():
            if path.is_file():
                path.unlink()


# Failure gate persistence
def gate_path(provider_id: str) -> Path:
    """Get path for provider's failure gate state."""
    return gate_dir() / f"{provider_id}.msgpack"


def cache_gate_state(
    provider_id: str,
    failures: list[dict],
    gated_until: datetime | None = None,
) -> None:
    """Save failure gate state to cache."""
    import msgspec

    path = gate_path(provider_id)
    path.parent.mkdir(parents=True, exist_ok=True)

    state = {
        "failures": failures,
        "gated_until": gated_until.isoformat() if gated_until else None,
    }

    data = msgspec.json.encode(state)
    path.write_bytes(data)


def load_cached_gate_state(provider_id: str) -> dict | None:
    """Load cached gate state for provider."""
    import msgspec

    path = gate_path(provider_id)
    if not path.exists():
        return None

    try:
        data = path.read_bytes()
        state = msgspec.json.decode(data)
        if state.get("gated_until"):
            state["gated_until"] = datetime.fromisoformat(state["gated_until"])
        return state
    except (msgspec.DecodeError, OSError, ValueError):
        return None
