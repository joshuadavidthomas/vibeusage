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

## Kimi / Moonshot

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

Check this path in `IsAvailable()` before falling back to our own credential store. Note `expires_at` is a Unix timestamp float — if expired, refresh using the refresh token and save back to our store.

The kimi-cli share dir varies by OS:
- Linux: `~/.local/share/kimi-cli/`
- macOS: `~/Library/Application Support/kimi-cli/`

#### 3. API key fallback

Users can also paste an API key from `https://www.kimi.com/code/console`. Simpler but no auto-refresh.

### API endpoint

```
GET https://api.kimi.com/coding/v1/usages
Authorization: Bearer <access_token or api_key>
```

Base URL overridable via `KIMI_CODE_BASE_URL` env var.

**Response format** (from [kimi-cli `usage.py`](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/ui/shell/usage.py)):
```json
{
  "usage": {
    "used": 54,
    "limit": 100,
    "remaining": 46,
    "name": "Weekly limit",
    "reset_at": "2026-01-26T13:59:00Z"
  },
  "limits": [
    {
      "name": "5h limit",
      "detail": {
        "used": 37,
        "limit": 100,
        "remaining": 63,
        "reset_at": "2026-02-20T16:59:00Z"
      },
      "window": {
        "duration": 300,
        "timeUnit": "MINUTE"
      }
    }
  ]
}
```

**Field name variants** (kimi-cli handles both):
- Reset time: `reset_at` / `resetAt` / `reset_time` / `resetTime`
- Usage: `used` and/or `remaining` (compute `used = limit - remaining`)
- Labels: `name` / `title` / `scope`

Use `json.RawMessage` or Go struct tags with both snake_case and camelCase? Simplest: define struct with `json:"reset_at"` and also try `resetAt` manually if the first is empty. Or just use the snake_case version since that's what the response actually uses (camelCase is a defensive fallback).

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
| `internal/config/credentials.go` | `ProviderCLIPaths`: `"kimi": {"~/.local/share/kimi-cli/credentials/kimi-code.json"}` |
| `internal/config/credentials.go` | `ProviderEnvVars`: `"kimi": "KIMI_CODE_API_KEY"` |

#### Status monitoring

No known status page. Return `StatusUnknown`.

### Open questions

