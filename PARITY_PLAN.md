# Provider Parity Plan

## Current State

vibeusage currently ships 9 providers:

`claude`, `gemini`, `codex`, `copilot`, `cursor`, `antigravity`, `kimi`, `minimax`, `zai`

Reference apps:

- **openusage**: 13 providers
- **CodexBar**: 21+ providers
- **ccusage**: different model (local log parsing), not a direct parity target

## Planning Principles

We prioritize providers by:

1. User impact (how many users likely have this provider)
2. Implementation effort (API-key and typed JSON first)
3. Cross-platform viability (Linux/macOS/Windows preferred)
4. Runtime fragility (cookie scraping and local app internals later)

## Prioritized Backlog

| Priority | Provider | Why now | Auth model | Platform risk | Estimated effort |
|---|---|---|---|---|---|
| P0 | OpenRouter | High demand, simple API key flow | API key | Low | S |
| P0 | Warp | Popular, documented key flow | API key | Low | M |
| P0 | KimiK2 | Common request, simple endpoint | API key | Low | S |
| P0 | Amp | Good parity win, manageable parsing | Local secrets file + bearer | Low | M |
| P1 | Factory | Strong parity value, moderate auth complexity | OAuth tokens + refresh | Medium | M |
| P1 | JetBrains | Unique local-only value, no web auth | Local XML files | Low | M |
| P1 | Kiro | CLI-native model fits vibeusage well | Local CLI session | Low | M |
| P1 | Windsurf | High parity value, but local LS probing | SQLite + local RPC | Medium/High | L |
| P2 | OpenCode | Cookie + workspace flow, brittle payloads | Cookie | High | L |
| P2 | Ollama | Cookie + HTML parsing | Cookie | High | M/L |
| P2 | Augment | Cookie + session behavior complexity | Cookie (optional CLI) | High | L |
| P3 | Perplexity | macOS cache internals + brittle extraction | App cache SQLite token | High (macOS-only) | L |
| P3 | VertexAI | Heavy Google Cloud/OAuth/quota pipeline | ADC OAuth + Monitoring API | Medium/High | L |

## Implementation Details by Provider

## P0

### 1. OpenRouter

- Auth
  - Env vars: `OPENROUTER_API_KEY`
  - Manual auth flow: `vibeusage auth openrouter` (manual key entry)
- Strategy
  - `APIKeyStrategy` with source `"api_key"`
- Endpoints
  - `GET https://openrouter.ai/api/v1/credits`
  - Optional enrichment: `GET /api/v1/key` with short timeout
- Mapping
  - Primary period: `Credits`, `PeriodMonthly`, utilization from `total_usage/total_credits`
  - Overage: `Used=total_usage`, `Limit=total_credits`, `Currency=USD`, `IsEnabled=true`
- Errors
  - 401/403 -> fatal re-auth hint
  - Non-200 -> include status + body summary
  - JSON parse errors -> include underlying error
- Tests
  - Parse success/failure cases, zero credits edge case, auth failure handling

### 2. Warp

- Auth
  - Env vars: `WARP_API_KEY`, `WARP_TOKEN`
  - Manual flow with prefix validation `wk-`
- Strategy
  - `APIKeyStrategy`, source `"api_key"`
- Endpoint
  - `POST https://app.warp.dev/graphql/v2?op=GetRequestLimitInfo`
- Request details
  - GraphQL operation `GetRequestLimitInfo`
  - Include headers expected by Warp (`x-warp-client-id`, OS headers, UA)
- Mapping
  - Primary period: monthly credits (requestLimit vs requestsUsed)
  - `resetsAt=nextRefreshTime`
  - Secondary optional period: bonus credits grants if present
- Tests
  - GraphQL happy path, unlimited plans, missing fields, error array parsing

### 3. KimiK2

- Auth
  - Env vars: `KIMI_K2_API_KEY`, `KIMI_API_KEY`, `KIMI_KEY`
  - Manual key flow
- Strategy
  - `APIKeyStrategy`, source `"api_key"`
- Endpoint
  - `GET https://kimi-k2.ai/api/user/credits`
- Mapping
  - Primary period: `Credits`, utilization from `consumed/(consumed+remaining)`
  - No reliable reset timestamp
- Parsing
  - Typed structs with tolerant fallback for known field variants
  - Header fallback for remaining credits if body omits it
- Tests
  - Multiple JSON schema variants and header fallback coverage

### 4. Amp

- Auth
  - Auto-detect from `~/.local/share/amp/secrets.json` key `apiKey@https://ampcode.com/`
  - Optional manual key fallback for portability
- Strategy
  - `CLISecretsStrategy`, source `"provider_cli"` or `"api_key"`
- Endpoint
  - `POST https://ampcode.com/api/internal`
  - JSON-RPC method `userDisplayBalanceInfo`
- Mapping
  - Free quota: daily period from `$remaining/$total`
  - Reset estimate from hourly replenishment rate
  - Credits displayed via identity and/or overage line
- Parsing
  - `displayText` regex extraction with explicit parse errors
- Tests
  - Free tier, credits-only, bonus text, malformed text, auth error

## P1

### 5. Factory

- Auth
  - Read `~/.factory/auth.json` and `~/.factory/auth.encrypted`
  - Support payloads containing `access_token` + `refresh_token`
  - Refresh via WorkOS when near expiry or on 401
- Strategy
  - `OAuthStrategy`, source `"oauth"`
- Endpoints
  - Usage: `POST https://api.factory.ai/api/organization/subscription/usage`
  - Refresh: `POST https://api.workos.com/user_management/authenticate`
