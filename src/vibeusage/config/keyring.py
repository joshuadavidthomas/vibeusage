"""Optional system keyring integration for secure credential storage."""

from functools import lru_cache

_keyring_available: bool | None = None


def _check_keyring_available() -> bool:
    """Check if keyring module is available and functional."""
    try:
        import keyring

        # Try to get the keyring backend
        backend = keyring.get_keyring()
        return backend is not None
    except Exception:
        return False


@lru_cache(maxsize=1)
def use_keyring() -> bool:
    """Check if keyring should be used.

    Returns True only if:
    1. Keyring is available
    2. It's enabled in config
    """
    from .settings import get_config

    config = get_config()
    if not config.credentials.use_keyring:
        return False

    return _check_keyring_available()


def keyring_key(provider_id: str, credential_type: str) -> str:
    """Generate a keyring key for storage."""
    return f"vibeusage:{provider_id}:{credential_type}"


def store_in_keyring(provider_id: str, credential_type: str, value: str) -> bool:
    """Store credential in system keyring.

    Returns:
        True if stored successfully, False otherwise
    """
    if not use_keyring():
        return False

    try:
        import keyring

        key = keyring_key(provider_id, credential_type)
        keyring.set_password("vibeusage", key, value)
        return True
    except Exception:
        return False


def get_from_keyring(provider_id: str, credential_type: str) -> str | None:
    """Retrieve credential from system keyring.

    Returns:
        Credential value if found, None otherwise
    """
    if not use_keyring():
        return None

    try:
        import keyring

        key = keyring_key(provider_id, credential_type)
        value = keyring.get_password("vibeusage", key)
        return value
    except Exception:
        return None


def delete_from_keyring(provider_id: str, credential_type: str) -> bool:
    """Delete credential from system keyring.

    Returns:
        True if deleted successfully, False otherwise
    """
    if not use_keyring():
        return False

    try:
        import keyring

        key = keyring_key(provider_id, credential_type)
        keyring.delete_password("vibeusage", key)
        return True
    except Exception:
        return False
