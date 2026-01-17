"""Tests for Claude OAuth strategy."""

from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime
from datetime import timedelta
from pathlib import Path
from unittest.mock import AsyncMock
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest

from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.providers.claude import ClaudeProvider
from vibeusage.providers.claude.oauth import ClaudeOAuthStrategy
from vibeusage.strategies.base import FetchResult


class TestClaudeOAuthStrategy:
    """Tests for ClaudeOAuthStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = ClaudeOAuthStrategy()
        assert strategy.name == "oauth"

    def test_credential_paths(self):
        """Credential paths are defined correctly."""
        strategy = ClaudeOAuthStrategy()
        assert len(strategy.CREDENTIAL_PATHS) >= 2
        assert "claude" in str(strategy.CREDENTIAL_PATHS[0]).lower()

    def test_is_available_returns_false_when_no_credentials(self):
        """is_available returns False when no credentials exist."""
        strategy = ClaudeOAuthStrategy()

        with patch.object(
            strategy, "CREDENTIAL_PATHS", [Path("/nonexistent/path.json")]
        ):
            result = strategy.is_available()
            assert result is False

    def test_is_available_returns_true_when_credentials_exist(self):
        """is_available returns True when credentials exist."""
        strategy = ClaudeOAuthStrategy()

        with patch("vibeusage.providers.claude.oauth.Path.exists") as mock_exists:
            mock_exists.return_value = True
            result = strategy.is_available()
            assert result is True


class TestClaudeOAuthLoadCredentials:
    """Tests for _load_credentials method."""

    def test_load_credentials_returns_none_when_no_files_exist(self):
        """_load_credentials returns None when no credential files exist."""
        strategy = ClaudeOAuthStrategy()

        with patch("vibeusage.providers.claude.oauth.read_credential", return_value=None):
            result = strategy._load_credentials()
            assert result is None

    def test_load_credentials_reads_vibeusage_format(self):
        """_load_credentials reads vibeusage credential format."""
        strategy = ClaudeOAuthStrategy()

        creds_data = {
            "access_token": "test_access_token",
            "refresh_token": "test_refresh_token",
            "token_type": "Bearer",
            "expires_at": (datetime.now(UTC) + timedelta(hours=1)).isoformat(),
        }

        with patch(
            "vibeusage.providers.claude.oauth.read_credential",
            return_value=json.dumps(creds_data).encode(),
        ):
            result = strategy._load_credentials()

        assert result is not None
        assert result["access_token"] == "test_access_token"
        assert result["refresh_token"] == "test_refresh_token"

    def test_load_credentials_handles_claude_cli_format(self):
        """_load_credentials converts Claude CLI format to standard format."""
        strategy = ClaudeOAuthStrategy()

        # Claude CLI format with camelCase keys and nested claudeAiOauth
        creds_data = {
            "claudeAiOauth": {
                "accessToken": "cli_access_token",
                "refreshToken": "cli_refresh_token",
                "expiresAt": int((datetime.now(UTC) + timedelta(hours=1)).timestamp() * 1000),
            }
        }

        with patch(
            "vibeusage.providers.claude.oauth.read_credential",
            return_value=json.dumps(creds_data).encode(),
        ):
            result = strategy._load_credentials()

        assert result is not None
        assert result["access_token"] == "cli_access_token"
        assert result["refresh_token"] == "cli_refresh_token"
        assert "expires_at" in result

    def test_load_credentials_skips_invalid_json(self):
        """_load_credentials skips files with invalid JSON."""
        strategy = ClaudeOAuthStrategy()

        with patch(
            "vibeusage.providers.claude.oauth.read_credential",
            return_value=b"invalid json {{{",
        ):
            result = strategy._load_credentials()

        assert result is None


class TestClaudeOAuthConvertCliFormat:
    """Tests for _convert_claude_cli_format method."""

    def test_convert_claude_cli_format_camelcase_to_snakecase(self):
        """_convert_claude_cli_format converts camelCase to snake_case."""
        strategy = ClaudeOAuthStrategy()

        input_data = {
            "accessToken": "token123",
            "refreshToken": "refresh123",
            "tokenType": "Bearer",
            "expiresAt": 1234567890000,
        }

        result = strategy._convert_claude_cli_format(input_data)

        assert "access_token" in result
        assert "refresh_token" in result
        assert "token_type" in result
        assert "expires_at" in result
        assert result["access_token"] == "token123"
        assert result["refresh_token"] == "refresh123"
        assert result["token_type"] == "Bearer"

    def test_convert_claude_cli_format_timestamp_to_iso(self):
        """_convert_claude_cli_format converts millisecond timestamp to ISO string."""
        strategy = ClaudeOAuthStrategy()

        # 2025-01-15 12:00:00 UTC in milliseconds
        timestamp = 1736949600000
        input_data = {"expiresAt": timestamp}

        result = strategy._convert_claude_cli_format(input_data)

        assert "expires_at" in result
        # Should be a valid ISO format string
        assert isinstance(result["expires_at"], str)
        # Can parse it back to datetime
        parsed = datetime.fromisoformat(result["expires_at"])
        assert parsed.tzinfo is not None

    def test_convert_claude_cli_format_handles_non_timestamp_expires_at(self):
        """_convert_claude_cli_format keeps non-timestamp expires_at as-is."""
        strategy = ClaudeOAuthStrategy()

        input_data = {"expiresAt": "2025-01-15T12:00:00+00:00"}

        result = strategy._convert_claude_cli_format(input_data)

        assert result["expires_at"] == "2025-01-15T12:00:00+00:00"

    def test_convert_claude_cli_format_preserves_unknown_fields(self):
        """_convert_claude_cli_format preserves unknown fields with snake_case."""
        strategy = ClaudeOAuthStrategy()

        input_data = {"scope": "usage.read", "myCustomField": "value"}

        result = strategy._convert_claude_cli_format(input_data)

        assert result["scope"] == "usage.read"
        assert result["my_custom_field"] == "value"


class TestClaudeOAuthNeedsRefresh:
    """Tests for _needs_refresh method."""

    def test_needs_refresh_returns_false_when_no_expires_at(self):
        """_needs_refresh returns False when expires_at is missing."""
        strategy = ClaudeOAuthStrategy()
        credentials = {"access_token": "token"}

        result = strategy._needs_refresh(credentials)

        assert result is False

    def test_needs_refresh_returns_true_when_expired(self):
        """_needs_refresh returns True when token is expired."""
        strategy = ClaudeOAuthStrategy()
        # Token expired 1 hour ago
        past_time = datetime.now(UTC) - timedelta(hours=1)
        credentials = {"access_token": "token", "expires_at": past_time.isoformat()}

        result = strategy._needs_refresh(credentials)

        assert result is True

    def test_needs_refresh_returns_true_when_expiring_now(self):
        """_needs_refresh returns True when token expires at current time."""
        strategy = ClaudeOAuthStrategy()
        # Token expires now
        now_time = datetime.now(UTC)
        credentials = {"access_token": "token", "expires_at": now_time.isoformat()}

        result = strategy._needs_refresh(credentials)

        assert result is True

    def test_needs_refresh_returns_false_when_valid(self):
        """_needs_refresh returns False when token is still valid."""
        strategy = ClaudeOAuthStrategy()
        # Token expires in 1 hour
        future_time = datetime.now(UTC) + timedelta(hours=1)
        credentials = {"access_token": "token", "expires_at": future_time.isoformat()}

        result = strategy._needs_refresh(credentials)

        assert result is False

    def test_needs_refresh_handles_invalid_iso_format(self):
        """_needs_refresh handles invalid expires_at format gracefully."""
        strategy = ClaudeOAuthStrategy()
        credentials = {"access_token": "token", "expires_at": "invalid-date"}

        result = strategy._needs_refresh(credentials)

        assert result is True


class TestClaudeOAuthRefreshToken:
    """Tests for _refresh_token method."""

    @pytest.mark.asyncio
    async def test_refresh_token_returns_none_without_refresh_token(self):
        """_refresh_token returns None when refresh_token is missing."""
        strategy = ClaudeOAuthStrategy()
        credentials = {"access_token": "token", "expires_at": "2025-01-01T00:00:00Z"}

        result = await strategy._refresh_token(credentials)

        assert result is None

    @pytest.mark.asyncio
    async def test_refresh_token_successful(self, mock_httpx_client, mock_response):
        """_refresh_token successfully refreshes access token."""
        strategy = ClaudeOAuthStrategy()
        credentials = {
            "access_token": "old_token",
            "refresh_token": "refresh_token_value",
        }

        # Mock successful token response
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "access_token": "new_access_token",
            "refresh_token": "new_refresh_token",
            "token_type": "Bearer",
            "expires_in": timedelta(seconds=3600),
        }

        mock_httpx_client.post.return_value = mock_response

        with patch(
            "vibeusage.providers.claude.oauth.get_http_client",
            return_value=mock_httpx_client,
        ):
            with patch.object(strategy, "_save_credentials"):
                result = await strategy._refresh_token(credentials)

        assert result is not None
        assert result["access_token"] == "new_access_token"
        assert "expires_at" in result

    @pytest.mark.asyncio
    async def test_refresh_token_handles_401_error(self, mock_httpx_client, mock_response):
        """_refresh_token returns None on 401 error."""
        strategy = ClaudeOAuthStrategy()
        credentials = {
            "access_token": "old_token",
            "refresh_token": "invalid_refresh_token",
        }

        mock_response.status_code = 401

        mock_httpx_client.post.return_value = mock_response

        with patch(
            "vibeusage.providers.claude.oauth.get_http_client",
            return_value=mock_httpx_client,
        ):
            result = await strategy._refresh_token(credentials)

        assert result is None

    @pytest.mark.asyncio
    async def test_refresh_token_handles_invalid_json(
        self, mock_httpx_client, mock_response
    ):
        """_refresh_token returns None when response is invalid JSON."""
        strategy = ClaudeOAuthStrategy()
        credentials = {
            "access_token": "old_token",
            "refresh_token": "refresh_token_value",
        }

        mock_response.status_code = 200
        mock_response.json.side_effect = json.JSONDecodeError("Invalid JSON", "", 0)

        mock_httpx_client.post.return_value = mock_response

        with patch(
            "vibeusage.providers.claude.oauth.get_http_client",
            return_value=mock_httpx_client,
        ):
            result = await strategy._refresh_token(credentials)

        assert result is None


class TestClaudeOAuthSaveCredentials:
    """Tests for _save_credentials method."""

    def test_save_credentials_writes_to_file(self, temp_config_dir):
        """_save_credentials writes credentials to the first credential path."""
        strategy = ClaudeOAuthStrategy()
        credentials = {
            "access_token": "test_token",
            "refresh_token": "test_refresh",
            "expires_at": "2025-01-15T12:00:00+00:00",
        }

        with patch("vibeusage.config.paths.config_dir", return_value=temp_config_dir):
            with patch("vibeusage.providers.claude.oauth.write_credential") as mock_write:
                strategy._save_credentials(credentials)

                # Verify write_credential was called
                mock_write.assert_called_once()
                args = mock_write.call_args
                assert "claude" in str(args[0][0]).lower()

                # Verify the content is JSON
                import json

                content = args[0][1]
                decoded = json.loads(content)
                assert decoded["access_token"] == "test_token"


class TestClaudeOAuthParseUsageResponse:
    """Tests for _parse_usage_response method."""

    def test_parse_usage_response_full_response(self, utc_now):
        """_parse_usage_response parses complete API response."""
        strategy = ClaudeOAuthStrategy()

        # Mock API response matching actual format
        data = {
            "five_hour": {
                "utilization": 65.5,
                "resets_at": "2025-01-15T15:00:00+00:00",
            },
            "seven_day": {
                "utilization": 27.0,
                "resets_at": "2025-01-22T18:00:00+00:00",
            },
            "seven_day_sonnet": {
                "utilization": 15.0,
                "resets_at": "2025-01-22T18:00:00+00:00",
            },
            "seven_day_opus": {
                "utilization": 45.0,
                "resets_at": "2025-01-22T18:00:00+00:00",
            },
            "monthly": {
                "utilization": 80.0,
                "resets_at": "2025-02-01T00:00:00+00:00",
            },
            "extra_usage": {
                "is_enabled": True,
                "used_credits": 2.50,
                "monthly_limit": 15.00,
            },
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        assert result.provider == "claude"
        assert result.source == "oauth"
        assert len(result.periods) == 5  # five_hour, seven_day, sonnet, opus, monthly

        # Check session period
        session_period = next(p for p in result.periods if p.period_type == PeriodType.SESSION)
        assert session_period.name == "Session (5h)"
        assert session_period.utilization == 65

        # Check weekly period
        weekly_period = next(p for p in result.periods if p.period_type == PeriodType.WEEKLY and p.model is None)
        assert weekly_period.name == "All Models"
        assert weekly_period.utilization == 27

        # Check model-specific periods
        sonnet_period = next(p for p in result.periods if p.model == "sonnet")
        assert sonnet_period.name == "Sonnet"
        assert sonnet_period.utilization == 15

        opus_period = next(p for p in result.periods if p.model == "opus")
        assert opus_period.name == "Opus"
        assert opus_period.utilization == 45

        # Check overage
        assert result.overage is not None
        assert result.overage.is_enabled is True
        assert result.overage.used == 2.50
        assert result.overage.limit == 15.00

    def test_parse_usage_response_minimal_response(self):
        """_parse_usage_response handles minimal response."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "five_hour": {"utilization": 50.0, "resets_at": "2025-01-15T15:00:00+00:00"},
            "seven_day": {"utilization": 30.0, "resets_at": "2025-01-22T18:00:00+00:00"},
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        assert len(result.periods) == 2
        assert result.overage is None
        assert result.identity is None
        assert result.status is None

    def test_parse_usage_response_handles_null_periods(self):
        """_parse_usage_response skips null period data."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "five_hour": None,
            "seven_day": {"utilization": 30.0, "resets_at": "2025-01-22T18:00:00+00:00"},
            "monthly": None,
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        assert len(result.periods) == 1  # Only seven_day should be parsed
        assert result.periods[0].name == "All Models"

    def test_parse_usage_response_handles_missing_utilization(self):
        """_parse_usage_response skips periods without utilization."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "five_hour": {"resets_at": "2025-01-15T15:00:00+00:00"},  # No utilization
            "seven_day": {"utilization": 30.0, "resets_at": "2025-01-22T18:00:00+00:00"},
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        assert len(result.periods) == 1  # Only seven_day
        assert result.periods[0].utilization == 30

    def test_parse_usage_response_handles_invalid_reset_time(self):
        """_parse_usage_response handles invalid reset time gracefully."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "five_hour": {
                "utilization": 50.0,
                "resets_at": "invalid-date-time",
            },
            "seven_day": {"utilization": 30.0, "resets_at": "2025-01-22T18:00:00+00:00"},
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        # five_hour period should be created but with None resets_at
        five_hour = next((p for p in result.periods if p.period_type == PeriodType.SESSION), None)
        assert five_hour is not None
        assert five_hour.resets_at is None

    def test_parse_usage_response_disabled_overage(self):
        """_parse_usage_response handles disabled overage."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "five_hour": {"utilization": 50.0, "resets_at": "2025-01-15T15:00:00+00:00"},
            "extra_usage": {"is_enabled": False},
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        assert result.overage is None

    def test_parse_usage_response_returns_snapshot_for_empty_data(self):
        """_parse_usage_response returns snapshot with no periods for empty response."""
        strategy = ClaudeOAuthStrategy()

        result = strategy._parse_usage_response({})

        # Implementation returns a snapshot even with no periods (not None)
        assert isinstance(result, UsageSnapshot)
        assert len(result.periods) == 0
        assert result.provider == "claude"

    def test_parse_usage_response_handles_haiku_model(self):
        """_parse_usage_response parses Haiku model usage."""
        strategy = ClaudeOAuthStrategy()

        data = {
            "seven_day_haiku": {
                "utilization": 5.0,
                "resets_at": "2025-01-22T18:00:00+00:00",
            },
        }

        result = strategy._parse_usage_response(data)

        assert isinstance(result, UsageSnapshot)
        haiku_period = next((p for p in result.periods if p.model == "haiku"), None)
        assert haiku_period is not None
        assert haiku_period.name == "Haiku"
        assert haiku_period.utilization == 5


