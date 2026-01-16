# Spec 04: Provider Implementations

**Status**: Draft
**Dependencies**: 01-architecture, 02-data-models, 03-authentication
**Dependents**: 05-cli-interface, 06-configuration, 07-error-handling

## Overview

This specification defines the provider implementations for vibeusage, detailing API endpoints, authentication mechanisms, data parsing, and rate window mapping for each supported LLM service.

## Design Goals

1. **Incremental Implementation**: Start with Claude, expand to other providers
2. **Consistent Interface**: All providers produce `UsageSnapshot` per spec 02
3. **Fallback Resilience**: Multiple fetch strategies per provider
4. **Maintainability**: Provider-specific logic isolated in dedicated modules

## Provider Priority

Implementation order based on usage frequency and complexity:

| Priority | Provider | Rationale |
|----------|----------|-----------|
| 1 | Claude | Primary target, most complex (OAuth + web + CLI) |
| 2 | Codex | Popular OpenAI/ChatGPT, similar OAuth pattern |
| 3 | Copilot | GitHub device flow, distinct auth model |
| 4 | Cursor | Cookie-based, popular IDE |
| 5 | Gemini | Google OAuth, growing usage |

**Later phases**: Augment, Factory, VertexAI, MiniMax, Antigravity, Kiro, Zai

## Provider Registry

All providers register via the decorator pattern from spec 01:

```python
from vibeusage.providers.base import Provider, ProviderMetadata, register_provider

@register_provider
class ClaudeProvider(Provider):
    metadata = ProviderMetadata(
        id="claude",
        name="Claude",
        description="Anthropic's Claude AI assistant",
        homepage="https://claude.ai",
        status_url="https://status.anthropic.com",
        dashboard_url="https://claude.ai/settings/usage",
    )
    ...
```

**Module structure**:

```
vibeusage/providers/
├── __init__.py          # Registry: get_provider, list_providers
├── base.py              # Provider protocol, ProviderMetadata
├── claude/
│   ├── __init__.py      # ClaudeProvider
│   ├── oauth.py         # ClaudeOAuthStrategy
│   ├── web.py           # ClaudeWebStrategy
│   └── cli.py           # ClaudeCLIStrategy
├── codex/
│   ├── __init__.py
│   ├── oauth.py
│   └── web.py
├── copilot/
│   ├── __init__.py
│   └── device_flow.py
├── cursor/
│   ├── __init__.py
│   └── web.py
└── gemini/
    ├── __init__.py
    └── oauth.py
```

---

## Provider: Claude

Anthropic's Claude AI assistant with Claude Code CLI integration.

### Metadata

```python
ProviderMetadata(
    id="claude",
    name="Claude",
    description="Anthropic's Claude AI assistant",
    homepage="https://claude.ai",
    status_url="https://status.anthropic.com",
    dashboard_url="https://claude.ai/settings/usage",
)
```

### Fetch Strategies

| Priority | Strategy | Source | Authentication |
|----------|----------|--------|----------------|
| 1 | OAuth | API | OAuth 2.0 tokens |
| 2 | Web | Web API | Session cookie |
| 3 | CLI | claude binary | CLI-managed session |

### OAuth Strategy

**Credential sources** (in order):
1. `~/.claude/.credentials.json` (Claude Code CLI)
2. `~/.config/vibeusage/credentials/claude/oauth.json` (vibeusage storage)
3. macOS Keychain service `"Claude Code-credentials"` (optional)

**Token refresh**:
- Endpoint: `POST https://api.anthropic.com/oauth/token`
- Refresh threshold: 5-8 days before expiry

**Usage endpoint**:

```
GET https://api.anthropic.com/api/oauth/usage
Headers:
  Authorization: Bearer {access_token}
  anthropic-beta: oauth-2025-04-20
```

**Response mapping**:

