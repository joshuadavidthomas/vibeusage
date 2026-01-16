"""Authentication strategies for vibeusage."""

from vibeusage.auth.base import (
    AuthCredentials,
    AuthResult,
    AuthStrategy,
    OAuth2Credentials,
    SessionCredentials,
    APIKeyCredentials,
    CLICredentials,
    LocalProcessCredentials,
    REFRESH_BUFFER,
    OAuth2Config,
    CookieConfig,
    CLIConfig,
    DeviceFlowConfig,
    LocalProcessConfig,
    ProviderAuthConfig,
)

__all__ = [
    "AuthCredentials",
    "AuthResult",
    "AuthStrategy",
    "OAuth2Credentials",
    "SessionCredentials",
    "APIKeyCredentials",
    "CLICredentials",
    "LocalProcessCredentials",
    "REFRESH_BUFFER",
    "OAuth2Config",
    "CookieConfig",
    "CLIConfig",
    "DeviceFlowConfig",
    "LocalProcessConfig",
    "ProviderAuthConfig",
]