class TestClaudeOAuthFetchIntegration:
    """Integration tests for the full fetch flow."""

    @pytest.mark.asyncio
    async def test_fetch_success_no_refresh_needed(self, mock_httpx_client, mock_response):
        """fetch completes successfully when token is valid."""
        strategy = ClaudeOAuthStrategy()

        # Mock credentials
        credentials = {
            "access_token": "valid_token",
            "expires_at": (datetime.now(UTC) + timedelta(hours=1)).isoformat(),
        }

        # Mock usage API response
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "five_hour": {"utilization": 50.0, "resets_at": "2025-01-15T15:00:00+00:00"},
            "seven_day": {"utilization": 30.0, "resets_at": "2025-01-22T18:00:00+00:00"},
        }

        mock_httpx_client.get.return_value = mock_response

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.claude.oauth.get_http_client",
                return_value=mock_httpx_client,
            ):
                result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot is not None
        assert isinstance(result.snapshot, UsageSnapshot)
        assert result.snapshot.provider == "claude"

    @pytest.mark.asyncio
    async def test_fetch_fails_without_credentials(self):
        """fetch fails when no credentials are available."""
        strategy = ClaudeOAuthStrategy()

        with patch.object(strategy, "_load_credentials", return_value=None):
            result = await strategy.fetch()

        assert result.success is False
        assert result.error == "No OAuth credentials found"

    @pytest.mark.asyncio
    async def test_fetch_fails_with_invalid_credentials(self, mock_httpx_client, mock_response):
        """fetch fails with 401 when credentials are invalid."""
        strategy = ClaudeOAuthStrategy()

        credentials = {
            "access_token": "invalid_token",
            "expires_at": (datetime.now(UTC) + timedelta(hours=1)).isoformat(),
        }

        mock_response.status_code = 401
        mock_httpx_client.get.return_value = mock_response

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.claude.oauth.get_http_client",
                return_value=mock_httpx_client,
            ):
                result = await strategy.fetch()

        assert result.success is False
        assert result.error is not None and "expired or invalid" in result.error

    @pytest.mark.asyncio
    async def test_fetch_handles_json_decode_error(self, mock_httpx_client, mock_response):
        """fetch handles invalid JSON response."""
        strategy = ClaudeOAuthStrategy()

        credentials = {
            "access_token": "valid_token",
            "expires_at": (datetime.now(UTC) + timedelta(hours=1)).isoformat(),
        }

        mock_response.status_code = 200
        mock_response.json.side_effect = json.JSONDecodeError("Invalid JSON", "", 0)
        mock_httpx_client.get.return_value = mock_response

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.claude.oauth.get_http_client",
                return_value=mock_httpx_client,
            ):
                result = await strategy.fetch()

        assert result.success is False
        assert result.error is not None and "Invalid response" in result.error

    @pytest.mark.asyncio
    async def test_fetch_with_token_refresh(self, mock_httpx_client, mock_response):
        """fetch refreshes token when needed."""
        strategy = ClaudeOAuthStrategy()

        # Expired credentials
        credentials = {
            "access_token": "expired_token",
            "refresh_token": "valid_refresh_token",
            "expires_at": (datetime.now(UTC) - timedelta(hours=1)).isoformat(),
        }

        # Mock refresh response
        refresh_response = MagicMock()
        refresh_response.status_code = 200
        refresh_response.json.return_value = {
            "access_token": "new_token",
            "refresh_token": "new_refresh",
            "expires_in": timedelta(seconds=3600),
        }

        # Mock usage response
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "five_hour": {"utilization": 50.0, "resets_at": "2025-01-15T15:00:00+00:00"},
        }

        mock_httpx_client.post.return_value = refresh_response
        mock_httpx_client.get.return_value = mock_response

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch.object(strategy, "_save_credentials"):
                with patch(
                    "vibeusage.providers.claude.oauth.get_http_client",
                    return_value=mock_httpx_client,
                ):
                    result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot is not None