```python
def parse_oauth_response(data: dict) -> UsageSnapshot:
    periods = []

    if five_hour := data.get("five_hour"):
        periods.append(UsagePeriod(
            name="Session (5h)",
            utilization=five_hour["utilization"],
            period_type=PeriodType.SESSION,
            resets_at=parse_iso_datetime(five_hour.get("resets_at")),
        ))

    if seven_day := data.get("seven_day"):
        periods.append(UsagePeriod(
            name="Weekly",
            utilization=seven_day["utilization"],
            period_type=PeriodType.WEEKLY,
            resets_at=parse_iso_datetime(seven_day.get("resets_at")),
        ))

    # Model-specific windows
    if opus := data.get("seven_day_opus"):
        periods.append(UsagePeriod(
            name="Opus",
            utilization=opus["utilization"],
            period_type=PeriodType.WEEKLY,
            resets_at=parse_iso_datetime(opus.get("resets_at")),
            model="opus",
        ))

    if sonnet := data.get("seven_day_sonnet"):
        periods.append(UsagePeriod(
            name="Sonnet",
            utilization=sonnet["utilization"],
            period_type=PeriodType.WEEKLY,
            resets_at=parse_iso_datetime(sonnet.get("resets_at")),
            model="sonnet",
        ))

    return UsageSnapshot(
        provider="claude",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        source="oauth",
    )
```

### Web Strategy

**Authentication**: Session cookie (`sessionKey`)

**Credential sources**:
1. Browser cookie import (Safari, Chrome, Firefox, Arc, Brave, Edge)
2. `~/.config/vibeusage/credentials/claude/session-key` (manual)

**API endpoints**:

```
# Get organizations (to find org_id)
GET https://claude.ai/api/organizations
Cookie: sessionKey={session_key}

# Get usage
GET https://claude.ai/api/organizations/{org_id}/usage
Cookie: sessionKey={session_key}

# Get overage (optional, may 404)
GET https://claude.ai/api/organizations/{org_id}/overage_spend_limit
Cookie: sessionKey={session_key}
```

**Organization selection**:
- Find org with `"chat"` in `capabilities` array (Claude Max subscription)
- Fall back to first org
- Cache org_id to `~/.cache/vibeusage/org-ids/claude`

**Overage mapping**:

```python
def parse_overage(data: dict | None) -> OverageUsage | None:
    if not data or not data.get("is_enabled"):
        return None
    return OverageUsage(
        used=Decimal(str(data["used_credits"])),
        limit=Decimal(str(data["monthly_credit_limit"])),
        currency=data["currency"],
        is_enabled=True,
    )
```

### CLI Strategy

**Prerequisite**: `claude` binary in PATH (from Claude Code)

**Command**:
```bash
claude /usage
```

**Output format**: ANSI-colored terminal output with progress bars

**Parsing**:

```python
import re

ANSI_PATTERN = re.compile(r'\x1b\[[0-9;]*m')

def parse_cli_output(output: str) -> UsageSnapshot:
    """Parse Claude CLI /usage output."""
    # Strip ANSI codes
    clean = ANSI_PATTERN.sub('', output)

    periods = []

    # Look for patterns like "Session (5h): 45% used"
    session_match = re.search(r'Session.*?(\d+)%', clean)
    if session_match:
        periods.append(UsagePeriod(
            name="Session (5h)",
            utilization=int(session_match.group(1)),
            period_type=PeriodType.SESSION,
        ))

    weekly_match = re.search(r'Weekly.*?(\d+)%', clean)
    if weekly_match:
        periods.append(UsagePeriod(
            name="Weekly",
            utilization=int(weekly_match.group(1)),
            period_type=PeriodType.WEEKLY,
        ))

    return UsageSnapshot(
        provider="claude",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        source="cli",
    )
```

**CLI execution**:

```python
async def fetch_via_cli() -> FetchResult:
    try:
        proc = await asyncio.create_subprocess_exec(
            "claude", "/usage",
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await asyncio.wait_for(
            proc.communicate(),
            timeout=30.0,
        )

        if proc.returncode != 0:
            return FetchResult.fail(f"CLI error: {stderr.decode()}")

        snapshot = parse_cli_output(stdout.decode())
        return FetchResult.ok(snapshot)

    except asyncio.TimeoutError:
        return FetchResult.fail("CLI timed out")
```

