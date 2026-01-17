"""Tests for configuration system."""

import os
import stat
from pathlib import Path
from unittest.mock import patch

import pytest

from vibeusage.config.settings import (
    Config,
    DisplayConfig,
    FetchConfig,
    ProviderConfig,
    CredentialsConfig,
    convert_config,
    get_config,
    load_config,
    reload_config,
    save_config,
    _load_from_toml,
    _save_to_toml,
    _apply_env_overrides,
)
from vibeusage.config.paths import (
    config_dir,
    cache_dir,
    state_dir,
    credentials_dir,
    snapshots_dir,
    org_ids_dir,
    gate_dir,
    config_file,
    ensure_directories,
    PACKAGE_NAME,
)
from vibeusage.config.credentials import (
    credential_path,
    find_provider_credential,
    write_credential,
    read_credential,
    delete_credential,
    check_credential_permissions,
    check_provider_credentials,
    get_all_credential_status,
    PROVIDER_CREDENTIAL_PATHS,
    _expand_path,
)


class TestPaths:
    """Tests for platform-specific paths."""

    def test_package_name(self):
        """PACKAGE_NAME is correctly set."""
        assert PACKAGE_NAME == "vibeusage"

    def test_config_dir_returns_path(self):
        """config_dir returns a Path object."""
        result = config_dir()
        assert isinstance(result, Path)
        assert "vibeusage" in str(result).lower()

    def test_cache_dir_returns_path(self):
        """cache_dir returns a Path object."""
        result = cache_dir()
        assert isinstance(result, Path)
        assert "vibeusage" in str(result).lower()

    def test_state_dir_returns_path(self):
        """state_dir returns a Path object."""
        result = state_dir()
        assert isinstance(result, Path)

    def test_credentials_dir_is_subdir(self):
        """credentials_dir is a subdirectory of config_dir."""
        result = credentials_dir()
        assert config_dir() in result.parents
        assert result.name == "credentials"

    def test_snapshots_dir_is_subdir(self):
        """snapshots_dir is a subdirectory of cache_dir."""
        result = snapshots_dir()
        assert cache_dir() in result.parents
        assert result.name == "snapshots"

    def test_org_ids_dir_is_subdir(self):
        """org_ids_dir is a subdirectory of cache_dir."""
        result = org_ids_dir()
        assert cache_dir() in result.parents
        assert result.name == "org-ids"

    def test_gate_dir_is_subdir(self):
        """gate_dir is a subdirectory of state_dir."""
        result = gate_dir()
        assert state_dir() in result.parents
        assert result.name == "gates"

    def test_config_file_path(self):
        """config_file returns path to config.toml."""
        result = config_file()
        assert config_dir() in result.parents
        assert result.name == "config.toml"

    def test_env_override_config_dir(self):
        """VIBEUSAGE_CONFIG_DIR environment variable overrides path."""
        custom_path = "/tmp/custom_config"
        with patch.dict(os.environ, {"VIBEUSAGE_CONFIG_DIR": custom_path}):
            result = config_dir()
            assert str(result) == custom_path

    def test_env_override_cache_dir(self):
        """VIBEUSAGE_CACHE_DIR environment variable overrides path."""
        custom_path = "/tmp/custom_cache"
        with patch.dict(os.environ, {"VIBEUSAGE_CACHE_DIR": custom_path}):
            result = cache_dir()
            assert str(result) == custom_path