- Mapping
  - Standard and Premium monthly periods
  - `resetsAt=endDate`, period duration from start/end
  - Identity plan inferred from allowance tiers
- Tests
  - Token refresh success/failure, plan inference, missing premium allowance

### 6. JetBrains

- Auth
  - None (local file parsing)
- Strategy
  - `LocalFileStrategy`, source `"local"`
- Data source
  - Auto-detect IDE config roots
  - Parse `<IDE>/options/AIAssistantQuotaManager2.xml`
- Mapping
  - Primary monthly period from quota `current/maximum`
  - Reset from `nextRefill.next`
  - Identity: detected IDE name/version + quota type
- Platform
  - Linux/macOS first; Windows support can be added via path expansion later
- Tests
  - XML/HTML-entity decoding, JSON payload extraction, missing fields

### 7. Kiro

- Auth
  - Delegated to local `kiro-cli` login state
- Strategy
  - `CLIStrategy`, source `"cli"`
- Data source
  - Run: `kiro-cli chat --no-interactive "/usage"`
  - Strip ANSI output and parse usage sections
- Mapping
  - Monthly period from parsed percent and reset date
  - Optional bonus credits as secondary period
- Tests
  - Parse old/new CLI output formats, unauthenticated output, timeout behavior

### 8. Windsurf

- Auth/Data source
  - Read API key from SQLite `state.vscdb` (`windsurfAuthStatus`)
  - Discover local language server process and CSRF token from args
  - Probe local RPC port, call `GetUserStatus`
- Strategy
  - `LocalRPCStrategy`, source `"local_rpc"`
- Mapping
  - Prompt credits period and Flex credits period
  - Convert hundredths to display units (`/100`)
  - `resetsAt=planEnd`
- Platform
  - Implement macOS first (path/process assumptions), gate unsupported platforms clearly
- Tests
  - Parser tests for `GetUserStatus` payloads and unit conversion

## P2

### 9. OpenCode

- Auth
  - Manual cookie header first (`vibeusage auth opencode`)
  - Optional workspace ID override
- Strategy
  - `WebStrategy`, source `"web"`
- Endpoint
  - `https://opencode.ai/_server` (workspace + subscription calls)
- Mapping
  - Rolling 5-hour period and weekly period from usage percent + reset seconds
- Notes
  - Keep MVP strict and explicit about parse failures

### 10. Ollama

- Auth
  - Manual cookie header first
- Strategy
  - `WebStrategy`, source `"web"`
- Data source
  - Parse `https://ollama.com/settings` HTML for session/weekly usage
- Mapping
  - Session and weekly periods with reset timestamps if present
- Notes
  - Browser auto-import can be deferred until a later UX pass

### 11. Augment

- Auth
  - Manual cookie header first
  - Optional CLI-based strategy can be added later if stable
- Strategy
  - `WebStrategy`, source `"web"`
- Endpoints
  - `/api/credits`, `/api/subscription`
- Mapping
  - Monthly credits period and plan identity
  - `resetsAt=billingPeriodEnd`
- Notes
  - Start without keepalive/session-refresh machinery

## P3

### 12. Perplexity

- Auth/data source
  - Extract bearer token from local Perplexity app cache SQLite DB
- Strategy
  - `LocalCacheStrategy`, source `"provider_cache"`
- Endpoints
  - Group + usage analytics REST endpoints
- Mapping
  - Cost-based USD usage vs balance, optional Pro identity
- Notes
  - macOS-only and format-fragile, so defer

### 13. VertexAI

- Auth
  - Google ADC credentials (`gcloud auth application-default login`)
- Strategy
  - `OAuthStrategy`, source `"oauth"`
- Endpoint
  - Cloud Monitoring quota usage/limit time-series for `aiplatform.googleapis.com`
- Mapping
  - Quota utilization period (`Current quota`), no guaranteed reset
- Notes
  - Highest complexity; build only after higher-impact providers are complete

## Cross-Cutting Engineering Tasks

1. Add new providers to import registration in `internal/cli/root.go`.
2. Expand `internal/cli/key.go` provider lists/maps for new providers.
3. Keep all provider responses typed (no `map[string]any` for top-level parsing).
4. Reuse `internal/httpclient` with `RequestOption` headers/cookies.
5. Preserve cache fallback behavior and `--refresh` semantics.
6. Ensure credential-specific auth hints (only suggest `vibeusage auth <provider>` for credential failures).
7. Add per-provider tests for parsing + auth failure + availability checks.

## Milestone Plan

### Milestone A (P0)

- [ ] OpenRouter
- [ ] Warp
- [ ] KimiK2
- [ ] Amp
- [ ] Docs and auth/key UX updates for all P0 providers

### Milestone B (P1)

- [ ] Factory
- [ ] JetBrains
- [ ] Kiro
- [ ] Windsurf (macOS-first)

### Milestone C (P2)

- [ ] OpenCode (manual-cookie MVP)
- [ ] Ollama (manual-cookie MVP)
- [ ] Augment (manual-cookie MVP)

### Milestone D (P3)

- [ ] Perplexity (macOS-only)
- [ ] VertexAI

## Definition of Done per Provider

- Provider is discoverable in `vibeusage auth --status`, `vibeusage key`, and default fetch paths.
- At least one fetch strategy passes availability checks when credentials exist.
- Snapshot includes meaningful period(s), and identity/overage when applicable.
- Fatal auth errors are clearly distinguishable from transient/network failures.
- Unit tests cover happy path + malformed response + auth failure.
- README/provider docs updated with credential setup and known platform constraints.