### Status Polling

**Source**: Statuspage.io

```
GET https://status.anthropic.com/api/v2/status.json
```

**Response mapping**:

```python
def parse_statuspage(data: dict) -> ProviderStatus:
    indicator = data["status"]["indicator"]
    return ProviderStatus(
        level=STATUS_MAP.get(indicator, StatusLevel.UNKNOWN),
        description=data["status"].get("description"),
        updated_at=parse_iso_datetime(data.get("page", {}).get("updated_at")),
    )

STATUS_MAP = {
    "none": StatusLevel.OPERATIONAL,
    "minor": StatusLevel.DEGRADED,
    "major": StatusLevel.PARTIAL_OUTAGE,
    "critical": StatusLevel.MAJOR_OUTAGE,
}
```

---

## Provider: Codex (OpenAI)

OpenAI's ChatGPT/Codex with CLI integration.

### Metadata

```python
ProviderMetadata(
    id="codex",
    name="Codex",
    description="OpenAI's ChatGPT and Codex",
    homepage="https://chatgpt.com",
    status_url="https://status.openai.com",
    dashboard_url="https://chatgpt.com/codex/settings/usage",
)
```

### Fetch Strategies

| Priority | Strategy | Source | Authentication |
|----------|----------|--------|----------------|
| 1 | OAuth | API | OAuth 2.0 tokens |
| 2 | Web | Dashboard | Session cookie (future) |

### OAuth Strategy

**Credential sources**:
1. `~/.codex/auth.json` (Codex CLI)
2. `~/.config/vibeusage/credentials/codex/oauth.json`

**Token refresh**:
- Endpoint: `POST https://auth.openai.com/oauth/token`
- Client ID: `app_EMoamEEZ73f0CkXaXp7hrann`
- Refresh threshold: 8 days before expiry

**Usage endpoint**:

```
GET https://chatgpt.com/backend-api/wham/usage
Headers:
  Authorization: Bearer {access_token}
```

**Note**: Endpoint may be customized in `~/.codex/config.toml` under `usage_url`.

**Response mapping**:

```python
def parse_codex_response(data: dict) -> UsageSnapshot:
    periods = []

    rate_limits = data.get("rate_limits", {})

    if primary := rate_limits.get("primary"):
        periods.append(UsagePeriod(
            name="Session",
            utilization=int(primary["used_percent"]),
            period_type=PeriodType.SESSION,
            resets_at=datetime.fromtimestamp(
                primary["reset_timestamp"]
            ).astimezone() if primary.get("reset_timestamp") else None,
        ))

    if secondary := rate_limits.get("secondary"):
        periods.append(UsagePeriod(
            name="Weekly",
            utilization=int(secondary["used_percent"]),
            period_type=PeriodType.WEEKLY,
            resets_at=datetime.fromtimestamp(
                secondary["reset_timestamp"]
            ).astimezone() if secondary.get("reset_timestamp") else None,
        ))

    # Credits (overage)
    overage = None
    credits = data.get("credits", {})
    if credits.get("has_credits"):
        overage = OverageUsage(
            used=Decimal("0"),  # API doesn't expose used amount
            limit=Decimal(str(credits.get("balance", 0))),
            currency="credits",
            is_enabled=True,
        )

    # Plan info
    identity = ProviderIdentity(
        plan=data.get("plan_type"),
    )

    return UsageSnapshot(
        provider="codex",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        overage=overage,
        identity=identity,
        source="oauth",
    )
```

### Status Polling

**Source**: Statuspage.io

```
GET https://status.openai.com/api/v2/status.json
```

Uses same `parse_statuspage()` as Claude.

---

## Provider: Copilot

GitHub Copilot with device flow OAuth.

### Metadata

```python
ProviderMetadata(
    id="copilot",
    name="Copilot",
    description="GitHub Copilot AI assistant",
    homepage="https://github.com/features/copilot",
    status_url="https://www.githubstatus.com",
    dashboard_url="https://github.com/settings/copilot",
)
```

### Fetch Strategies