class TestEnsureDirectories:
    """Tests for ensure_directories function."""

    def test_creates_directories(self, tmp_path):
        """ensure_directories creates all required directories."""
        with patch("vibeusage.config.paths.config_dir", return_value=tmp_path / "config"), patch(
            "vibeusage.config.paths.cache_dir", return_value=tmp_path / "cache"
        ), patch("vibeusage.config.paths.state_dir", return_value=tmp_path / "state"):
            ensure_directories()

            assert (tmp_path / "config").exists()
            assert (tmp_path / "cache").exists()
            assert (tmp_path / "state").exists()
            assert (tmp_path / "config" / "credentials").exists()
            assert (tmp_path / "cache" / "snapshots").exists()
            assert (tmp_path / "cache" / "org-ids").exists()
            assert (tmp_path / "state" / "gates").exists()

    def test_existing_directories_ok(self, tmp_path):
        """ensure_directories doesn't fail with existing directories."""
        with patch("vibeusage.config.paths.config_dir", return_value=tmp_path), patch(
            "vibeusage.config.paths.cache_dir", return_value=tmp_path
        ), patch("vibeusage.config.paths.state_dir", return_value=tmp_path):
            # Create directories first
            ensure_directories()
            # Call again - should not raise
            ensure_directories()


class TestDisplayConfig:
    """Tests for DisplayConfig."""

    def test_default_values(self):
        """DisplayConfig has correct defaults."""
        config = DisplayConfig()
        assert config.show_remaining is True
        assert config.pace_colors is True
        assert config.reset_format == "countdown"

    def test_custom_values(self):
        """Can create DisplayConfig with custom values."""
        config = DisplayConfig(
            show_remaining=False, pace_colors=False, reset_format="absolute"
        )
        assert config.show_remaining is False
        assert config.pace_colors is False
        assert config.reset_format == "absolute"


class TestFetchConfig:
    """Tests for FetchConfig."""

    def test_default_values(self):
        """FetchConfig has correct defaults."""
        config = FetchConfig()
        assert config.timeout == 30.0
        assert config.max_concurrent == 5
        assert config.stale_threshold_minutes == 60

    def test_custom_values(self):
        """Can create FetchConfig with custom values."""
        config = FetchConfig(timeout=60.0, max_concurrent=10, stale_threshold_minutes=30)
        assert config.timeout == 60.0
        assert config.max_concurrent == 10
        assert config.stale_threshold_minutes == 30


class TestCredentialsConfig:
    """Tests for CredentialsConfig."""

    def test_default_values(self):
        """CredentialsConfig has correct defaults."""
        config = CredentialsConfig()
        assert config.use_keyring is False
        assert config.reuse_provider_credentials is True


class TestProviderConfig:
    """Tests for ProviderConfig."""

    def test_default_values(self):
        """ProviderConfig has correct defaults."""
        config = ProviderConfig()
        assert config.auth_source == "auto"
        assert config.preferred_browser is None
        assert config.enabled is True

    def test_custom_values(self):
        """Can create ProviderConfig with custom values."""
        config = ProviderConfig(
            auth_source="oauth", preferred_browser="firefox", enabled=False
        )
        assert config.auth_source == "oauth"
        assert config.preferred_browser == "firefox"
        assert config.enabled is False


class TestConfig:
    """Tests for Config."""

    def test_default_values(self):
        """Config has correct defaults."""
        config = Config()
        assert config.enabled_providers == []
        assert isinstance(config.display, DisplayConfig)
        assert isinstance(config.fetch, FetchConfig)
        assert isinstance(config.credentials, CredentialsConfig)
        assert config.providers == {}

    def test_get_provider_config_exists(self):
        """get_provider_config returns existing config."""
        provider_cfg = ProviderConfig(auth_source="oauth", enabled=False)
        config = Config(providers={"claude": provider_cfg})

        result = config.get_provider_config("claude")
        assert result.auth_source == "oauth"
        assert result.enabled is False

    def test_get_provider_config_default(self):
        """get_provider_config returns default for missing provider."""
        config = Config()
        result = config.get_provider_config("claude")
        assert isinstance(result, ProviderConfig)
        assert result.auth_source == "auto"
        assert result.enabled is True

    def test_is_provider_enabled_empty_list(self):
        """Provider enabled when enabled_providers is empty."""
        config = Config(enabled_providers=[])
        assert config.is_provider_enabled("claude") is True

    def test_is_provider_enabled_in_list(self):
        """Provider enabled when in enabled_providers list."""
        config = Config(enabled_providers=["claude", "codex"])
        assert config.is_provider_enabled("claude") is True
        assert config.is_provider_enabled("codex") is True
        assert config.is_provider_enabled("gemini") is False

    def test_is_provider_enabled_explicitly_disabled(self):
        """Provider not enabled when explicitly disabled in config."""
        provider_cfg = ProviderConfig(enabled=False)
        config = Config(providers={"claude": provider_cfg}, enabled_providers=["claude"])
        assert config.is_provider_enabled("claude") is False

    def test_is_provider_enabled_enabled_in_config(self):
        """Provider enabled when explicitly enabled in config."""
        provider_cfg = ProviderConfig(enabled=True)
        config = Config(providers={"claude": provider_cfg}, enabled_providers=["claude"])
        assert config.is_provider_enabled("claude") is True


