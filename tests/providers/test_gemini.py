"""Tests for Gemini provider."""
from __future__ import annotations

import json
from datetime import datetime
from datetime import timedelta
from datetime import timezone
from pathlib import Path
from unittest.mock import patch

import pytest

from vibeusage.models import PeriodType
from vibeusage.providers.gemini import GeminiProvider
from vibeusage.providers.gemini.api_key import GeminiApiKeyStrategy
from vibeusage.providers.gemini.oauth import GeminiOAuthStrategy


class TestGeminiProvider:
    """Tests for GeminiProvider."""

    def test_metadata(self):
        """GeminiProvider has correct metadata."""
        assert GeminiProvider.metadata.id == "gemini"
        assert GeminiProvider.metadata.name == "Gemini"
        assert "Google" in GeminiProvider.metadata.description
        assert GeminiProvider.metadata.homepage == "https://gemini.google.com"
        assert GeminiProvider.metadata.status_url is None
        assert (
            GeminiProvider.metadata.dashboard_url
            == "https://aistudio.google.com/app/usage"
        )

    def test_id_property(self):
        """id property returns correct value."""
        provider = GeminiProvider()
        assert provider.id == "gemini"

    def test_name_property(self):
        """name property returns correct value."""
        provider = GeminiProvider()
        assert provider.name == "Gemini"

    def test_fetch_strategies_returns_list(self):
        """fetch_strategies returns list of strategies."""
        provider = GeminiProvider()
        strategies = provider.fetch_strategies()

        assert isinstance(strategies, list)
        assert len(strategies) == 2

    def test_fetch_strategy_order(self):
        """Strategies are in correct priority order."""
        provider = GeminiProvider()
        strategies = provider.fetch_strategies()

        # Should be OAuth, API Key in that order
        assert isinstance(strategies[0], GeminiOAuthStrategy)
        assert isinstance(strategies[1], GeminiApiKeyStrategy)

    def test_fetch_status(self):
        """fetch_status returns async operation."""
        provider = GeminiProvider()

        import inspect

        assert inspect.iscoroutinefunction(provider.fetch_status)