| Priority | Strategy | Source | Authentication |
|----------|----------|--------|----------------|
| 1 | Device Flow | GitHub API | OAuth token |

### Device Flow OAuth

**Configuration**:
- Device code endpoint: `POST https://github.com/login/device/code`
- Token endpoint: `POST https://github.com/login/oauth/access_token`
- Client ID: `Iv1.b507a08c87ecfe98` (VS Code Copilot client ID)
- Scope: `read:user`

**Credential storage**: `~/.config/vibeusage/credentials/copilot/oauth.json`

**Usage endpoint**:

```
GET https://api.github.com/copilot_internal/user
Headers:
  Authorization: token {access_token}
  Accept: application/json
```

**Response mapping**:

```python
def parse_copilot_response(data: dict) -> UsageSnapshot:
    periods = []

    # Premium interactions quota
    if premium := data.get("premium_interactions"):
        entitlement = premium.get("entitlement", 0)
        remaining = premium.get("remaining", 0)
        utilization = 100 - int((remaining / entitlement) * 100) if entitlement > 0 else 0

        periods.append(UsagePeriod(
            name="Premium",
            utilization=utilization,
            period_type=PeriodType.MONTHLY,
            resets_at=parse_iso_datetime(data.get("quota_reset_date")),
        ))

    # Chat quota (model-specific)
    if chat := data.get("chat"):
        entitlement = chat.get("entitlement", 0)
        remaining = chat.get("remaining", 0)
        utilization = 100 - int((remaining / entitlement) * 100) if entitlement > 0 else 0

        periods.append(UsagePeriod(
            name="Chat",
            utilization=utilization,
            period_type=PeriodType.MONTHLY,
            resets_at=parse_iso_datetime(data.get("quota_reset_date")),
            model="chat",
        ))

    identity = ProviderIdentity(
        plan=data.get("plan_type"),  # "premium" or "free"
    )

    return UsageSnapshot(
        provider="copilot",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        identity=identity,
        source="device_flow",
    )
```

### Status Polling

**Source**: Statuspage.io

```
GET https://www.githubstatus.com/api/v2/status.json
```

---

## Provider: Cursor

Cursor IDE with browser cookie authentication.

### Metadata

```python
ProviderMetadata(
    id="cursor",
    name="Cursor",
    description="Cursor IDE AI features",
    homepage="https://cursor.com",
    status_url="https://status.cursor.com",
    dashboard_url="https://cursor.com/settings/usage",
)
```

### Fetch Strategies

| Priority | Strategy | Source | Authentication |
|----------|----------|--------|----------------|
| 1 | Web | Cursor API | Session cookie |

### Web Strategy

**Cookie configuration**:
- Names: `WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `next-auth.session-token`
- Domains: `cursor.com`, `cursor.sh`

**Credential sources**:
1. Browser cookie import
2. `~/.config/vibeusage/credentials/cursor/session.json` (stored)
3. Manual session key

**API endpoints**:

```
# Primary usage endpoint
POST https://www.cursor.com/api/usage-summary
Cookie: {session_cookie}
Content-Type: application/json
Body: {}

# User info
GET https://www.cursor.com/api/auth/me
Cookie: {session_cookie}
```

**Response mapping**:

```python
def parse_cursor_response(usage_data: dict, user_data: dict) -> UsageSnapshot:
    periods = []

    # Premium requests
    premium = usage_data.get("premium_requests", {})
    used = premium.get("used", 0)
    available = premium.get("available", 0)
    total = used + available
    utilization = int((used / total) * 100) if total > 0 else 0

    # Billing cycle for reset time
    billing_end = usage_data.get("billing_cycle", {}).get("end")

    periods.append(UsagePeriod(
        name="Premium Requests",
        utilization=utilization,
        period_type=PeriodType.MONTHLY,
        resets_at=parse_iso_datetime(billing_end),
    ))

    # On-demand spend (overage)
    overage = None
    on_demand = usage_data.get("on_demand_spend", {})
    if on_demand.get("limit_cents", 0) > 0:
        overage = OverageUsage(
            used=Decimal(on_demand.get("used_cents", 0)) / 100,
            limit=Decimal(on_demand.get("limit_cents", 0)) / 100,
            currency="USD",
            is_enabled=True,
        )

    identity = ProviderIdentity(
        email=user_data.get("email"),
        plan=user_data.get("membership_type"),
    )

    return UsageSnapshot(
        provider="cursor",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        overage=overage,
        identity=identity,
        source="web",
    )