- [ ] Verify response format with real device flow token
- [ ] Does the API return subscription tier info? (kimi-cli doesn't seem to fetch it)
- [ ] What does the response look like for a free vs paid account?
- [ ] Does `expires_at` in the token need to be checked before each request? kimi-cli has a background refresh loop — we should refresh inline if expired.

## Z.ai (Zhipu AI)

Chinese AI provider offering GLM model access via subscription coding plans. Two auth approaches: API key for the quota endpoint, or web session cookie for the dashboard API.

### What to track

- **Token quota**: `TOKENS_LIMIT` type, reported as `percentage` (maps to `Utilization`)
- **MCP usage**: `TIME_LIMIT` type with `currentValue`, `usage`, `usageDetails`
- Period types: `PeriodSession` for token quota (5h window), `PeriodMonthly` for MCP
- Subscription tier: Lite / Pro / Max

### Auth strategies (2 tiers)

#### 1. API key (primary)

From [glm-plan-usage plugin](https://github.com/zai-org/zai-coding-plugins/blob/main/plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs):

```
Authorization: <api_key>
Accept-Language: en-US,en
```

API key from: `https://z.ai/manage-apikey/apikey-list`

Two equivalent base URLs:
- International: `https://api.z.ai`
- China: `https://open.bigmodel.cn`

#### 2. Web session cookie (secondary)

The Z.ai dashboard at `https://z.ai` is a Next.js app. When logged in, the browser holds a session cookie that authenticates requests to the same API endpoints. This lets users skip API key creation — just grab the cookie from DevTools.

**TODO**: Need to identify the session cookie name. The dashboard sets `acw_tc` (generic Alibaba Cloud WAF cookie) and `NEXT_LOCALE`, but the actual auth cookie only appears after login. User needs to check DevTools → Application → Cookies after logging in.

Likely candidates:
- A JWT or session token cookie (common in Next.js apps)
- Could be the same `Authorization` header value stored as a cookie
- May use `next-auth` session pattern (`__Secure-next-auth.session-token`)

### API endpoints

**Quota limit** (primary):
```
GET /api/monitor/usage/quota/limit
→ { "data": { "limits": [
      { "type": "TOKENS_LIMIT", "percentage": 42 },
      { "type": "TIME_LIMIT", "percentage": 15, "currentValue": 180, "usage": 1200 }
    ] } }
```

**Model usage** (optional, for per-model breakdown):
```
GET /api/monitor/usage/model-usage?startTime=...&endTime=...
```

**Tool usage** (optional):
```
GET /api/monitor/usage/tool-usage?startTime=...&endTime=...
```

Unauthenticated requests return: `{"code":1001,"msg":"Authentication parameter not received in Header","success":false}`

### Implementation

#### New files

- `internal/provider/zai/zai.go` — Provider, metadata, APIKeyStrategy, WebStrategy
- `internal/provider/zai/response.go` — Response types for quota limits

#### Wiring

| File | Change |
|---|---|
| `cmd/root.go` | Blank import + add `"zai"` to provider list |
| `cmd/auth.go` | `case "zai": return authZai()` (prompt for API key or session cookie) |
| `cmd/key.go` | Add `"zai"` to key loop + `"zai": "api_key"` to `credentialKeyMap` |
| `internal/config/credentials.go` | `ProviderEnvVars`: `"zai": "ZAI_API_KEY"` |

#### Status monitoring

No known status page. Return `StatusUnknown`.

### Open questions

- [ ] What is the session cookie name on z.ai after login? (user needs to check DevTools)
- [ ] Does the quota endpoint return reset times, or just percentages?
- [ ] Is `percentage` 0-100 or 0.0-1.0?
- [ ] Is there a way to detect subscription tier from the API?
- [ ] Does the session cookie work with the same `/api/monitor/usage/quota/limit` endpoint, or does the dashboard hit a different internal endpoint?

## Minimax

Chinese AI provider with M2.5 model. Subscription plans with 5-hour refresh cycles. Has a documented REST API endpoint for quota checking.

### What to track

- Remaining prompts in current 5-hour window (convert to utilization percentage)
- Plan tier: Starter ($10) / Plus ($20) / Max ($50) / Highspeed variants
- Period type: `PeriodSession` (5-hour rolling window)
- 1 prompt ≈ 15 model calls

### Auth strategies (2 tiers)

#### 1. API key (primary)

From [Minimax FAQ](https://platform.minimax.io/docs/coding-plan/faq):

```
GET https://www.minimax.io/v1/api/openplatform/coding_plan/remains
Authorization: Bearer <CODING_PLAN_API_KEY>
Content-Type: application/json
```

Note: Minimax has separate API keys for coding plans vs pay-as-you-go. Only the Coding Plan key works for quota tracking. Key from: `https://platform.minimax.io/user-center/payment/coding-plan`

#### 2. Web session cookie (secondary)

The Minimax dashboard at `https://platform.minimax.io` shows coding plan usage. Like Z.ai, the browser holds a session cookie after login that could authenticate quota requests.

**TODO**: Need to identify the session cookie name. User needs to check DevTools → Application → Cookies on the Minimax dashboard after logging in.

### API endpoint

```
GET https://www.minimax.io/v1/api/openplatform/coding_plan/remains
Authorization: Bearer <api_key_or_session_token>
```

**Response format**: Not documented. Expected to contain remaining prompts, possibly with reset time and plan info. Need to make a real request to determine structure.

### Implementation

#### New files

- `internal/provider/minimax/minimax.go` — Provider, metadata, APIKeyStrategy, WebStrategy
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

### Open questions

- [ ] What is the exact response format from `/coding_plan/remains`?
- [ ] Does it return remaining count, total, percentage, or all?
- [ ] Does it include reset time?
- [ ] Is plan tier info included?
- [ ] What is the session cookie name on platform.minimax.io? (user needs to check DevTools)

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

### Phase 2: Kimi

Best-documented remaining provider. Device flow auth is proven (Copilot uses same pattern). Open-source CLI gives us the full OAuth spec, usage endpoint, and response parser.

1. Create `internal/provider/kimi/` with device flow + API key strategies
2. Implement device flow in `cmd/auth.go` (model after `authCopilot`)
3. Add credential reuse from kimi-cli installation
4. Parse usage response with window-to-period mapping
5. Wire into CLI
6. Test with real device flow

### Phase 3: Z.ai

Simple API key auth with well-understood endpoint. Web strategy depends on identifying the session cookie.

1. Create `internal/provider/zai/` with API key strategy
2. Parse quota/limit response (percentage-based)
3. Wire into CLI
4. Test with real API key
5. **After cookie identified**: Add web strategy

### Phase 4: Minimax

Simple API key auth but unknown response format. Web strategy depends on identifying the session cookie.

1. Make a real request to `/coding_plan/remains` to determine response format
2. Create `internal/provider/minimax/` with API key strategy
3. Wire into CLI
4. Test with real API key
5. **After cookie identified**: Add web strategy

### Phase 5: Kiro

Blocked on OAuth details. Implement only when we have real endpoint info.

### Phase 6: Polish

1. Extract shared Google OAuth between Gemini and Antigravity (if duplication hurts)
2. Extract shared device flow helper between Copilot and Kimi (if duplication hurts)
3. Update README with new provider docs