class TestLoadSaveTOML:
    """Tests for TOML loading and saving."""

    def test_load_from_nonexistent_file(self, tmp_path):
        """Loading nonexistent file returns empty dict."""
        result = _load_from_toml(tmp_path / "nonexistent.toml")
        assert result == {}

    def test_save_and_load_toml(self, tmp_path):
        """Saving and loading preserves data."""
        data = {
            "enabled_providers": ["claude", "codex"],
            "display": {"show_remaining": True, "pace_colors": True},
            "fetch": {"timeout": 60.0},
        }

        toml_file = tmp_path / "config.toml"
        _save_to_toml(data, toml_file)
        result = _load_from_toml(toml_file)

        assert result["enabled_providers"] == ["claude", "codex"]
        assert result["display"]["show_remaining"] is True
        assert result["fetch"]["timeout"] == 60.0


class TestConvertConfig:
    """Tests for convert_config function."""

    def test_convert_empty_dict(self):
        """Converting empty dict returns default config."""
        result = convert_config({})
        assert isinstance(result, Config)
        assert result.enabled_providers == []

    def test_convert_full_config(self):
        """Converting full dict creates proper Config."""
        data = {
            "enabled_providers": ["claude"],
            "display": {"show_remaining": False},
            "fetch": {"timeout": 45.0},
            "credentials": {"use_keyring": True},
            "providers": {
                "claude": {"auth_source": "oauth", "enabled": False},
            },
        }

        result = convert_config(data)

        assert result.enabled_providers == ["claude"]
        assert result.display.show_remaining is False
        assert result.fetch.timeout == 45.0
        assert result.credentials.use_keyring is True
        assert result.providers["claude"].auth_source == "oauth"
        assert result.providers["claude"].enabled is False

    def test_convert_preserves_defaults(self):
        """Converting partial config preserves defaults."""
        data = {"enabled_providers": ["codex"]}
        result = convert_config(data)

        assert result.enabled_providers == ["codex"]
        assert result.display.show_remaining is True  # Default
        assert result.fetch.timeout == 30.0  # Default


class TestApplyEnvOverrides:
    """Tests for _apply_env_overrides function."""

    def test_enabled_providers_override(self):
        """VIBEUSAGE_ENABLED_PROVIDERS overrides config."""
        config = Config(enabled_providers=["claude"])

        with patch.dict(os.environ, {"VIBEUSAGE_ENABLED_PROVIDERS": "codex,gemini"}):
            result = _apply_env_overrides(config)
            assert result.enabled_providers == ["codex", "gemini"]

    def test_no_color_override(self):
        """VIBEUSAGE_NO_COLOR disables pace colors."""
        config = Config(display=DisplayConfig(pace_colors=True))

        with patch.dict(os.environ, {"VIBEUSAGE_NO_COLOR": "1"}):
            result = _apply_env_overrides(config)
            assert result.display.pace_colors is False

    def test_no_env_vars_unchanged(self):
        """No environment vars leaves config unchanged."""
        config = Config(enabled_providers=["claude"])
        with patch.dict(os.environ, {}, clear=False):
            # Remove specific vars if present
            env = os.environ.copy()
            env.pop("VIBEUSAGE_ENABLED_PROVIDERS", None)
            env.pop("VIBEUSAGE_NO_COLOR", None)

            with patch.dict(os.environ, env, clear=True):
                result = _apply_env_overrides(config)
                assert result.enabled_providers == ["claude"]