```

### Status Polling

**Source**: Statuspage.io

```
GET https://status.cursor.com/api/v2/status.json
```

---

## Provider: Gemini

Google Gemini with OAuth via CLI credentials.

### Metadata

```python
ProviderMetadata(
    id="gemini",
    name="Gemini",
    description="Google Gemini AI",
    homepage="https://gemini.google.com",
    status_url=None,  # Uses Google Workspace status
    dashboard_url="https://aistudio.google.com/app/usage",
)
```

### Fetch Strategies

| Priority | Strategy | Source | Authentication |
|----------|----------|--------|----------------|
| 1 | OAuth | Cloud Code API | OAuth 2.0 tokens |

### OAuth Strategy

**Credential sources**:
1. `~/.gemini/oauth_creds.json` (Gemini CLI)
2. `~/.config/vibeusage/credentials/gemini/oauth.json`

**Token refresh**:
- Client credentials extracted from Gemini CLI installation (`oauth2.js`)
- Endpoint: `POST https://oauth2.googleapis.com/token`

**API endpoints**:

```
# Quota information
POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota
Headers:
  Authorization: Bearer {access_token}
Content-Type: application/json
Body: {}

# User tier (optional)
POST https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist
Headers:
  Authorization: Bearer {access_token}
```

**Response mapping**:

```python
def parse_gemini_response(quota_data: dict, user_data: dict | None) -> UsageSnapshot:
    periods = []

    # Each quota bucket is a model
    for bucket in quota_data.get("quota_buckets", []):
        remaining_fraction = bucket.get("remaining_fraction", 1.0)
        utilization = int((1 - remaining_fraction) * 100)
        model_id = bucket.get("model_id", "unknown")

        # Extract model name from ID
        model_name = model_id.split("/")[-1] if "/" in model_id else model_id

        periods.append(UsagePeriod(
            name=model_name.title(),
            utilization=utilization,
            period_type=PeriodType.DAILY,  # Gemini uses daily quotas
            resets_at=parse_iso_datetime(bucket.get("reset_time")),
            model=model_name,
        ))

    # User tier
    identity = None
    if user_data:
        tier = user_data.get("user_tier", "unknown")
        identity = ProviderIdentity(plan=tier)

    return UsageSnapshot(
        provider="gemini",
        fetched_at=datetime.now().astimezone(),
        periods=tuple(periods),
        identity=identity,
        source="oauth",
    )
```

### Status Polling

**Source**: Google Workspace incidents feed

```
GET https://www.google.com/appsstatus/dashboard/incidents.json
```

Filter for "Gemini" product.

---

## Future Providers

Brief specifications for providers in later implementation phases.

### Augment

- **Auth**: Browser cookies (`session`, `_session`, `web_rpc_proxy_session`)
- **Domains**: `augmentcode.com`, `app.augmentcode.com`
- **Endpoints**: `GET /api/credits`, `GET /api/subscription`
- **Data**: Credits remaining/consumed, subscription plan, billing period

### Factory (Droid)

- **Auth**: Multi-layer fallback (Safari cookies → Chromium → stored session → bearer token → refresh token)
- **Cookie types**: `wos-session`, next-auth variants, `access-token`
- **Domains**: `factory.ai`, `app.factory.ai`, `auth.factory.ai`
- **Endpoints**: `GET /api/app/auth/me`, `GET /api/organization/subscription/usage`
- **Data**: Standard/premium tier tokens, org tokens, allowance, overage

### VertexAI

