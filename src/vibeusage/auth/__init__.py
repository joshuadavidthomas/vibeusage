"""Authentication strategies for vibeusage."""
from __future__ import annotations

from vibeusage.auth.base import REFRESH_BUFFER
from vibeusage.auth.base import APIKeyCredentials
from vibeusage.auth.base import AuthCredentials
from vibeusage.auth.base import AuthResult
from vibeusage.auth.base import AuthStrategy
from vibeusage.auth.base import CLIConfig
from vibeusage.auth.base import CLICredentials
from vibeusage.auth.base import CookieConfig
from vibeusage.auth.base import DeviceFlowConfig
from vibeusage.auth.base import LocalProcessConfig
from vibeusage.auth.base import LocalProcessCredentials
from vibeusage.auth.base import OAuth2Config
from vibeusage.auth.base import OAuth2Credentials
from vibeusage.auth.base import ProviderAuthConfig
from vibeusage.auth.base import SessionCredentials

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
