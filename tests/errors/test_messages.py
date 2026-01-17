"""Tests for errors/messages.py (provider-specific error message templates)."""

from __future__ import annotations

import pytest

from vibeusage.errors.messages import AUTH_ERROR_TEMPLATES
from vibeusage.errors.messages import get_auth_error_message
from vibeusage.errors.messages import get_provider_remediation
from vibeusage.errors.types import ErrorCategory


class TestAuthErrorTemplates:
    """Tests for AUTH_ERROR_TEMPLATES constant."""

    def test_has_entries_for_all_providers(self):
        """AUTH_ERROR_TEMPLATES has entries for all known providers."""
        expected_providers = ["claude", "codex", "copilot", "cursor", "gemini"]

        for provider in expected_providers:
            assert provider in AUTH_ERROR_TEMPLATES
            assert isinstance(AUTH_ERROR_TEMPLATES[provider], dict)

    def test_claud_templates_have_required_keys(self):
        """Claude templates have all required error type keys."""
        claude_templates = AUTH_ERROR_TEMPLATES["claude"]

        # Check for standard error types
        assert "401" in claude_templates
        assert "403" in claude_templates
        assert "no_credentials" in claude_templates

        # Verify values are strings
        for key, value in claude_templates.items():
            assert isinstance(value, str)
            assert len(value) > 0

    def test_codex_templates_have_required_keys(self):
        """Codex templates have all required error type keys."""
        codex_templates = AUTH_ERROR_TEMPLATES["codex"]

        assert "401" in codex_templates
        assert "403" in codex_templates
        assert "no_credentials" in codex_templates

    def test_copilot_templates_have_required_keys(self):
        """Copilot templates have all required error type keys."""
        copilot_templates = AUTH_ERROR_TEMPLATES["copilot"]

        assert "401" in copilot_templates
        assert "403" in copilot_templates
        assert "no_credentials" in copilot_templates

    def test_cursor_templates_have_required_keys(self):
        """Cursor templates have all required error type keys."""
        cursor_templates = AUTH_ERROR_TEMPLATES["cursor"]

        assert "401" in cursor_templates
        assert "no_credentials" in cursor_templates

    def test_gemini_templates_have_required_keys(self):
        """Gemini templates have all required error type keys."""
        gemini_templates = AUTH_ERROR_TEMPLATES["gemini"]

        assert "401" in gemini_templates
        assert "403" in gemini_templates
        assert "no_credentials" in gemini_templates

    def test_templates_contain_helpful_commands(self):
        """Error templates contain references to vibeusage commands."""
        claude_templates = AUTH_ERROR_TEMPLATES["claude"]

        # Check that templates reference the auth command
        assert "vibeusage auth" in claude_templates["401"]
        assert "vibeusage auth" in claude_templates["no_credentials"]

    def test_claud_has_cli_not_found_template(self):
        """Claude has a specific template for CLI not found error."""
        claude_templates = AUTH_ERROR_TEMPLATES["claude"]

        assert "cli_not_found" in claude_templates
        assert "claude.ai/download" in claude_templates["cli_not_found"]


class TestGetAuthErrorMessage:
    """Tests for get_auth_error_message function."""

    def test_claud_401_error_message(self):
        """Returns Claude-specific 401 error message."""
        message = get_auth_error_message("claude", "401")

        assert "session expired" in message.lower() or "invalid" in message.lower()
        assert "vibeusage auth claude" in message

    def test_claud_403_error_message(self):
        """Returns Claude-specific 403 error message."""
        message = get_auth_error_message("claude", "403")

        assert "Access denied" in message or "account" in message
        assert "claude.ai/settings/billing" in message

    def test_claud_no_credentials_message(self):
        """Returns Claude-specific no credentials message."""
        message = get_auth_error_message("claude", "no_credentials")

        assert "No Claude credentials" in message
        assert "vibeusage auth claude" in message

    def test_claud_cli_not_found_message(self):
        """Returns Claude CLI not found message."""
        message = get_auth_error_message("claude", "cli_not_found")

        assert "CLI not found" in message
        assert "claude.ai/download" in message

    def test_codex_401_error_message(self):
        """Returns Codex-specific 401 error message."""
        message = get_auth_error_message("codex", "401")

        assert "session expired" in message.lower() or "invalid" in message.lower()
        assert "vibeusage auth codex" in message

    def test_codex_403_error_message(self):
        """Returns Codex-specific 403 error message."""
        message = get_auth_error_message("codex", "403")

        assert "Access denied" in message or "account" in message
        assert "chatgpt.com/settings/subscription" in message

    def test_codex_no_credentials_message(self):
        """Returns Codex-specific no credentials message."""
        message = get_auth_error_message("codex", "no_credentials")

        assert "No Codex credentials" in message
        assert "vibeusage auth codex" in message

    def test_copilot_401_error_message(self):
        """Returns Copilot-specific 401 error message."""
        message = get_auth_error_message("copilot", "401")

        assert "token expired" in message.lower() or "expired" in message.lower()
        assert "vibeusage auth copilot" in message

    def test_copilot_403_error_message(self):
        """Returns Copilot-specific 403 error message."""
        message = get_auth_error_message("copilot", "403")

        assert "Copilot not enabled" in message or "Access denied" in message
        assert "github.com/settings/copilot" in message

    def test_copilot_no_credentials_message(self):
        """Returns Copilot-specific no credentials message."""
        message = get_auth_error_message("copilot", "no_credentials")

        assert "No Copilot credentials" in message
        assert "vibeusage auth copilot" in message

    def test_cursor_401_error_message(self):
        """Returns Cursor-specific 401 error message."""
        message = get_auth_error_message("cursor", "401")

        assert "session expired" in message.lower()
        assert "cursor.com" in message
        assert "vibeusage auth cursor" in message

    def test_cursor_no_credentials_message(self):
        """Returns Cursor-specific no credentials message."""
        message = get_auth_error_message("cursor", "no_credentials")

        assert "No Cursor session" in message
        assert "cursor.com" in message

    def test_gemini_401_error_message(self):
        """Returns Gemini-specific 401 error message."""
        message = get_auth_error_message("gemini", "401")

        assert "credentials expired" in message.lower()
        assert "vibeusage auth gemini" in message

    def test_gemini_403_error_message(self):
        """Returns Gemini-specific 403 error message."""
        message = get_auth_error_message("gemini", "403")

        assert "quota exceeded" in message.lower() or "access denied" in message.lower()
        assert "aistudio.google.com/app/usage" in message

    def test_gemini_no_credentials_message(self):
        """Returns Gemini-specific no credentials message."""
        message = get_auth_error_message("gemini", "no_credentials")

        assert "No Gemini credentials" in message
        assert "vibeusage auth gemini" in message

    def test_unknown_provider_returns_generic_message(self):
        """Returns generic message for unknown provider."""
        message = get_auth_error_message("unknown_provider", "401")

        assert "Authentication error for unknown_provider" in message
        assert "vibeusage auth unknown_provider" in message

    def test_unknown_error_type_returns_default(self):
        """Returns default message for unknown error type."""
        message = get_auth_error_message("claude", "unknown_error")

        assert "Authentication error for claude" in message
        assert "vibeusage auth claude" in message

    def test_empty_provider_returns_generic_message(self):
        """Returns generic message for empty provider string."""
        message = get_auth_error_message("", "401")

        assert "Authentication error for" in message

    def test_empty_error_type_returns_default(self):
        """Returns default message for empty error type."""
        message = get_auth_error_message("claude", "")

        assert "Authentication error for claude" in message


