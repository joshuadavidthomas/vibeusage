# Plan: Add New Providers

Reference implementations:
- [jlcodes99/cockpit-tools](https://github.com/jlcodes99/cockpit-tools) (Antigravity, Kiro)
- [zai-org/zai-coding-plugins](https://github.com/zai-org/zai-coding-plugins) (Z.ai quota API)
- [MoonshotAI/kimi-cli](https://github.com/MoonshotAI/kimi-cli) (Kimi usage API)

## Antigravity (Google's AI IDE)

Google's flagship AI IDE, launched Nov 2025 alongside Gemini 3. Built on the Windsurf acquisition ($2.4B). Uses the same Google OAuth infrastructure as Gemini, making this the easier of the two providers to add.

### What to track

- Per-model quota as `remainingFraction` (percentage-based, maps directly to `UsagePeriod.Utilization`)
- Reset times per model
- Subscription tier: FREE / PRO / ULTRA (maps to `ProviderIdentity.Plan`)
- Period type: `PeriodSession` for Pro/Ultra (5-hour refresh), `PeriodWeekly` for Free tier

### Auth strategy: Google OAuth (reuse Gemini)

Antigravity uses the same Google OAuth2 flow as Gemini but with different client credentials and scopes.

**Client credentials** (from cockpit-tools `src-tauri/src/modules/oauth.rs`):
- Client ID: `1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com`
- Client Secret: `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`
- Token URL: `https://oauth2.googleapis.com/token` (same as Gemini)

**Scopes**:
- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`
- `https://www.googleapis.com/auth/userinfo.profile`
- `https://www.googleapis.com/auth/cclog`
- `https://www.googleapis.com/auth/experimentsandconfigs`

**Credential reuse**: Antigravity stores credentials at `~/.config/Antigravity/` (Linux), similar to how Gemini CLI stores at `~/.gemini/`. Check if Antigravity stores an OAuth token file we can read directly.

### API endpoints

**Quota endpoint** (from cockpit-tools `src-tauri/src/modules/quota.rs`):
- Production: `https://cloudcode-pa.googleapis.com/...` (same base as Gemini's `quotaURL`)
- Request metadata must include `ideType: "ANTIGRAVITY"` and `pluginType: "GEMINI"`

```json
{
  "metadata": {
    "ideType": "ANTIGRAVITY",
    "platform": "PLATFORM_UNSPECIFIED",
    "pluginType": "GEMINI"
  }
}
```

**Response format**: Same `QuotaResponse` shape as Gemini — array of `QuotaBucket` with `model_id`, `remaining_fraction`, `reset_time`. The existing `gemini/response.go` types should work as-is.

### Implementation

#### New files

- `internal/provider/antigravity/antigravity.go` — Provider struct, metadata, strategy list, status
- `internal/provider/antigravity/response.go` — Response types (likely reuse/alias Gemini types if identical, or define separately if the quota request metadata differs enough)

#### Approach

The Gemini provider already calls `cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota` with an empty `{}` body. Antigravity uses the same endpoint but with a metadata payload specifying `ideType: "ANTIGRAVITY"`. Two options:

1. **Separate provider with shared types** (preferred) — New `antigravity` package that imports Gemini's response types but has its own OAuth credentials, quota request payload, and credential paths. Keeps providers independently testable.
2. **Shared strategy code** — Extract common Google OAuth + quota logic into a shared package. More DRY but adds coupling. Consider this as a follow-up refactor if the duplication becomes painful.

#### Wiring changes

| File | Change |
|---|---|
| `cmd/root.go:23-28` | Add `_ "github.com/joshuadavidthomas/vibeusage/internal/provider/antigravity"` blank import |
| `cmd/root.go:69` | Add `"antigravity"` to the provider ID list |
| `cmd/key.go:26` | Add `"antigravity"` to the key command loop |
| `cmd/key.go:91-97` | Add `"antigravity": "access_token"` to `credentialKeyMap` |
| `cmd/auth.go:34-42` | Falls through to `authGeneric` (fine for now, could add custom flow later) |
| `internal/config/credentials.go:12-17` | Add `"antigravity": {"~/.config/Antigravity/credentials.json"}` to `ProviderCLIPaths` (verify actual path) |
| `internal/config/credentials.go:21-26` | Add `"antigravity": "GOOGLE_APPLICATION_CREDENTIALS"` or similar to `ProviderEnvVars` (if applicable) |

#### Status monitoring

Use the same Google Apps Status approach as Gemini (`fetchGeminiStatus`), or extract it into a shared helper since both providers care about the same Google infrastructure incidents. Filter for Antigravity-specific keywords: `"antigravity"`, `"gemini"`, `"cloud code"`, `"generative ai"`.

### Open questions

- [ ] What is the exact Antigravity credential file path on Linux? cockpit-tools shows `~/.config/Antigravity/` — verify by installing Antigravity
- [ ] Does the quota endpoint URL differ from Gemini's, or is it the same URL with different metadata?
- [ ] Is the `ideType: "ANTIGRAVITY"` metadata required, or does it work with Gemini credentials and an empty body?
- [ ] Should we share/extract Google OAuth logic between Gemini and Antigravity?

## Kiro (AWS's AI IDE)

AWS's AI coding IDE. Uses OAuth 2.0 + PKCE with AWS-hosted endpoints. Credits-based quota model with bonus/free-trial tracking. The API response format is known to be unstable (cockpit-tools implements 7+ fallback paths per metric).

### What to track

- Prompt credits: used / total (convert to `UsagePeriod.Utilization` as percentage)
- Bonus credits: used / total (separate `UsagePeriod` or `OverageUsage`)
- Bonus/trial expiry: days remaining
- Plan name and tier
- Usage reset time
- Period type: `PeriodMonthly` (credits-based)

### Auth strategy: OAuth 2.0 + PKCE

**Endpoints** (from cockpit-tools `src-tauri/src/modules/kiro_oauth.rs`):
- Auth portal: Kiro-hosted, opens browser for login
- Token endpoint: POST with `code`, `code_verifier`, `redirect_uri`
- Refresh endpoint: POST with `refresh_token`
- Callback: Local HTTP server on `localhost` handling `/oauth/callback` and `/signin/callback`

**PKCE flow**:
1. Generate `state`, `code_verifier`, `code_challenge` (SHA-256)
2. Start local TCP server for callback
3. Open browser to auth portal with PKCE parameters
4. Handle callback — extract `code` from query params (handles both `snake_case` and `camelCase` parameter names)
5. Exchange code for tokens
6. Store tokens at `~/.config/vibeusage/credentials/kiro/oauth.json`

This is more complex than Gemini OAuth but similar in spirit to the Copilot device flow — both require an interactive step. The PKCE flow needs a local HTTP callback server.

### API endpoints

**Usage endpoint** (from cockpit-tools `src-tauri/src/modules/kiro_oauth.rs`):
- `GET /getUsageLimits` with `Authorization: Bearer <token>`
- Region-based endpoint routing:
  - `us-east-1` → `https://q.us-east-1.amazonaws.com`
  - `eu-central-1` → `https://q.eu-central-1.amazonaws.com`
  - etc.

**Response format**: Deeply nested JSON that varies between accounts/plans/API versions. cockpit-tools handles this with multi-path fallback resolution.

### Credit resolution (multi-path fallback)

The Kiro API response format has changed multiple times. cockpit-tools resolves each metric by trying multiple JSON paths in priority order. For vibeusage, implement this as a helper that tries paths sequentially:

**Prompt credits total** (7 paths):
1. Direct `credits_total` field
2. Sum of `usageBreakdowns[*].usageTotal`
3. `usageBreakdownList[0].usageLimitWithPrecision`
4. `usageBreakdownList[0].usageLimit`
5. `estimatedUsage.total`
6. `usageBreakdowns.plan.totalCredits`
7. Root `totalCredits`

**Prompt credits used** (similar set of fallback paths)

In Go, store the raw response as `json.RawMessage` and write helper functions that walk the paths using `json.Unmarshal` into intermediate maps/slices. This aligns with the existing pattern of typed structs + `json.RawMessage` for variable fields.

### Implementation

#### New files

- `internal/provider/kiro/kiro.go` — Provider struct, metadata, OAuth PKCE strategy
- `internal/provider/kiro/response.go` — Response types and multi-path credit resolution
- `internal/provider/kiro/response_test.go` — Test credit resolution with fixtures from multiple API response formats

#### PKCE auth flow

Add to `cmd/auth.go` as a new case:
```
case "kiro":
    return authKiro()
```

`authKiro()` would:
1. Start a local HTTP server on a random port
2. Generate PKCE parameters
3. Open the Kiro auth URL in the browser
4. Wait for callback with authorization code
5. Exchange code for tokens
6. Save tokens to `~/.config/vibeusage/credentials/kiro/oauth.json`

Consider extracting a reusable PKCE helper since this pattern could be useful for future providers.

#### Wiring changes

| File | Change |
|---|---|
| `cmd/root.go:23-28` | Add `_ "github.com/joshuadavidthomas/vibeusage/internal/provider/kiro"` blank import |
| `cmd/root.go:69` | Add `"kiro"` to the provider ID list |
| `cmd/auth.go:34-42` | Add `case "kiro": return authKiro()` |
| `cmd/key.go:26` | Add `"kiro"` to the key command loop |
| `cmd/key.go:91-97` | Add `"kiro": "access_token"` to `credentialKeyMap` |
| `internal/config/credentials.go:12-17` | Add `"kiro": {"~/.config/kiro/credentials.json"}` to `ProviderCLIPaths` (verify actual path) |
| `internal/config/credentials.go:21-26` | Add `"kiro": "KIRO_API_KEY"` to `ProviderEnvVars` (if applicable) |

#### Status monitoring

Kiro doesn't appear to have a public Statuspage.io page. Options:
- Check AWS health dashboard for relevant services
- Return `StatusUnknown` initially and add monitoring later
- Check if Kiro has a status endpoint (undocumented)

### Open questions

- [ ] What are the exact Kiro OAuth endpoints (auth portal URL, token URL, redirect URI)?
- [ ] What is the Kiro credential file path on Linux? cockpit-tools shows it's stored per-account — need to find single-user path
- [ ] What region does the user's account use, and how do we detect it? From the token response? From a config file?
- [ ] Does Kiro have a public status page?
- [ ] What does the raw `/getUsageLimits` response actually look like? cockpit-tools' 7 fallback paths suggest high variance — we need real response samples
- [ ] Should bonus credits map to a separate `UsagePeriod`, to `OverageUsage`, or to a new field on `UsageSnapshot`?

## Z.ai (Zhipu AI GLM Coding Plans)

Chinese AI provider offering GLM model access via subscription coding plans. Plans have 5-hour refresh cycles (like Claude) plus weekly limits. Auth is a simple API key/bearer token.

### What to track

- **5-hour token quota**: `TOKENS_LIMIT` type, reported as `percentage` (maps directly to `UsagePeriod.Utilization`)
- **Monthly MCP usage**: `TIME_LIMIT` type, with `currentValue`, `usage` (total), and `usageDetails`
- Period types: `PeriodSession` for 5-hour quota, `PeriodMonthly` for MCP usage
- Subscription tier: Lite ($10/mo) / Pro ($30/mo) / Max ($56/mo)

### Auth strategy: API key (bearer token)

Simple bearer token auth, same pattern as the existing Gemini API key strategy.

- API key managed at: `https://z.ai/manage-apikey/apikey-list`
- Token format: `Authorization: <api_key>` (note: the Z.ai plugin uses `ANTHROPIC_AUTH_TOKEN` env var since it's designed for Claude Code integration, but we'd use our own env var)

### API endpoints

Discovered from the [glm-plan-usage plugin](https://github.com/zai-org/zai-coding-plugins/blob/main/plugins/glm-plan-usage/skills/usage-query-skill/scripts/query-usage.mjs):

**Base URLs** (two platforms, same API):
- Z.ai: `https://api.z.ai`
- ZHIPU (Chinese): `https://open.bigmodel.cn`

**Endpoints**:

| Endpoint | Method | Query Params | Purpose |
|---|---|---|---|
| `/api/monitor/usage/quota/limit` | GET | None | **Primary** — Returns quota limits with percentages |
| `/api/monitor/usage/model-usage` | GET | `startTime`, `endTime` | Token usage breakdown by model |
| `/api/monitor/usage/tool-usage` | GET | `startTime`, `endTime` | Tool usage (web search, etc.) |

**Headers**:
```
Authorization: <api_key>
Accept-Language: en-US,en
Content-Type: application/json
```

**Quota limit response format**:
```json
{
  "data": {
    "limits": [
      {
        "type": "TOKENS_LIMIT",
        "percentage": 42
      },
      {
        "type": "TIME_LIMIT",
        "percentage": 15,
        "currentValue": 180,
        "usage": 1200,
        "usageDetails": { ... }
      }
    ]
  }
}
```

The `TOKENS_LIMIT` percentage maps directly to utilization (5-hour window). The `TIME_LIMIT` tracks monthly MCP tool usage.

**Time range params** (for model-usage and tool-usage):
- Format: `yyyy-MM-dd HH:mm:ss`
- Window: 24 hours (yesterday same hour to today same hour)

### Implementation

#### New files

- `internal/provider/zai/zai.go` — Provider struct, metadata, API key strategy
- `internal/provider/zai/response.go` — Response types for quota limit, model usage

#### Approach

The simplest provider to implement. Single strategy (API key), single primary endpoint (`/api/monitor/usage/quota/limit`), percentage-based response. Optionally also fetch model-usage for per-model breakdown.

#### Wiring changes

| File | Change |
|---|---|
| `cmd/root.go:23-28` | Add `_ "github.com/joshuadavidthomas/vibeusage/internal/provider/zai"` blank import |
| `cmd/root.go:69` | Add `"zai"` to the provider ID list |
| `cmd/key.go:26` | Add `"zai"` to the key command loop |
| `cmd/key.go:91-97` | Add `"zai": "api_key"` to `credentialKeyMap` |
| `cmd/auth.go` | Falls through to `authGeneric` |
| `internal/config/credentials.go` | Add `"zai": "ZAI_API_KEY"` to `ProviderEnvVars` |

#### Status monitoring

No known public status page. Return `StatusUnknown` initially.

### Open questions

- [ ] Does the quota/limit endpoint return reset times, or just percentages?
- [ ] Is the `percentage` field 0-100 or 0.0-1.0?
- [ ] Does model-usage return per-model quota data, or just historical token counts?
- [ ] Is there a way to detect the subscription tier from the API?

## Minimax (Coding Plans)

Chinese AI provider with M2.5 model (80.2% SWE-bench). Subscription plans with 5-hour refresh cycles. Has a documented REST API endpoint for quota checking.

### What to track

- Remaining prompts in current 5-hour window (convert to utilization percentage)
- Plan tier: Starter ($10) / Plus ($20) / Max ($50) / Highspeed variants
- Period type: `PeriodSession` (5-hour rolling window)
- 1 prompt ≈ 15 model calls

### Auth strategy: API key (bearer token)

Minimax uses separate API keys for coding plans vs pay-as-you-go:
- **Coding Plan API key**: From `https://platform.minimax.io/user-center/payment/coding-plan`
- **Standard API key**: Different key, for token-based billing

Only the Coding Plan API key works for quota tracking.

### API endpoints

**Quota endpoint** (from [Minimax FAQ](https://platform.minimax.io/docs/coding-plan/faq)):

```
GET https://www.minimax.io/v1/api/openplatform/coding_plan/remains
Authorization: Bearer <CODING_PLAN_API_KEY>
Content-Type: application/json
```

**Response format**: Not fully documented. Expected to contain remaining prompts, possibly with reset time information. Need to test with real credentials.

### Implementation

#### New files

- `internal/provider/minimax/minimax.go` — Provider struct, metadata, API key strategy
- `internal/provider/minimax/response.go` — Response types (to be determined from real API response)

#### Approach

Simple API key strategy. The main uncertainty is the response format — we need to make a real request to determine the structure. Start with a minimal implementation and iterate once we have sample responses.

#### Wiring changes

| File | Change |
|---|---|
| `cmd/root.go:23-28` | Add `_ "github.com/joshuadavidthomas/vibeusage/internal/provider/minimax"` blank import |
| `cmd/root.go:69` | Add `"minimax"` to the provider ID list |
| `cmd/key.go:26` | Add `"minimax"` to the key command loop |
| `cmd/key.go:91-97` | Add `"minimax": "api_key"` to `credentialKeyMap` |
| `cmd/auth.go` | Falls through to `authGeneric` |
| `internal/config/credentials.go` | Add `"minimax": "MINIMAX_API_KEY"` to `ProviderEnvVars` |

#### Status monitoring

No known public status page. Return `StatusUnknown` initially.

### Open questions

- [ ] What is the exact response format from `/coding_plan/remains`? Need a real API call
- [ ] Does it return remaining count, total, percentage, or all of the above?
- [ ] Does it include reset time information?
- [ ] Is plan tier information included in the response?

## Moonshot / KimiCode

Chinese AI provider with K2.5 model. KimiCode is an open-source CLI agent. Has subscription plans with 7-day rolling cycles (not 5-hour like others). Usage API exists at `https://api.kimi.com/coding/v1/usages`.

### What to track

- **Weekly usage summary**: `used` / `limit` with reset time (maps to `PeriodWeekly`)
- **Per-window limits**: Array of limit entries, each with `used`, `limit`, `remaining`, and `window` (duration + timeUnit)
- Reset times: ISO 8601 timestamps
- Period types: Mix of `PeriodWeekly` (7-day cycles) and `PeriodSession` (for sub-day windows, determined by `window.duration`)

### Auth strategy: API key (bearer token)

- API key from Kimi Code console: `https://www.kimi.com/code/console`
- Separate from Moonshot Open Platform API keys
- Base URL overridable via `KIMI_CODE_BASE_URL` env var

### API endpoints

Discovered from [KimiCode CLI source](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/ui/shell/usage.py):

```
GET https://api.kimi.com/coding/v1/usages
Authorization: Bearer <api_key>
```

**Response format** (from KimiCode parser):
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

**Field resolution** (KimiCode handles multiple field name variants):
- Reset time: tries `reset_at`, `resetAt`, `reset_time`, `resetTime`
- Duration-based reset: tries `reset_in`, `resetIn`, `ttl`, `window` (seconds)
- Usage values: supports both `used` and `remaining` (computes `used = limit - remaining`)
- Labels: tries `name`, `title`, `scope`

**Window duration to period type mapping**:
- `duration: 300, timeUnit: MINUTE` → 5 hours → `PeriodSession`
- `duration: 7, timeUnit: DAY` → `PeriodWeekly`
- `duration: 24, timeUnit: HOUR` → `PeriodDaily`

### Implementation

#### New files

- `internal/provider/kimi/kimi.go` — Provider struct, metadata, API key strategy
- `internal/provider/kimi/response.go` — Response types with flexible field resolution

#### Approach

Simple API key strategy with a well-understood response format (thanks to the open-source CLI). The response parsing needs to handle field name variants (snake_case and camelCase) and compute `used` from `remaining` when `used` isn't provided.

The `window` field's `duration` + `timeUnit` maps naturally to vibeusage's `PeriodType`:
- 300 MINUTE (5h) → `PeriodSession`
- 7 DAY → `PeriodWeekly`

#### Wiring changes

| File | Change |
|---|---|
| `cmd/root.go:23-28` | Add `_ "github.com/joshuadavidthomas/vibeusage/internal/provider/kimi"` blank import |
| `cmd/root.go:69` | Add `"kimi"` to the provider ID list |
| `cmd/key.go:26` | Add `"kimi"` to the key command loop |
| `cmd/key.go:91-97` | Add `"kimi": "api_key"` to `credentialKeyMap` |
| `cmd/auth.go` | Falls through to `authGeneric` |
| `internal/config/credentials.go` | Add `"kimi": "KIMI_CODE_API_KEY"` to `ProviderEnvVars` |

#### Status monitoring

No known public status page. Return `StatusUnknown` initially.

### Open questions

- [ ] Is the example response format above accurate? Need to verify with real credentials
- [ ] What subscription tiers exist and are they detectable from the API response?
- [ ] Does the API return plan information?
- [ ] Should we also support the Moonshot Open Platform API (pay-as-you-go) for token consumption tracking?

## Implementation order

### Phase 1: Antigravity

Lowest risk, highest code reuse. Same Google OAuth infra as Gemini, same quota response format, just different client credentials and request metadata.

1. Create `internal/provider/antigravity/` with response types
2. Implement OAuth strategy (adapt from Gemini)
3. Wire into CLI
4. Test with real Antigravity credentials
5. Verify credential reuse from Antigravity installation

### Phase 2: Kiro

More involved due to PKCE auth flow and unstable API response format.

1. Create `internal/provider/kiro/` with response types and multi-path resolution
2. Implement PKCE auth flow in `cmd/auth.go` (consider extracting reusable PKCE helper)
3. Implement credit-to-utilization conversion
4. Wire into CLI
5. Test with real Kiro credentials and multiple response format variants

### Phase 3: Z.ai

Simple API key auth, well-understood quota endpoint from reverse-engineered plugin.

1. Create `internal/provider/zai/` with response types
2. Implement API key strategy with quota/limit endpoint
3. Wire into CLI
4. Test with real Z.ai API key

### Phase 4: Kimi (Moonshot)

Simple API key auth, well-documented response format from open-source CLI.

1. Create `internal/provider/kimi/` with response types
2. Implement API key strategy with flexible field resolution
3. Map `window` duration to `PeriodType`
4. Wire into CLI
5. Test with real Kimi Code API key

### Phase 5: Minimax

Simple API key auth, but undocumented response format.

1. Test the `/coding_plan/remains` endpoint with real credentials to determine response format
2. Create `internal/provider/minimax/` with response types
3. Implement API key strategy
4. Wire into CLI

### Phase 6: Polish

1. Consider extracting shared Google OAuth logic between Gemini and Antigravity
2. Consider extracting reusable PKCE helper for future providers
3. Add response format test fixtures from real API responses
4. Update README with new provider docs
