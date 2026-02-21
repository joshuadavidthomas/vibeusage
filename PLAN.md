# Plan: Add New Providers

Reference implementations:
- [jlcodes99/cockpit-tools](https://github.com/jlcodes99/cockpit-tools) (Antigravity, Kiro)
- [zai-org/zai-coding-plugins](https://github.com/zai-org/zai-coding-plugins) (Z.ai quota API)
- [MoonshotAI/kimi-cli](https://github.com/MoonshotAI/kimi-cli) (Kimi device flow + usage API)

## Antigravity (Google's AI IDE) — ✅ DONE

Implemented on branch `add-antigravity-provider`.

### What was built

- **OAuth strategy** reading credentials from Antigravity's vscdb (`state.vscdb` via `sqlite3` CLI)
- **Quota endpoint**: `POST https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels` with empty `{}` body and `User-Agent: antigravity`
- **Tier detection via `loadCodeAssist`**: `POST /v1internal:loadCodeAssist` with `ideType: "ANTIGRAVITY"` metadata — but this returns the product name ("Antigravity"), not the subscription tier
- **Subscription parsing from protobuf**: The real subscription (e.g. "Google AI Pro") lives in field 36 of the protobuf blob in the vscdb auth status. Parsed with `google.golang.org/protobuf/encoding/protowire`.
- **Period type inference**: Pro/Ultra → `PeriodSession` (5h windows), Free → `PeriodWeekly`
- **Summary period**: Compact panel view shows highest utilization across all models; expanded view shows per-model breakdown
- **XDG-compliant paths**: Uses `os.UserConfigDir()` instead of hardcoded `.config`
- **Status monitoring**: Reuses Google Apps Status (same infra as Gemini)

### Key learnings

- `fetchAvailableModels` (not `retrieveUserQuota`) is the right endpoint — returns per-model `quotaInfo` with `remainingFraction` and `resetTime`
- The `loadCodeAssist` tier info is misleading — it shows the product tier, not the Google One subscription
- Protobuf field 36 in vscdb `userStatusProtoBinaryBase64` contains the real subscription: `{tierID: "g1-pro-tier", tierName: "Google AI Pro"}`
- Models without `resetTime` (tab completion, internal) should be filtered out

## Kimi / Moonshot — ✅ DONE

Implemented on branch `add-kimi-provider`.

Chinese AI provider with K2.5 model. KimiCode is an open-source CLI agent. The kimi-cli source code gives us the full auth flow and usage API.

### What to track

- **Overall usage summary**: `used` / `limit` with reset time
- **Per-window limits**: Array of limit entries with `used`, `limit`, `remaining`, `window` (duration + timeUnit)
- Reset times: ISO 8601 timestamps
- Period types derived from `window.duration` + `timeUnit`:
  - 300 MINUTE (5h) → `PeriodSession`
  - 7 DAY → `PeriodWeekly`
  - 24 HOUR → `PeriodDaily`

### Auth strategies (3 tiers)

#### 1. Device flow OAuth (primary — like Copilot)

From [kimi-cli `auth/oauth.py`](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/auth/oauth.py):

```
Client ID:  17e5f671-d194-4dfb-9706-5516cb48c098
OAuth host: https://auth.kimi.com  (override: KIMI_CODE_OAUTH_HOST)

POST /api/oauth/device_authorization
  body: client_id=<client_id>
  → { user_code, device_code, verification_uri_complete, interval, expires_in }

POST /api/oauth/token  (poll)
  body: client_id=<client_id>&device_code=<code>&grant_type=urn:ietf:params:oauth:grant-type:device_code
  → { access_token, refresh_token, expires_in, scope, token_type }

POST /api/oauth/token  (refresh)
  body: client_id=<client_id>&grant_type=refresh_token&refresh_token=<token>
  → same shape
```

Headers to include (from kimi-cli `_common_headers`):
```
X-Msh-Platform: vibeusage
X-Msh-Version: <version>
X-Msh-Device-Name: <hostname>
X-Msh-Device-Model: <os arch>
X-Msh-Os-Version: <os version>
X-Msh-Device-Id: <persistent uuid>
```

This is almost identical to the Copilot device flow pattern. Implementation in `cmd/auth.go`:
```go
case "kimi":
    return authKimi()
```

`authKimi()` follows the same pattern as `authCopilot()`:
1. POST to device authorization endpoint
2. Print user code and verification URL
3. Open browser
4. Poll token endpoint at `interval` seconds
5. Save tokens to `~/.config/vibeusage/credentials/kimi/oauth.json`

#### 2. Credential reuse from kimi-cli

kimi-cli stores OAuth tokens at `~/.local/share/kimi-cli/credentials/kimi-code.json`:
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_at": 1740000000.0,
  "scope": "...",
  "token_type": "bearer"
}
```

Check this path in `IsAvailable()` before falling back to our own credential store. Note `expires_at` is a Unix timestamp float — tokens expire in ~15 minutes. If expired, refresh using the refresh token and save back to our store.

The kimi-cli share dir is `~/.kimi/` (hardcoded as `Path.home() / ".kimi"`, overridable via `KIMI_SHARE_DIR` env var).

#### 3. API key fallback

Users can also paste an API key from `https://www.kimi.com/code/console`. Simpler but no auto-refresh.

### API endpoint

```
GET https://api.kimi.com/coding/v1/usages
Authorization: Bearer <access_token or api_key>
```

Base URL overridable via `KIMI_CODE_BASE_URL` env var.

**Verified response** (real device flow token, free tier):
```json
{
  "user": {
    "userId": "d5s8j1fftae5hmncss20",
    "region": "REGION_OVERSEA",
    "membership": {
      "level": "LEVEL_BASIC"
    },
    "businessId": ""
  },
  "usage": {
    "limit": "100",
    "remaining": "100",
    "resetTime": "2026-02-25T04:01:38Z"
  },
  "limits": [
    {
      "window": {
        "duration": 300,
        "timeUnit": "TIME_UNIT_MINUTE"
      },
      "detail": {
        "limit": "100",
        "remaining": "100",
        "resetTime": "2026-02-21T08:01:38Z"
      }
    }
  ]
}
```

**Field details**:
- `user.membership.level`: plan tier (`"LEVEL_BASIC"` = free; likely `"LEVEL_PRO"` etc for paid)
- `user.region`: `"REGION_OVERSEA"` — may affect display name
- `usage`: overall summary — `limit` and `remaining` are **strings** (not ints!), `resetTime` is ISO 8601
- `limits[].window`: `duration: 300, timeUnit: "TIME_UNIT_MINUTE"` → 5 hours → `PeriodSession`
- `limits[].detail`: same shape as `usage` but for the specific window
- No `used` field — compute as `limit - remaining`
- No `name` field on limits — derive from window (e.g. "5h limit", "Weekly")
- Reset times are camelCase `resetTime` (not snake_case `reset_at` as kimi-cli's parser suggested)

**Mapping to vibeusage periods**:
- `usage` → summary period: `PeriodWeekly` (reset ~4 days out)
- `limits[0]` with `duration: 300, TIME_UNIT_MINUTE` → `PeriodSession` (5h window)

### Implementation

#### New files

- `internal/provider/kimi/kimi.go` — Provider, metadata, DeviceFlowStrategy, APIKeyStrategy
- `internal/provider/kimi/response.go` — Response types, window-to-period mapping
- `internal/provider/kimi/response_test.go` — Parse tests with fixtures
- `internal/provider/kimi/device_flow.go` — Device flow auth (extract to shared if Kiro needs it later)

#### Wiring

| File | Change |
|---|---|
| `cmd/root.go` | Blank import + add `"kimi"` to provider list |
| `cmd/auth.go` | `case "kimi": return authKimi()` (device flow, like Copilot) |
| `cmd/key.go` | Add `"kimi"` to key loop + `"kimi": "api_key"` to `credentialKeyMap` |
| `internal/config/credentials.go` | `ProviderCLIPaths`: `"kimi": {"~/.kimi/credentials/kimi-code.json"}` |
| `internal/config/credentials.go` | `ProviderEnvVars`: `"kimi": "KIMI_CODE_API_KEY"` |

#### Status monitoring

No known status page. Return `StatusUnknown`.

### Verified

- ✅ Response format identical for both device flow tokens and API keys (`sk-kimi-...`)
- ✅ camelCase fields, `limit`/`remaining` are strings
- ✅ Subscription tier: `user.membership.level` (`"LEVEL_BASIC"` for free)
- ✅ Credential path: `~/.kimi/credentials/kimi-code.json` (not `~/.local/share/kimi-cli/`)
- ✅ Token lifetime: ~15 minutes (`expires_at` is Unix float), needs inline refresh
- ✅ API keys work with the same endpoint — no functional difference in response

### Open questions

- [ ] What does `membership.level` look like for paid tiers? (`"LEVEL_PRO"`? `"LEVEL_PREMIUM"`?)

## Z.ai (Zhipu AI)

Chinese AI provider offering GLM model access via subscription coding plans. Two auth approaches: API key for the quota endpoint, or web session cookie for the dashboard API.

### What to track

- **Token quota**: `TOKENS_LIMIT` type, reported as `percentage` (maps to `Utilization`)
- **MCP usage**: `TIME_LIMIT` type with `currentValue`, `usage`, `usageDetails`
- Period types: `PeriodSession` for token quota (5h window), `PeriodMonthly` for MCP
- Subscription tier: Lite / Pro / Max

### Auth: Bearer token (single strategy)

Both API keys and web session tokens use the same mechanism: `Authorization: Bearer <token>`.

#### API key

From [glm-plan-usage plugin](https://github.com/zai-org/zai-coding-plugins/blob/main/plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs):

```
Authorization: <api_key>
Accept-Language: en-US,en
```

API key from: `https://z.ai/manage-apikey/apikey-list`

Two equivalent base URLs:
- International: `https://api.z.ai`
- China: `https://open.bigmodel.cn`

#### Dashboard auth (investigated, not worth it)

Z.ai uses no auth cookies. The dashboard stores a JWT in localStorage (`z-ai-open-platform-token-production`) which works as a Bearer token. But since there's no way to read browser localStorage programmatically, the user would have to open DevTools and paste it manually — at that point, creating a dedicated API key is simpler and more stable (the JWT may have a server-side TTL despite having no `exp` claim).

For `vibeusage auth zai`, just prompt for an API key with instructions to create one at `https://z.ai/manage-apikey/apikey-list`.

### API endpoints

**Quota limit** (primary):
```
GET /api/monitor/usage/quota/limit
Authorization: <api_key_or_jwt>
```

**Verified response** (real API key, Pro tier):
```json
{
  "code": 200,
  "msg": "Operation successful",
  "data": {
    "limits": [
      {
        "type": "TOKENS_LIMIT",
        "unit": 3,
        "number": 5,
        "percentage": 1,
        "nextResetTime": 1771661559241
      },
      {
        "type": "TIME_LIMIT",
        "unit": 5,
        "number": 1,
        "usage": 1000,
        "currentValue": 0,
        "remaining": 1000,
        "percentage": 0,
        "nextResetTime": 1773596236985,
        "usageDetails": [
          { "modelCode": "search-prime", "usage": 0 },
          { "modelCode": "web-reader", "usage": 33 },
          { "modelCode": "zread", "usage": 0 }
        ]
      }
    ],
    "level": "pro"
  },
  "success": true
}
```

**Field details**:
- `percentage`: 0-100 integer, maps directly to `Utilization`
- `nextResetTime`: Unix timestamp in **milliseconds** (divide by 1000 for Go `time.Unix`)
- `unit` + `number`: encodes the window duration
  - `unit: 3, number: 5` → 5 hours → `PeriodSession`
  - `unit: 5, number: 1` → 1 month → `PeriodMonthly`
- `level`: subscription tier (`"pro"`, likely also `"lite"`, `"max"`)
- `TOKENS_LIMIT`: token quota for the 5-hour window (the main metric)
- `TIME_LIMIT`: monthly MCP tool usage with per-tool breakdown in `usageDetails`

**Other endpoints** (optional, for per-model breakdown):
```
GET /api/monitor/usage/model-usage?startTime=...&endTime=...
GET /api/monitor/usage/tool-usage?startTime=...&endTime=...
```

Unauthenticated requests return: `{"code":1001,"msg":"Authentication parameter not received in Header","success":false}`

### Implementation

#### New files

- `internal/provider/zai/zai.go` — Provider, metadata, single BearerTokenStrategy
- `internal/provider/zai/response.go` — Response types for quota limits

#### Wiring

| File | Change |
|---|---|
| `cmd/root.go` | Blank import + add `"zai"` to provider list |
| `cmd/auth.go` | `case "zai": return authZai()` (prompt for API key or localStorage JWT) |
| `cmd/key.go` | Add `"zai"` to key loop + `"zai": "api_key"` to `credentialKeyMap` |
| `internal/config/credentials.go` | `ProviderEnvVars`: `"zai": "ZAI_API_KEY"` |

#### Status monitoring

No known status page. Return `StatusUnknown`.

### Resolved questions

- ✅ `percentage` is 0-100 integer
- ✅ Reset times: `nextResetTime` field, Unix millis
- ✅ Subscription tier: `data.level` field (`"pro"`)
- ✅ Window encoding: `unit` + `number` (unit 3 = hours, unit 5 = months)

### Open questions

- [ ] What are all possible `unit` values? (confirmed: 3=hours, 5=months — what are 1, 2, 4?)

## Minimax

Chinese AI provider with M2.5 model. Subscription plans with 5-hour refresh cycles. Has a documented REST API endpoint for quota checking.

### What to track

- Remaining prompts in current 5-hour window (convert to utilization percentage)
- Plan tier: Starter ($10) / Plus ($20) / Max ($50) / Highspeed variants
- Period type: `PeriodSession` (5-hour rolling window)
- 1 prompt ≈ 15 model calls

### Auth strategy: API key

Minimax has separate API keys for coding plans vs pay-as-you-go. Only the **Coding Plan key** (prefix `sk-cp-`) works for quota tracking. Standard API keys (`sk-api-...`) return `{"status_code":1004,"status_msg":"cookie is missing, log in again"}`.

Key from: `https://platform.minimax.io/user-center/payment/coding-plan`

During `vibeusage auth minimax`, validate that the key starts with `sk-cp-`.

### API endpoint

```
GET https://platform.minimax.io/v1/api/openplatform/coding_plan/remains
Authorization: Bearer <coding_plan_api_key>
Content-Type: application/json
User-Agent: Mozilla/5.0 ...  (required — Cloudflare 1010 bot block without it)
Referer: https://platform.minimax.io/
```

Note: the host is `platform.minimax.io` (not `www.minimax.io`). Requires a browser-like `User-Agent` to pass Cloudflare bot detection. The dashboard sends a `GroupId` query param but it's **not required** — the API key already identifies the account.

**Verified response** (real coding plan key, Plus tier):
```json
{
  "model_remains": [
    {
      "start_time": 1771650000000,
      "end_time": 1771668000000,
      "remains_time": 8196068,
      "current_interval_total_count": 1500,
      "current_interval_usage_count": 1500,
      "model_name": "MiniMax-M2"
    },
    {
      "start_time": 1771650000000,
      "end_time": 1771668000000,
      "remains_time": 8196068,
      "current_interval_total_count": 1500,
      "current_interval_usage_count": 1500,
      "model_name": "MiniMax-M2.1"
    },
    {
      "start_time": 1771650000000,
      "end_time": 1771668000000,
      "remains_time": 8196068,
      "current_interval_total_count": 1500,
      "current_interval_usage_count": 1500,
      "model_name": "MiniMax-M2.5"
    }
  ],
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

**Field details**:
- `model_remains[]`: per-model quota info
- `start_time` / `end_time`: Unix millis, defines the 5-hour window (05:00–10:00 UTC)
- `remains_time`: milliseconds remaining in the window (for countdown, not quota)
- `current_interval_total_count`: total prompts in window (1500 = Plus tier)
- `current_interval_usage_count`: prompts used (1500/1500 = 100% used)
- `model_name`: model identifier
- Utilization: `usage_count / total_count * 100`
- Reset time: `end_time` (Unix millis)
- No plan tier field in response — infer from `total_count` (500=Starter, 1500=Plus, 5000=Max?)

### Implementation

#### New files

- `internal/provider/minimax/minimax.go` — Provider, metadata, APIKeyStrategy
- `internal/provider/minimax/response.go` — Response types (TBD from real API response)
- `internal/provider/minimax/response_test.go` — Parse tests once format is known

#### Wiring

| File | Change |
|---|---|
| `cmd/root.go` | Blank import + add `"minimax"` to provider list |
| `cmd/auth.go` | `case "minimax": return authMinimax()` (prompt for API key or session cookie) |
| `cmd/key.go` | Add `"minimax"` to key loop + `"minimax": "api_key"` to `credentialKeyMap` |
| `internal/config/credentials.go` | `ProviderEnvVars`: `"minimax": "MINIMAX_API_KEY"` |

#### Status monitoring

No known status page. Return `StatusUnknown`.

### Resolved questions

- ✅ Response format: per-model array with `total_count`, `usage_count`, `start_time`, `end_time`
- ✅ Counts not percentages — compute utilization as `usage_count / total_count * 100`
- ✅ Reset time: `end_time` field (Unix millis)
- ✅ Host is `platform.minimax.io` not `www.minimax.io`
- ✅ Requires browser User-Agent (Cloudflare bot protection)
- ✅ `GroupId` query param is optional — API key identifies the account

### Open questions

- [ ] What are the `total_count` values per tier? (confirmed 1500 for Plus)
- [ ] Is plan tier info available from any other endpoint?


## Kiro (AWS)

AWS's AI coding IDE. Most complex auth (PKCE) and most unstable API (7+ fallback paths per metric). **Lowest priority** — blocked on missing OAuth endpoint details.

### What to track

- Prompt credits: used / total → utilization percentage
- Bonus credits: used / total (separate period or `OverageUsage`)
- Bonus/trial expiry
- Plan name, usage reset time
- Period type: `PeriodMonthly`

### Auth strategy: OAuth 2.0 + PKCE

From cockpit-tools — needs a local HTTP callback server:

1. Generate `state`, `code_verifier`, `code_challenge` (SHA-256)
2. Start local TCP server for `/oauth/callback` and `/signin/callback`
3. Open browser to Kiro auth portal
4. Exchange code for tokens with `code_verifier`
5. Refresh via `refresh_token` grant

### API endpoint

```
GET /getUsageLimits
Authorization: Bearer <token>
```

Region-based routing: `https://q.{region}.amazonaws.com`

Response format is deeply nested and unstable. cockpit-tools uses 7+ fallback JSON paths per metric. Would use `json.RawMessage` + sequential path resolution in Go.

### Open questions (blockers)

- [ ] Exact OAuth endpoints (auth portal URL, token URL, redirect URI)
- [ ] How to detect user's region
- [ ] Real `/getUsageLimits` response sample
- [ ] Credential file path on Linux

## Implementation Order

### Phase 1: Antigravity — ✅ DONE

Branch `add-antigravity-provider`, 5 commits.

### Phase 2: Kimi — ✅ DONE

Branch `add-kimi-provider`, 1 commit.

- DeviceFlowStrategy + APIKeyStrategy
- Credential reuse from kimi-cli (~/.kimi/credentials/kimi-code.json)
- Device flow auth in cmd/auth.go (same pattern as Copilot)
- Window-to-period mapping (300 MINUTE → PeriodSession, etc.)
- Plan tier from user.membership.level
- Tested with real device flow token

### Phase 3: Z.ai — ✅ DONE

Branch `add-zai-provider`, 1 commit.

- BearerTokenStrategy for API keys and JWT tokens
- Quota endpoint: `GET /api/monitor/usage/quota/limit` with `Accept-Language: en-US,en`
- TOKENS_LIMIT (5h session) and TIME_LIMIT (monthly MCP) period parsing
- Percentage-based utilization (0-100, clamped)
- Reset times from Unix millisecond timestamps
- Plan tier from `data.level` (lite/pro/max)
- Auth flow prompts for API key from `z.ai/manage-apikey/apikey-list`
- Handles auth error code 1001 as fatal

### Phase 4: Minimax

API key auth, per-model response format verified. No blockers.

1. Create `internal/provider/minimax/` with API key strategy
2. Wire into CLI
3. Test with real API key

### Phase 5: Kiro

Blocked on OAuth details. Implement only when we have real endpoint info.

### Phase 6: Polish

1. Extract shared Google OAuth between Gemini and Antigravity (if duplication hurts)
2. Extract shared device flow helper between Copilot and Kimi (if duplication hurts)
3. Update README with new provider docs