class TestLoadConfig:
    """Tests for load_config function."""

    def test_load_default_config(self, tmp_path):
        """Load config with no file returns defaults."""
        with patch("vibeusage.config.settings.config_file", return_value=tmp_path / "config.toml"):
            config = load_config()
            assert isinstance(config, Config)
            assert config.enabled_providers == []

    def test_load_from_file(self, tmp_path):
        """Load config from existing file."""
        config_file = tmp_path / "config.toml"
        _save_to_toml({"enabled_providers": ["claude", "codex"]}, config_file)

        with patch("vibeusage.config.settings.config_file", return_value=config_file):
            config = load_config()
            assert config.enabled_providers == ["claude", "codex"]


class TestSaveConfig:
    """Tests for save_config function."""

    def test_save_creates_file(self, tmp_path):
        """Saving config creates file."""
        config = Config(enabled_providers=["claude"])
        config_file = tmp_path / "config.toml"

        save_config(config, path=config_file)
        assert config_file.exists()

    def test_save_roundtrip(self, tmp_path):
        """Saving and loading preserves config."""
        original = Config(
            enabled_providers=["claude", "codex"],
            display=DisplayConfig(show_remaining=False),
            fetch=FetchConfig(timeout=60.0),
        )
        config_file = tmp_path / "config.toml"

        save_config(original, path=config_file)
        loaded = _load_from_toml(config_file)

        assert loaded["enabled_providers"] == ["claude", "codex"]
        assert loaded["display"]["show_remaining"] is False
        assert loaded["fetch"]["timeout"] == 60.0


class TestConfigSingleton:
    """Tests for config singleton behavior."""

    def test_get_config_returns_singleton(self, tmp_path):
        """get_config returns same instance on subsequent calls."""
        with patch("vibeusage.config.settings.config_file", return_value=tmp_path / "config.toml"):
            # Reset singleton
            import vibeusage.config.settings as settings_module
            settings_module._config = None

            config1 = get_config()
            config2 = get_config()
            assert config1 is config2

    def test_reload_config_refreshes(self, tmp_path):
        """reload_config refreshes the singleton."""
        config_file = tmp_path / "config.toml"
        _save_to_toml({"enabled_providers": ["claude"]}, config_file)

        with patch("vibeusage.config.settings.config_file", return_value=config_file):
            import vibeusage.config.settings as settings_module
            settings_module._config = None

            config1 = get_config()
            assert config1.enabled_providers == ["claude"]

            # Modify file
            _save_to_toml({"enabled_providers": ["codex"]}, config_file)
            config2 = reload_config()
            assert config2.enabled_providers == ["codex"]