class TestGetProviderRemediation:
    """Tests for get_provider_remediation function."""

    def test_authentication_category_remediation(self):
        """Returns remediation for authentication errors."""
        message = get_provider_remediation("claude", ErrorCategory.AUTHENTICATION)

        assert message is not None
        assert "vibeusage auth claude" in message
        assert "re-authenticate" in message.lower()

    def test_authorization_category_remediation(self):
        """Returns remediation for authorization errors."""
        message = get_provider_remediation("codex", ErrorCategory.AUTHORIZATION)

        assert message is not None
        assert "codex" in message
        assert "permissions" in message.lower() or "subscription" in message.lower()

    def test_rate_limited_category_remediation(self):
        """Returns remediation for rate limit errors."""
        message = get_provider_remediation("copilot", ErrorCategory.RATE_LIMITED)

        assert message is not None
        assert "Wait" in message or "few minutes" in message

    def test_network_category_remediation(self):
        """Returns remediation for network errors."""
        message = get_provider_remediation("cursor", ErrorCategory.NETWORK)

        assert message is not None
        assert "internet connection" in message.lower()

    def test_provider_category_remediation(self):
        """Returns remediation for provider errors."""
        message = get_provider_remediation("gemini", ErrorCategory.PROVIDER)

        assert message is not None
        assert "gemini" in message.lower()
        assert "status" in message.lower()

    def test_configuration_category_remediation(self):
        """Returns remediation for configuration errors."""
        message = get_provider_remediation("claude", ErrorCategory.CONFIGURATION)

        assert message is not None
        assert "vibeusage config show" in message

    def test_unknown_category_returns_none(self):
        """Returns None for categories without specific remediation."""
        # PARSE, NOT_FOUND, and UNKNOWN don't have remediation
        message = get_provider_remediation("claude", ErrorCategory.PARSE)
        assert message is None

        message = get_provider_remediation("claude", ErrorCategory.NOT_FOUND)
        assert message is None

        message = get_provider_remediation("claude", ErrorCategory.UNKNOWN)
        assert message is None

    def test_string_category_returns_none(self):
        """Returns None for string category not in mapping."""
        message = get_provider_remediation("claude", "not_a_real_category")

        assert message is None

    def test_remediation_includes_provider_name(self):
        """Remediation messages include the provider name where relevant."""
        message = get_provider_remediation("testprovider", ErrorCategory.AUTHENTICATION)

        assert "testprovider" in message.lower()

    def test_all_error_categories_have_remediation(self):
        """All ErrorCategory enum values have remediation messages."""
        # Note: PARSE, NOT_FOUND, and UNKNOWN don't have specific remediation
        # so we only test the categories that do
        categories_with_remediation = [
            ErrorCategory.AUTHENTICATION,
            ErrorCategory.AUTHORIZATION,
            ErrorCategory.RATE_LIMITED,
            ErrorCategory.NETWORK,
            ErrorCategory.PROVIDER,
            ErrorCategory.CONFIGURATION,
        ]
        for category in categories_with_remediation:
            message = get_provider_remediation("test", category)
            assert message is not None, f"No remediation for {category}"
            assert len(message) > 0