class TestGeminiOAuthStrategy:
    """Tests for GeminiOAuthStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = GeminiOAuthStrategy()
        assert strategy.name == "oauth"

    def test_credential_paths(self):
        """Strategy has correct credential paths."""
        strategy = GeminiOAuthStrategy()
        assert len(strategy.CREDENTIAL_PATHS) == 2

    def test_is_available_returns_false_when_no_credentials(self):
        """is_available returns False when no credentials exist."""
        strategy = GeminiOAuthStrategy()

        with patch.object(strategy, "CREDENTIAL_PATHS", [Path("/nonexistent/path")]):
            result = strategy.is_available()
            assert result is False

    def test_is_available_returns_true_when_credentials_exist(self):
        """is_available returns True when credentials exist."""
        strategy = GeminiOAuthStrategy()

        with patch("pathlib.Path.exists", return_value=True):
            result = strategy.is_available()
            assert result is True

    @pytest.mark.asyncio
    async def test_fetch_fails_without_credentials(self):
        """fetch fails when no credentials are found."""
        strategy = GeminiOAuthStrategy()

        with patch.object(strategy, "_load_credentials", return_value=None):
            result = await strategy.fetch()

            assert result.success is False
            assert "No OAuth credentials found" in result.error

    def test_load_credentials_from_vibeusage_format(self):
        """_load_credentials handles vibeusage format."""
        strategy = GeminiOAuthStrategy()
        creds_data = {
            "access_token": "test_token",
            "refresh_token": "test_refresh",
            "expires_at": "2026-01-01T00:00:00+00:00",
        }

        with patch(
            "vibeusage.providers.gemini.oauth.read_credential",
            return_value=json.dumps(creds_data).encode(),
        ):
            creds = strategy._load_credentials()

            assert creds is not None
            assert creds["access_token"] == "test_token"
            assert creds["refresh_token"] == "test_refresh"

    def test_load_credentials_from_gemini_cli_format(self):
        """_load_credentials handles Gemini CLI format with 'installed' key."""
        strategy = GeminiOAuthStrategy()
        cli_data = {
            "installed": {
                "token": "cli_token",
                "refresh_token": "cli_refresh",
                "expiry_date": 1735689600000,  # Millisecond timestamp
            }
        }

        with patch(
            "vibeusage.providers.gemini.oauth.read_credential",
            return_value=json.dumps(cli_data).encode(),
        ):
            creds = strategy._load_credentials()

            assert creds is not None
            assert creds["access_token"] == "cli_token"
            assert creds["refresh_token"] == "cli_refresh"
            assert "expires_at" in creds

    def test_load_credentials_from_token_format(self):
        """_load_credentials handles format with 'token' key (not in 'installed')."""
        strategy = GeminiOAuthStrategy()
        token_data = {
            "token": "bare_token",
            "refresh_token": "bare_refresh",
            "expiry_date": "2026-01-01T00:00:00+00:00",
        }

        with patch(
            "vibeusage.providers.gemini.oauth.read_credential",
            return_value=json.dumps(token_data).encode(),
        ):
            creds = strategy._load_credentials()

            assert creds is not None
            assert creds["access_token"] == "bare_token"
            assert creds["refresh_token"] == "bare_refresh"

    def test_convert_gemini_cli_format(self):
        """_convert_gemini_cli_format converts CLI format to standard."""
        strategy = GeminiOAuthStrategy()

        cli_data = {
            "token": "test_token",
            "refresh_token": "test_refresh",
            "expiry_date": 1735689600000,  # 2025-01-01 00:00:00 UTC in milliseconds
        }

        result = strategy._convert_gemini_cli_format(cli_data)

        assert result["access_token"] == "test_token"
        assert result["refresh_token"] == "test_refresh"
        assert "2025-01-01" in result["expires_at"]

    def test_convert_gemini_cli_format_with_iso_expiry(self):
        """_convert_gemini_cli_format handles ISO format expiry."""
        strategy = GeminiOAuthStrategy()

        cli_data = {
            "token": "test_token",
            "expiry_date": "2026-01-01T00:00:00+00:00",
        }

        result = strategy._convert_gemini_cli_format(cli_data)

        assert result["access_token"] == "test_token"
        assert result["expires_at"] == "2026-01-01T00:00:00+00:00"

    def test_needs_refresh_returns_false_without_expiry(self):
        """_needs_refresh returns False when no expiry set."""
        strategy = GeminiOAuthStrategy()
        assert strategy._needs_refresh({}) is False

    def test_needs_refresh_returns_true_for_expired_token(self):
        """_needs_refresh returns True when token is expired."""
        strategy = GeminiOAuthStrategy()

        # Token expired in the past
        past_expiry = (datetime.now(timezone.utc) - timedelta(hours=1)).isoformat()
        assert strategy._needs_refresh({"expires_at": past_expiry}) is True

    def test_needs_refresh_returns_true_for_near_expiry(self):
        """_needs_refresh returns True when token expires soon."""
        strategy = GeminiOAuthStrategy()

        # Token expires within threshold
        near_expiry = (datetime.now(timezone.utc) + timedelta(minutes=2)).isoformat()
        assert strategy._needs_refresh({"expires_at": near_expiry}) is True

    def test_needs_refresh_returns_false_for_valid_token(self):
        """_needs_refresh returns False when token is valid."""
        strategy = GeminiOAuthStrategy()

        # Token expires far in future
        future_expiry = (datetime.now(timezone.utc) + timedelta(days=1)).isoformat()
        assert strategy._needs_refresh({"expires_at": future_expiry}) is False

    @pytest.mark.asyncio
    async def test_parse_usage_response_with_quota_buckets(self):
        """_parse_usage_response handles quota_buckets format."""
        strategy = GeminiOAuthStrategy()

        quota_data = {
            "quota_buckets": [
                {
                    "model_id": "models/gemini-1.5-flash",
                    "remaining_fraction": 0.75,
                    "reset_time": "2026-01-18T00:00:00Z",
                },
                {
                    "model_id": "models/gemini-1.5-pro",
                    "remaining_fraction": 0.50,
                    "reset_time": "2026-01-18T00:00:00Z",
                },
            ]
        }

        user_data = {
            "user_tier": "paid",
        }

        snapshot = strategy._parse_usage_response(quota_data, user_data)

        assert snapshot.provider == "gemini"
        assert snapshot.source == "oauth"
        assert len(snapshot.periods) == 2

        # First period (gemini-1.5-flash)
        period1 = snapshot.periods[0]
        assert period1.utilization == 25  # (1 - 0.75) * 100
        assert period1.period_type == PeriodType.DAILY
        assert "Gemini 1.5 Flash" in period1.name or "flash" in period1.name.lower()

        # Second period (gemini-1.5-pro)
        period2 = snapshot.periods[1]
        assert period2.utilization == 50  # (1 - 0.50) * 100

        # Identity
        assert snapshot.identity is not None
        assert snapshot.identity.plan == "paid"

    @pytest.mark.asyncio
    async def test_parse_usage_response_without_quota_buckets(self):
        """_parse_usage_response creates default period when no quota buckets."""
        strategy = GeminiOAuthStrategy()

        snapshot = strategy._parse_usage_response({}, None)

        assert snapshot.provider == "gemini"
        assert len(snapshot.periods) == 1
        assert snapshot.periods[0].utilization == 0
        assert snapshot.periods[0].period_type == PeriodType.DAILY

    @pytest.mark.asyncio
    async def test_parse_usage_response_handles_unix_timestamp(self):
        """_parse_usage_response handles Unix timestamp format."""
        strategy = GeminiOAuthStrategy()

        quota_data = {
            "quota_buckets": [
                {
                    "model_id": "models/gemini-1.5-flash",
                    "remaining_fraction": 0.75,
                    "reset_time": "1737110400",  # Unix timestamp
                },
            ]
        }

        snapshot = strategy._parse_usage_response(quota_data, None)

        assert snapshot.periods[0].resets_at is not None
        assert snapshot.periods[0].resets_at.year == 2025

    @pytest.mark.asyncio
    async def test_parse_usage_response_handles_model_id_without_prefix(self):
        """_parse_usage_response handles model_id without 'models/' prefix."""
        strategy = GeminiOAuthStrategy()

        quota_data = {
            "quota_buckets": [
                {
                    "model_id": "gemini-1.5-flash",
                    "remaining_fraction": 0.75,
                },
            ]
        }

        snapshot = strategy._parse_usage_response(quota_data, None)

        assert len(snapshot.periods) == 1
        assert snapshot.periods[0].model == "gemini-1.5-flash"


class TestGeminiApiKeyStrategy:
    """Tests for GeminiApiKeyStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = GeminiApiKeyStrategy()
        assert strategy.name == "api_key"

    def test_env_var(self):
        """Strategy has correct environment variable."""
        strategy = GeminiApiKeyStrategy()
        assert strategy.ENV_VAR == "GEMINI_API_KEY"

    def test_credential_paths(self):
        """Strategy has correct credential paths."""
        strategy = GeminiApiKeyStrategy()
        assert len(strategy.CREDENTIAL_PATHS) == 2

    def test_is_available_returns_false_when_no_credentials(self):
        """is_available returns False when no credentials exist."""
        strategy = GeminiApiKeyStrategy()

        with patch.object(strategy, "CREDENTIAL_PATHS", [Path("/nonexistent/path")]):
            with patch.dict("os.environ", {}, clear=False):
                # Remove GEMINI_API_KEY if present
                env = os.environ.pop("GEMINI_API_KEY", None)
                try:
                    result = strategy.is_available()
                    assert result is False
                finally:
                    if env:
                        os.environ["GEMINI_API_KEY"] = env

    def test_is_available_returns_true_from_env_var(self):
        """is_available returns True when API key in environment."""
        strategy = GeminiApiKeyStrategy()

        with patch.dict("os.environ", {"GEMINI_API_KEY": "test_key"}):
            result = strategy.is_available()
            assert result is True


class TestGeminiProviderIntegration:
    """Integration tests for Gemini provider with registry."""

    def test_gemini_registered(self):
        """GeminiProvider is registered in the registry."""
        from vibeusage.providers import get_provider

        provider_cls = get_provider("gemini")
        assert provider_cls is not None
        assert provider_cls == GeminiProvider

    def test_create_gemini_provider(self):
        """Can create GeminiProvider instance via registry."""
        from vibeusage.providers import create_provider

        provider = create_provider("gemini")
        assert isinstance(provider, GeminiProvider)

    def test_list_includes_gemini(self):
        """list_provider_ids includes gemini."""
        from vibeusage.providers import list_provider_ids

        ids = list_provider_ids()
        assert "gemini" in ids


# Import os for environment variable tests
import os