class TestCredentialPath:
    """Tests for credential_path function."""

    def test_credential_path_format(self, tmp_path):
        """credential_path returns correct format."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"):
            result = credential_path("claude", "oauth")
            assert result == tmp_path / "credentials" / "claude" / "oauth.json"


class TestExpandPath:
    """Tests for _expand_path function."""

    def test_expand_user_path(self):
        """Expands tilde to home directory."""
        with patch.dict(os.environ, {"HOME": "/home/user"}):
            # Note: This test's behavior depends on the actual system
            result = _expand_path("~/.test")
            assert str(result).startswith("/")
            assert ".test" in str(result)

    def test_expand_normal_path(self):
        """Normal path is unchanged."""
        result = _expand_path("/tmp/test.json")
        assert result == Path("/tmp/test.json")


class TestWriteCredential:
    """Tests for write_credential function."""

    def test_write_creates_file(self, tmp_path):
        """Writing credential creates file."""
        cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
        content = b'{"token": "test"}'

        write_credential(cred_path, content)

        assert cred_path.exists()
        assert cred_path.read_bytes() == content

    def test_write_creates_parent_dirs(self, tmp_path):
        """Writing creates parent directories."""
        cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
        content = b'{"token": "test"}'

        write_credential(cred_path, content)

        assert cred_path.parent.exists()

    def test_write_sets_permissions(self, tmp_path):
        """Writing sets 0o600 permissions."""
        cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
        content = b'{"token": "test"}'

        write_credential(cred_path, content)

        mode = cred_path.stat().st_mode
        # Check owner read/write
        assert bool(mode & stat.S_IRUSR)
        assert bool(mode & stat.S_IWUSR)
        # Check no group/other permissions
        assert not bool(mode & stat.S_IRGRP)
        assert not bool(mode & stat.S_IWGRP)
        assert not bool(mode & stat.S_IROTH)
        assert not bool(mode & stat.S_IWOTH)

    def test_write_atomic(self, tmp_path):
        """Writing is atomic (uses temp file)."""
        cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
        content = b'{"token": "test"}'

        write_credential(cred_path, content)

        # No .tmp file should remain
        assert not cred_path.with_suffix(".tmp").exists()


class TestReadCredential:
    """Tests for read_credential function."""

    def test_read_existing_file(self, tmp_path):
        """Reading existing file returns content."""
        cred_path = tmp_path / "oauth.json"
        content = b'{"token": "test"}'
        cred_path.write_bytes(content)
        cred_path.chmod(0o600)

        result = read_credential(cred_path)
        assert result == content

    def test_read_nonexistent_file(self, tmp_path):
        """Reading nonexistent file returns None."""
        result = read_credential(tmp_path / "nonexistent.json")
        assert result is None

    def test_read_insecure_permissions(self, tmp_path):
        """Reading file with insecure permissions returns None."""
        cred_path = tmp_path / "oauth.json"
        content = b'{"token": "test"}'
        cred_path.write_bytes(content)
        cred_path.chmod(0o644)  # World-readable

        result = read_credential(cred_path)
        assert result is None


class TestDeleteCredential:
    """Tests for delete_credential function."""

    def test_delete_existing_file(self, tmp_path):
        """Deleting existing file returns True."""
        cred_path = tmp_path / "oauth.json"
        cred_path.write_bytes(b"test")

        result = delete_credential(cred_path)
        assert result is True
        assert not cred_path.exists()

    def test_delete_nonexistent_file(self, tmp_path):
        """Deleting nonexistent file returns False."""
        result = delete_credential(tmp_path / "nonexistent.json")
        assert result is False


class TestCheckCredentialPermissions:
    """Tests for check_credential_permissions function."""

    def test_nonexistent_file_is_secure(self, tmp_path):
        """Nonexistent file is considered secure."""
        assert check_credential_permissions(tmp_path / "nonexistent.json") is True

    def test_secure_permissions_pass(self, tmp_path):
        """File with 0o600 permissions passes."""
        cred_path = tmp_path / "oauth.json"
        cred_path.write_bytes(b"test")
        cred_path.chmod(0o600)

        assert check_credential_permissions(cred_path) is True

    def test_group_readable_fails(self, tmp_path):
        """File with group read permission fails."""
        cred_path = tmp_path / "oauth.json"
        cred_path.write_bytes(b"test")
        cred_path.chmod(0o640)

        assert check_credential_permissions(cred_path) is False

    def test_world_readable_fails(self, tmp_path):
        """File with world read permission fails."""
        cred_path = tmp_path / "oauth.json"
        cred_path.write_bytes(b"test")
        cred_path.chmod(0o644)

        assert check_credential_permissions(cred_path) is False

    def test_strict_permissions_pass(self, tmp_path):
        """File with stricter permissions (0o400) passes."""
        cred_path = tmp_path / "oauth.json"
        cred_path.write_bytes(b"test")
        cred_path.chmod(0o400)

        assert check_credential_permissions(cred_path) is True


class TestFindProviderCredential:
    """Tests for find_provider_credential function."""

    def test_finds_vibeusage_credential(self, tmp_path):
        """Finds credential in vibeusage storage."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.settings.get_config"
        ) as mock_get_config:
            mock_get_config.return_value = Config()

            # Create credential file
            cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
            cred_path.parent.mkdir(parents=True, exist_ok=True)
            cred_path.write_bytes(b'{"token": "test"}')

            found, source, path = find_provider_credential("claude")
            assert found is True
            assert source == "vibeusage"
            assert path == cred_path

    def test_finds_provider_cli_credential(self, tmp_path):
        """Finds credential in provider CLI location."""
        provider_path = tmp_path / ".claude" / ".credentials.json"
        provider_path.parent.mkdir(parents=True, exist_ok=True)
        provider_path.write_bytes(b'{"token": "test"}')

        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.credentials._expand_path", return_value=provider_path
        ), patch("vibeusage.config.settings.get_config") as mock_get_config:
            mock_get_config.return_value = Config(credentials=CredentialsConfig(reuse_provider_credentials=True))

            found, source, path = find_provider_credential("claude")
            assert found is True
            assert source == "provider_cli"
            assert path == provider_path

    def test_finds_env_var_credential(self):
        """Finds credential in environment variable."""
        with patch.dict(os.environ, {"ANTHROPIC_API_KEY": "sk-test"}), patch(
            "vibeusage.config.credentials.credentials_dir", return_value=Path("/tmp/credentials")
        ), patch("vibeusage.config.settings.get_config") as mock_get_config:
            mock_get_config.return_value = Config()

            found, source, path = find_provider_credential("claude")
            assert found is True
            assert source == "env"
            assert path is None

    def test_no_credential_found(self, tmp_path):
        """Returns not found when no credential exists."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.settings.get_config"
        ) as mock_get_config:
            mock_get_config.return_value = Config()

            found, source, path = find_provider_credential("claude")
            assert found is False
            assert source is None
            assert path is None

    def test_respects_reuse_provider_credentials(self, tmp_path):
        """Respects reuse_provider_credentials setting."""
        provider_path = tmp_path / ".claude" / ".credentials.json"
        provider_path.parent.mkdir(parents=True, exist_ok=True)
        provider_path.write_bytes(b'{"token": "test"}')

        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.credentials._expand_path", return_value=provider_path
        ), patch("vibeusage.config.settings.get_config") as mock_get_config:
            # Disable reuse
            mock_get_config.return_value = Config(credentials=CredentialsConfig(reuse_provider_credentials=False))

            found, source, path = find_provider_credential("claude")
            # Should not find provider CLI credential when reuse is disabled
            assert found is False


class TestCheckProviderCredentials:
    """Tests for check_provider_credentials function."""

    def test_has_credentials(self, tmp_path):
        """Returns True when credentials exist."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.settings.get_config"
        ) as mock_get_config:
            mock_get_config.return_value = Config()

            cred_path = tmp_path / "credentials" / "claude" / "oauth.json"
            cred_path.parent.mkdir(parents=True, exist_ok=True)
            cred_path.write_bytes(b'{"token": "test"}')

            has_creds, source = check_provider_credentials("claude")
            assert has_creds is True
            assert source == "vibeusage"

    def test_no_credentials(self, tmp_path):
        """Returns False when no credentials exist."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.settings.get_config"
        ) as mock_get_config:
            mock_get_config.return_value = Config()

            has_creds, source = check_provider_credentials("claude")
            assert has_creds is False
            assert source is None


class TestGetAllCredentialStatus:
    """Tests for get_all_credential_status function."""

    def test_returns_status_for_all_providers(self, tmp_path):
        """Returns status for all known providers."""
        with patch("vibeusage.config.credentials.credentials_dir", return_value=tmp_path / "credentials"), patch(
            "vibeusage.config.settings.get_config"
        ) as mock_get_config:
            mock_get_config.return_value = Config()

            status = get_all_credential_status()

            assert "claude" in status
            assert "codex" in status
            assert "gemini" in status
            assert "copilot" in status
            assert "cursor" in status

            for provider_status in status.values():
                assert "has_credentials" in provider_status
                assert "source" in provider_status
