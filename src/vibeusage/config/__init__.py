"""Configuration management for vibeusage."""

from __future__ import annotations

from vibeusage.config.cache import cache_org_id
from vibeusage.config.cache import cache_snapshot
from vibeusage.config.cache import clear_all_cache
from vibeusage.config.cache import clear_org_id_cache
from vibeusage.config.cache import clear_snapshot_cache
from vibeusage.config.cache import is_snapshot_fresh
from vibeusage.config.cache import load_cached_org_id
from vibeusage.config.cache import load_cached_snapshot
from vibeusage.config.cache import snapshot_path
from vibeusage.config.credentials import check_credential_permissions
from vibeusage.config.credentials import check_provider_credentials
from vibeusage.config.credentials import credential_path
from vibeusage.config.credentials import delete_credential
from vibeusage.config.credentials import find_provider_credential
from vibeusage.config.credentials import read_credential
from vibeusage.config.credentials import write_credential
from vibeusage.config.keyring import delete_from_keyring
from vibeusage.config.keyring import get_from_keyring
from vibeusage.config.keyring import keyring_key
from vibeusage.config.keyring import store_in_keyring
from vibeusage.config.keyring import use_keyring
from vibeusage.config.paths import cache_dir
from vibeusage.config.paths import config_dir
from vibeusage.config.paths import config_file
from vibeusage.config.paths import credentials_dir
from vibeusage.config.paths import ensure_directories
from vibeusage.config.paths import gate_dir
from vibeusage.config.paths import org_ids_dir
from vibeusage.config.paths import snapshots_dir
from vibeusage.config.paths import state_dir
from vibeusage.config.settings import Config
from vibeusage.config.settings import DisplayConfig
from vibeusage.config.settings import FetchConfig
from vibeusage.config.settings import ProviderConfig
from vibeusage.config.settings import get_config
from vibeusage.config.settings import load_config
from vibeusage.config.settings import reload_config
from vibeusage.config.settings import save_config

__all__ = [
    # paths
    "config_dir",
    "cache_dir",
    "state_dir",
    "credentials_dir",
    "snapshots_dir",
    "org_ids_dir",
    "gate_dir",
    "config_file",
    "ensure_directories",
    # settings
    "Config",
    "DisplayConfig",
    "FetchConfig",
    "ProviderConfig",
    "get_config",
    "load_config",
    "reload_config",
    "save_config",
    # credentials
    "credential_path",
    "write_credential",
    "read_credential",
    "delete_credential",
    "check_credential_permissions",
    "find_provider_credential",
    "check_provider_credentials",
    # cache
    "snapshot_path",
    "cache_snapshot",
    "load_cached_snapshot",
    "is_snapshot_fresh",
    "org_ids_dir",
    "cache_org_id",
    "load_cached_org_id",
    "clear_org_id_cache",
    "clear_snapshot_cache",
    "clear_all_cache",
    # keyring
    "use_keyring",
    "keyring_key",
    "store_in_keyring",
    "get_from_keyring",
    "delete_from_keyring",
]