- **Auth**: Google Cloud ADC (`~/.config/gcloud/application_default_credentials.json`)
- **Token refresh**: `POST https://oauth2.googleapis.com/token`
- **Endpoint**: `GET https://monitoring.googleapis.com/v3/projects/{projectId}/timeSeries`
- **Data**: Quota allocation usage metrics for `aiplatform.googleapis.com`

### MiniMax

- **Auth**: Combined Cookie + Bearer token from browser localStorage
- **Endpoints**: HTML scraping + JSON API
- **Data**: Plan name, available prompts, usage count, reset times

### Antigravity

- **Auth**: Local process communication (CSRF token from running `language_server_macos`)
- **Endpoints**: Local HTTPS gRPC to `127.0.0.1`
- **Data**: Model configs with quota info, user status, plan info

### Kiro

- **Auth**: CLI-managed OAuth (`kiro-cli`)
- **Command**: `kiro-cli chat --no-interactive /usage`
- **Data**: Plan name, credits used/total, bonus credits, reset date

### Zai

- **Auth**: API key (`Z_AI_API_KEY` env var)
- **Endpoint**: `GET https://api.z.ai/api/monitor/usage/quota/limit`
- **Data**: Time limits, token limits, per-model usage, plan name

---

## Error Handling by Provider

Provider-specific error handling:

| Error | Claude | Codex | Copilot | Cursor | Gemini |
|-------|--------|-------|---------|--------|--------|
| 401 Unauthorized | Refresh OAuth or re-auth | Refresh OAuth | Re-run device flow | Re-import cookies | Refresh OAuth |
| 403 Forbidden | Account lacks access | Account lacks access | Copilot not enabled | Subscription expired | Quota exceeded |
| 404 Not Found | Org not found | Endpoint moved | N/A | User not found | N/A |
| 429 Rate Limited | Back off + retry | Back off + retry | Back off + retry | Back off + retry | Back off + retry |

### Error Messages

```python
PROVIDER_ERROR_MESSAGES = {
    "claude": {
        "401": "Claude session expired. Run: vibeusage auth claude",
        "403": "Account may not have Claude Max subscription.",
        "no_org": "No organization found. Ensure you're logged into Claude.",
    },
    "codex": {
        "401": "Codex session expired. Run: vibeusage auth codex",
        "403": "Account may not have ChatGPT Plus/Pro subscription.",
    },
    "copilot": {
        "401": "GitHub token expired. Run: vibeusage auth copilot",
        "no_copilot": "GitHub Copilot not enabled for this account.",
    },
    "cursor": {
        "401": "Cursor session expired. Log into cursor.com in your browser.",
        "no_session": "No Cursor session found. Log into cursor.com first.",
    },
    "gemini": {
        "401": "Gemini credentials expired. Run: vibeusage auth gemini",
        "quota": "Gemini quota exceeded. Resets at {reset_time}.",
    },
}
```

---

## Adding New Providers

To add a new provider:

### 1. Create Provider Module

```python
# vibeusage/providers/newprovider/__init__.py
from vibeusage.providers.base import Provider, ProviderMetadata, register_provider
from vibeusage.strategies.base import FetchStrategy

@register_provider
class NewProvider(Provider):
    metadata = ProviderMetadata(
        id="newprovider",
        name="New Provider",
        description="Description of the provider",
        homepage="https://newprovider.com",
        status_url="https://status.newprovider.com",
        dashboard_url="https://newprovider.com/usage",
    )

    def fetch_strategies(self) -> list[FetchStrategy]:
        return [
            NewProviderOAuthStrategy(),
            NewProviderWebStrategy(),
        ]

    async def fetch_status(self) -> ProviderStatus:
        return await fetch_statuspage_status(self.metadata.status_url)
```

### 2. Implement Fetch Strategies

