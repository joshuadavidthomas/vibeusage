"""Configuration management for vibeusage."""

from vibeusage.config.cache import (
    cache_org_id,
    cache_snapshot,
    clear_all_cache,
    clear_org_id_cache,
    clear_snapshot_cache,
    is_snapshot_fresh,
    load_cached_org_id,
    load_cached_snapshot,
    snapshot_path,
)
from vibeusage.config.credentials import (
    check_credential_permissions,
    check_provider_credentials,
    credential_path,
    delete_credential,
    find_provider_credential,
    read_credential,
    write_credential,
)
from vibeusage.config.keyring import (
    delete_from_keyring,
    get_from_keyring,
    keyring_key,
    store_in_keyring,
    use_keyring,
)
from vibeusage.config.paths import (
    cache_dir,
    config_dir,
    config_file,
    credentials_dir,
    ensure_directories,
    gate_dir,
    org_ids_dir,
    snapshots_dir,
    state_dir,
)
from vibeusage.config.settings import (
    Config,
    DisplayConfig,
    FetchConfig,
    ProviderConfig,
    get_config,
    load_config,
    reload_config,
    save_config,
)

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
