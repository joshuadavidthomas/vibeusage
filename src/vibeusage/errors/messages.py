"""Provider-specific error message templates with remediation."""

from __future__ import annotations


AUTH_ERROR_TEMPLATES: dict[str, dict[str, str]] = {
    "claude": {
        "401": (
            "Claude session expired or invalid.\n"
            "Run: [cyan]vibeusage auth claude[/cyan]"
        ),
        "403": (
            "Access denied. Your account may not have a Claude Max subscription.\n"
            "Check your subscription at: [cyan]https://claude.ai/settings/billing[/cyan]"
        ),
        "no_credentials": (
            "No Claude credentials found.\n"
            "Run '[cyan]vibeusage auth claude[/cyan]' to configure authentication."
        ),
        "cli_not_found": (
            "Claude CLI not found in PATH.\n"
            "Install it from: [cyan]https://claude.ai/download[/cyan]"
        ),
    },
    "codex": {
        "401": (
            "Codex session expired or invalid.\nRun: [cyan]vibeusage auth codex[/cyan]"
        ),
        "403": (
            "Access denied. Your account may not have a ChatGPT Plus/Pro subscription.\n"
            "Check your subscription at: [cyan]https://chatgpt.com/settings/subscription[/cyan]"
        ),
        "no_credentials": (
            "No Codex credentials found.\n"
            "Run '[cyan]vibeusage auth codex[/cyan]' to configure authentication."
        ),
    },
    "copilot": {
        "401": ("GitHub token expired.\nRun: [cyan]vibeusage auth copilot[/cyan]"),
        "403": (
            "GitHub Copilot not enabled for this account.\n"
            "Enable it at: [cyan]https://github.com/settings/copilot[/cyan]"
        ),
        "no_credentials": (
            "No Copilot credentials found.\n"
            "Run '[cyan]vibeusage auth copilot[/cyan]' to authenticate with GitHub."
        ),
    },
    "cursor": {
        "401": (
            "Cursor session expired.\n"
            "Log into cursor.com in your browser, then run:\n"
            "  [cyan]vibeusage auth cursor[/cyan]"
        ),
        "no_credentials": (
            "No Cursor session found.\n"
            "Log into [cyan]cursor.com[/cyan] in your browser first."
        ),
    },
    "gemini": {
        "401": ("Gemini credentials expired.\nRun: [cyan]vibeusage auth gemini[/cyan]"),
        "403": (
            "Gemini quota exceeded or access denied.\n"
            "Check your usage at: [cyan]https://aistudio.google.com/app/usage[/cyan]"
        ),
        "no_credentials": (
            "No Gemini credentials found.\n"
            "Run '[cyan]vibeusage auth gemini[/cyan]' to configure authentication."
        ),
    },
}


def get_auth_error_message(
    provider_id: str,
    error_type: str,
) -> str:
    """Get provider-specific auth error message.

    Args:
        provider_id: Provider identifier (e.g., "claude", "codex")
        error_type: Error type key (e.g., "401", "no_credentials")

    Returns:
        Error message string with remediation steps
    """
    templates = AUTH_ERROR_TEMPLATES.get(provider_id, {})
    return templates.get(
        error_type,
        f"Authentication error for {provider_id}. "
        f"Run '[cyan]vibeusage auth {provider_id}[/cyan]' to configure.",
    )


def get_provider_remediation(provider_id: str, category: str) -> str | None:
    """Get remediation message for a provider error category.

    Args:
        provider_id: Provider identifier
        category: Error category (e.g., "authentication", "network")

    Returns:
        Remediation message or None
    """
    from vibeusage.errors.types import ErrorCategory

    # General remediation by category
    general_remediation = {
        ErrorCategory.AUTHENTICATION: (
            f"Run '[cyan]vibeusage auth {provider_id}[/cyan]' to re-authenticate."
        ),
        ErrorCategory.AUTHORIZATION: (
            f"Check your {provider_id} account permissions and subscription status."
        ),
        ErrorCategory.RATE_LIMITED: ("Wait a few minutes before trying again."),
        ErrorCategory.NETWORK: ("Check your internet connection and try again."),
        ErrorCategory.PROVIDER: (
            f"The {provider_id} service may be experiencing issues. "
            f"Check [cyan]https://status.{provider_id}.com[/cyan] for outages."
        ),
        ErrorCategory.CONFIGURATION: (
            f"Run '[cyan]vibeusage config show[/cyan]' to check your configuration."
        ),
    }

    if category in general_remediation:
        return general_remediation[category]

    return None