```python
# vibeusage/providers/newprovider/oauth.py
from vibeusage.strategies.base import FetchStrategy, FetchResult
from vibeusage.auth.oauth import OAuth2Strategy, OAuth2Config
from vibeusage.models import UsageSnapshot, UsagePeriod, PeriodType

class NewProviderOAuthStrategy(FetchStrategy):
    @property
    def name(self) -> str:
        return "oauth"

    async def is_available(self) -> bool:
        auth = OAuth2Strategy(OAuth2Config(
            token_endpoint="https://auth.newprovider.com/token",
            client_id="...",
            credentials_file=Path("~/.config/vibeusage/credentials/newprovider/oauth.json"),
        ))
        return await auth.is_available()

    async def fetch(self) -> FetchResult:
        auth_result = await self._authenticate()
        if not auth_result.success:
            return FetchResult.fail(auth_result.error)

        # Fetch usage data
        async with httpx.AsyncClient() as client:
            response = await client.get(
                "https://api.newprovider.com/usage",
                headers=auth_result.credentials.to_headers(),
            )
            response.raise_for_status()
            data = response.json()

        snapshot = self._parse_response(data)
        return FetchResult.ok(snapshot)

    def _parse_response(self, data: dict) -> UsageSnapshot:
        # Map provider-specific response to UsageSnapshot
        ...
```

### 3. Add Configuration

Add credential paths to spec 06 configuration.

### 4. Add Error Messages

Add provider-specific error messages to `PROVIDER_ERROR_MESSAGES`.

### 5. Add Tests

```python
# tests/providers/test_newprovider.py
import pytest
from vibeusage.providers import get_provider

def test_newprovider_metadata():
    provider = get_provider("newprovider")
    assert provider.metadata.id == "newprovider"
    assert provider.metadata.name == "New Provider"

@pytest.mark.asyncio
async def test_newprovider_fetch_strategies():
    provider = get_provider("newprovider")
    strategies = provider.fetch_strategies()
    assert len(strategies) >= 1
    assert strategies[0].name == "oauth"
```

---

## Implementation Checklist

### Phase 1: Claude (MVP)

- [ ] `vibeusage/providers/claude/__init__.py` - ClaudeProvider
- [ ] `vibeusage/providers/claude/oauth.py` - OAuth strategy
- [ ] `vibeusage/providers/claude/web.py` - Web/session strategy
- [ ] `vibeusage/providers/claude/cli.py` - CLI strategy
- [ ] Organization ID caching
- [ ] Overage parsing
- [ ] Status polling

### Phase 2: Codex

- [ ] `vibeusage/providers/codex/__init__.py` - CodexProvider
- [ ] `vibeusage/providers/codex/oauth.py` - OAuth strategy
- [ ] Config.toml endpoint override
- [ ] Credits display

### Phase 3: Copilot

- [ ] `vibeusage/providers/copilot/__init__.py` - CopilotProvider
- [ ] `vibeusage/providers/copilot/device_flow.py` - Device flow OAuth
- [ ] Interactive authorization prompt
- [ ] Premium vs chat quota separation

### Phase 4: Cursor

- [ ] `vibeusage/providers/cursor/__init__.py` - CursorProvider
- [ ] `vibeusage/providers/cursor/web.py` - Cookie-based strategy
- [ ] Browser cookie extraction
- [ ] On-demand spend (overage)

### Phase 5: Gemini

- [ ] `vibeusage/providers/gemini/__init__.py` - GeminiProvider
- [ ] `vibeusage/providers/gemini/oauth.py` - OAuth strategy
- [ ] Per-model quota parsing
- [ ] Tier detection

---

## Open Questions

1. **Browser cookie extraction**: Should we implement cookie extraction or require manual paste? Extraction is complex but much better UX.

2. **CLI detection**: Should we auto-detect installed CLIs and adjust fetch strategies accordingly?

3. **Rate limiting**: Should we implement per-provider rate limiting to avoid getting blocked?

4. **Caching**: Should provider responses be cached to reduce API calls during development?

5. **Mock mode**: Should we support a mock/demo mode for testing without real credentials?

## Implementation Notes

- Each provider module should be independently importable
- Use `httpx.AsyncClient` with connection pooling for efficiency
- Parse all timestamps as timezone-aware datetimes
- Handle missing/null fields gracefully (providers change APIs frequently)
- Log all API requests at DEBUG level for troubleshooting
- Consider retry with exponential backoff for transient failures
