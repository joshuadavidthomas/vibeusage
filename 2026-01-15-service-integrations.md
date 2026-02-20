---
date: 2026-01-15T12:00:00Z
researcher: Claude Code
git_commit: d0a79d9855d0f1024e209109fb0eb2396b20fe6f
branch: main
repository: CodexBar
topic: "Service Integrations Architecture"
tags: [research, codebase, integrations, providers, authentication]
status: complete
last_updated: 2026-01-15
last_updated_by: Claude Code
---

# Research: Service Integrations Architecture

**Date**: 2026-01-15
**Researcher**: Claude Code
**Git Commit**: d0a79d9855d0f1024e209109fb0eb2396b20fe6f
**Branch**: main
**Repository**: CodexBar

## Research Question

How does CodexBar integrate with all the services it monitors? Specifically:
- Which services does it integrate with?
- How does it authenticate with those services?
- What data does it grab from the services?
- How is that data presented?

## Summary

CodexBar is a macOS menu bar application that monitors usage quotas and costs across **12 AI/LLM service providers**. Each provider has a dedicated integration module following a consistent architectural pattern: a `ProviderDescriptor` defines metadata and fetch strategies, while dedicated probe/fetcher classes handle authentication and data retrieval. The application supports multiple authentication methods including OAuth 2.0, browser cookie import, API tokens, CLI session keys, and local process communication. Data is presented through a macOS status bar icon with dropdown menus, a CLI interface with ANSI coloring, and macOS widgets.

## Detailed Findings

### Services Integrated

CodexBar integrates with **12 AI/LLM service providers**:

| Provider | Display Name | Primary Use Case |
|----------|--------------|------------------|
| Claude | Claude | Anthropic's Claude AI (web, OAuth, CLI) |
| Codex | Codex | OpenAI's Codex/ChatGPT (OAuth, CLI, web dashboard) |
| Copilot | Copilot | GitHub Copilot (device flow OAuth) |
| Cursor | Cursor | Cursor IDE AI features |
| Gemini | Gemini | Google Gemini AI |
| VertexAI | Vertex AI | Google Cloud Vertex AI |
| Augment | Augment | Augment Code AI |
| Antigravity | Antigravity | Google-backed AI coding assistant |
| Factory | Droid | Factory.ai coding assistant |
| MiniMax | MiniMax | MiniMax AI platform |
| Kiro | Kiro | Amazon-backed Kiro IDE |
| Zai | z.ai | Z.ai API platform |

---

### Authentication Mechanisms

#### 1. OAuth 2.0 with Token Refresh

**Providers**: Claude, Codex, VertexAI, Gemini

**Implementation Pattern**:
- Credentials stored in JSON files (`~/.claude/.credentials.json`, `~/.codex/auth.json`, `~/.config/gcloud/application_default_credentials.json`)
- Backup storage in macOS Keychain
- Token refresh triggered when `needsRefresh` returns true (typically 5-8 day threshold)
- Refresh tokens exchanged at OAuth token endpoints

**Claude OAuth** (`Sources/CodexBarCore/Providers/Claude/ClaudeOAuth/`):
- Token endpoint: `https://api.anthropic.com`
- Usage endpoint: `/api/oauth/usage` with beta header `oauth-2025-04-20`
- Requires `user:profile` scope for usage data
- Credentials in `~/.claude/.credentials.json` or Keychain service `"Claude Code-credentials"`

**Codex OAuth** (`Sources/CodexBarCore/Providers/Codex/CodexOAuth/`):
- Token endpoint: `https://auth.openai.com/oauth/token`
- Client ID: `app_EMoamEEZ73f0CkXaXp7hrann`
- Refresh triggered after 8 days
- Credentials in `~/.codex/auth.json`

**VertexAI OAuth** (`Sources/CodexBarCore/Providers/VertexAI/VertexAIOAuth/`):
- Uses gcloud application default credentials
- Token endpoint: `https://oauth2.googleapis.com/token`
- Project ID from `~/.config/gcloud/configurations/config_default` or environment variables
- 5-minute refresh buffer before expiry

**Gemini OAuth** (`Sources/CodexBarCore/Providers/Gemini/GeminiStatusProbe.swift`):
- Tokens from `~/.gemini/oauth_creds.json`
- Client credentials extracted from Gemini CLI installation (`oauth2.js`)
- Token endpoint: `https://oauth2.googleapis.com/token`

#### 2. GitHub Device Flow OAuth

**Provider**: Copilot

**Implementation** (`Sources/CodexBarCore/Providers/Copilot/CopilotDeviceFlow.swift`):
- Device code endpoint: `POST https://github.com/login/device/code`
- Token endpoint: `POST https://github.com/login/oauth/access_token`
- Client ID: `Iv1.b507a08c87ecfe98` (VS Code Client ID)
- Scope: `read:user`
- User enters code at `verificationUri` (github.com/login/device)
- Polling with `slow_down` backoff handling

#### 3. Browser Cookie Import

**Providers**: Cursor, Augment, Factory, MiniMax

**Implementation Pattern**:
- Uses `BrowserCookieClient` to read from browser SQLite databases
- Supports multiple browsers: Safari, Chrome, Firefox, Arc, Brave, Edge, Opera, Vivaldi
- Cookie domains vary by provider
- Fallback to stored session cookies from previous successful imports

**Cursor Cookies** (`Sources/CodexBarCore/Providers/Cursor/CursorStatusProbe.swift`):
- Cookie names: `WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `next-auth.session-token`
- Domains: `cursor.com`, `cursor.sh`
- Persisted to `~/Library/Application Support/CodexBar/cursor-session.json`

**Augment Cookies** (`Sources/CodexBarCore/Providers/Augment/AugmentStatusProbe.swift`):
- Cookie names: `session`, `_session`, `web_rpc_proxy_session`, various next-auth and authjs variants
- Domains: `augmentcode.com`, `app.augmentcode.com`
- Auto-saves successful imports to `~/Library/Application Support/CodexBar/augment-session.json`

**Factory Cookies** (`Sources/CodexBarCore/Providers/Factory/FactoryStatusProbe.swift`):
- 8-layer fallback cascade including Safari, Chromium, stored sessions, bearer tokens, refresh tokens, localStorage
- Cookie types: `wos-session`, various next-auth/authjs variants, `access-token`
- Domains: `factory.ai`, `app.factory.ai`, `auth.factory.ai`
- WorkOS OAuth integration for token refresh

**MiniMax Cookies** (`Sources/CodexBarCore/Providers/MiniMax/`):
- Combined Cookie + Bearer token authentication
- Access tokens imported from browser localStorage
- Group IDs from localStorage for API queries
- Multi-attempt strategy tries each browser with multiple token combinations

#### 4. API Token Authentication

**Providers**: Zai, Copilot

**Zai** (`Sources/CodexBarCore/Providers/Zai/`):
- Environment variable: `Z_AI_API_KEY`
- Bearer token in Authorization header
- Stored in macOS Keychain via preferences

**Copilot** (`Sources/CodexBarCore/Providers/Copilot/`):
- GitHub OAuth token used as API token
- Header: `Authorization: token <oauth_token>`

#### 5. CLI Session Keys

**Providers**: Claude, Kiro

**Claude CLI** (`Sources/CodexBarCore/Providers/Claude/ClaudeStatusProbe.swift`):
- Runs `claude` binary in PTY with `/usage` command
- Parses ANSI-formatted terminal output
- Uses `ClaudeCLISession` singleton to avoid warm-up overhead
- Session key from browser cookies used as web API fallback

**Kiro CLI** (`Sources/CodexBarCore/Providers/Kiro/KiroStatusProbe.swift`):
- Runs `kiro-cli chat --no-interactive /usage`
- CLI manages OAuth internally
- Parses ANSI-formatted output with progress bars

#### 6. Local Process Communication

**Provider**: Antigravity

**Implementation** (`Sources/CodexBarCore/Providers/Antigravity/AntigravityStatusProbe.swift`):
- Detects running `language_server_macos` process via `ps -ax`
- Extracts CSRF token from process command line arguments
- Discovers listening ports via `lsof -nP -iTCP -sTCP:LISTEN`
- HTTPS POST to local endpoints on `127.0.0.1`
- Accepts self-signed certificates via custom URLSession delegate

---

### Data Fetched from Each Service

#### Claude

**API Endpoints**:
- OAuth: `GET https://api.anthropic.com/api/oauth/usage`
- Web: `GET https://claude.ai/api/organizations/{orgId}/usage`
- Web Cost: `GET https://claude.ai/api/organizations/{orgId}/overage_spend_limit`

**Data Retrieved**:
- `five_hour` rate window (session): utilization percentage, reset time
- `seven_day` rate window (weekly): utilization percentage, reset time
- `seven_day_opus/sonnet` rate windows (model-specific)
- Extra usage (cost): enabled flag, monthly limit, used credits, currency
- Account: email, organization, rate limit tier/plan

#### Codex (OpenAI)

**API Endpoints**:
- OAuth: Dynamic endpoint from `~/.codex/config.toml`, default `https://chatgpt.com/backend-api/wham/usage`
- Web Dashboard: `https://chatgpt.com/codex/settings/usage` (WebView scraping)

**Data Retrieved**:
- Plan type (free, go, plus, pro, team, business, enterprise, etc.)
- Rate limits: primary/secondary windows with used percent, reset timestamp, window seconds
- Credits: has_credits flag, unlimited flag, balance
- Web dashboard: code review remaining percent, credit events history, daily breakdown

#### Copilot

**API Endpoint**: `GET https://api.github.com/copilot_internal/user`

**Data Retrieved**:
- Plan type (premium, free)
- Premium interactions quota: entitlement, remaining, percent remaining
- Chat quota: entitlement, remaining, percent remaining
- Quota reset date

#### Cursor

**API Endpoints**:
- `POST /api/usage-summary`: Primary usage data
- `GET /api/auth/me`: User information
- `GET /api/usage?user=ID`: Legacy request-based usage

**Data Retrieved**:
- Billing cycle dates (start, end)
- Individual and team usage (tokens, on-demand)
- On-demand spend: used cents, limit cents
- User: email, name, user ID
- Membership type, premium requests (used/available)

#### Gemini

**API Endpoints**:
- `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota`
- `POST https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist`
- `GET https://cloudresourcemanager.googleapis.com/v1/projects`

**Data Retrieved**:
- Quota buckets per model: remaining fraction, reset time, model ID
- User tier: free-tier, standard-tier, legacy-tier
- Email from JWT id_token
- Workspace domain (for plan determination)

#### VertexAI

**API Endpoint**: `GET https://monitoring.googleapis.com/v3/projects/{projectId}/timeSeries`

**Data Retrieved**:
- Quota allocation usage metrics for `aiplatform.googleapis.com`
- Quota limits per metric
- Maximum quota usage percentage across all quotas
- Project ID, user email from credentials

#### Augment

**API Endpoints**:
- `GET /api/credits`: Credit balance
- `GET /api/subscription`: Subscription info

**Data Retrieved**:
- Credits: remaining, consumed this cycle, available, balance status
- Subscription: plan name, billing period end, email, organization

#### Antigravity

**Local API Endpoints** (via localhost language server):
- `POST /exa.language_server_pb.LanguageServerService/GetUserStatus`
- `POST /exa.language_server_pb.LanguageServerService/GetCommandModelConfigs`

**Data Retrieved**:
- Model configs: label, model ID, quota info (remaining fraction, reset time)
- User status: email, plan status, plan info
- Cascaded model quotas for Claude, Gemini Pro, Gemini Flash

#### Factory (Droid)

**API Endpoints**:
- `GET /api/app/auth/me`: User and subscription info
- `GET /api/organization/subscription/usage`: Token usage

**Data Retrieved**:
- Standard tier: user tokens, org tokens, allowance, overage
- Premium tier: user tokens, org tokens, allowance
- Period dates, plan name/tier, organization name, user ID

#### MiniMax

**API Endpoints**:
- HTML: `GET https://platform.minimax.io/user-center/payment/coding-plan?cycle_type=3`
- JSON: `GET https://platform.minimax.io/v1/api/openplatform/coding_plan/remains`

**Data Retrieved**:
- Plan name, available prompts, window minutes
- Usage: total count, usage count, remaining
- Reset times (start, end, remains_time)

#### Kiro

**CLI Command**: `kiro-cli chat --no-interactive /usage`

**Data Retrieved** (parsed from ANSI output):
- Plan name (e.g., "KIRO FREE")
- Credits: used, total, percentage
- Bonus credits: used, total, expiry days
- Reset date (MM/DD format)

#### Zai

**API Endpoint**: `GET https://api.z.ai/api/monitor/usage/quota/limit`

**Data Retrieved**:
- Limits array with type (TIME_LIMIT, TOKENS_LIMIT)
- Per-limit: unit, window size, usage, current value, remaining, percentage
- Next reset time (Unix timestamp milliseconds)
- Usage details per model
- Plan name

---

### Data Presentation

#### 1. macOS Status Bar (`StatusItemController.swift`)

**Icon Display**:
- Single merged icon or separate per-provider icons
- Icon shows usage percentages via `IconRenderer`
- Animated during refresh operations
- Stale indicator when data is outdated

**Menu System**:
- `MenuDescriptor` builds menu structure with sections
- `MenuContent` renders SwiftUI views in menu
- Sections: usage stats, actions (refresh, dashboard links), settings
- Chart views for usage breakdown and cost history

**Usage Display**:
- Primary rate window (e.g., "Session: 45% left")
- Secondary rate window (e.g., "Weekly: 78% left")
- Tertiary/Opus window for premium tier tracking
- Reset countdown or absolute time
- Cost tracking with currency formatting

#### 2. CLI Renderer (`CLIRenderer.swift`)

**Output Format**:
- Colored header line per provider
- Rate lines with ANSI color coding:
  - Green: >= 25% remaining
  - Yellow: 10-25% remaining
  - Red: < 10% remaining
- Account email and plan display
- Status indicator with service-specific coloring

**Formatting** (via `UsageFormatter`):
- Percentages: "X% left" or "X% used"
- Reset times: countdown ("in 2h 30m") or absolute ("tomorrow at 3pm")
- Currency: localized with 2 decimal places
- Token counts: K/M/B suffixes for large numbers

#### 3. macOS Widgets (`CodexBarWidget/`)

**Widget Types**:
- Small, medium, large sizes
- `WidgetSnapshot` data structure for efficient transfer
- `CodexBarWidgetProvider` manages timeline updates
- SwiftUI views in `CodexBarWidgetViews.swift`

#### 4. Data Storage (`UsageStore.swift`)

**Observable Store Pattern**:
- `@Observable` class with `menuObservationToken` for SwiftUI binding
- `snapshots` dictionary: provider -> `UsageSnapshot`
- `errors` dictionary: provider -> error messages
- `lastSourceLabels`: tracks data source per provider
- `tokenSnapshots`: cost/token usage data
- `credits`: OpenAI credits snapshot
- `statuses`: provider status indicators

**Refresh Flow**:
- `refresh()` triggers concurrent provider fetches via `withTaskGroup`
- `refreshProvider()` executes fetch plan strategies in order
- Failure gate prevents flapping on intermittent errors
- Auto-save to widget snapshot after refresh

---

## Code References

### Core Framework
- `Sources/CodexBarCore/Providers/ProviderDescriptor.swift` - Base provider protocol
- `Sources/CodexBarCore/Providers/Providers.swift` - Provider enumeration (12 providers)
- `Sources/CodexBarCore/Providers/ProviderFetchPlan.swift` - Fetch strategy pipeline
- `Sources/CodexBarCore/UsageFetcher.swift` - Usage data structures

### Provider Implementations

| Provider | Descriptor | Authentication | Fetcher/Probe |
|----------|-----------|----------------|---------------|
| Claude | `Claude/ClaudeProviderDescriptor.swift:7` | `ClaudeOAuth/ClaudeOAuthCredentials.swift` | `ClaudeUsageFetcher.swift`, `ClaudeStatusProbe.swift` |
| Codex | `Codex/CodexProviderDescriptor.swift:8` | `CodexOAuth/CodexOAuthCredentials.swift` | `CodexOAuthUsageFetcher.swift` |
| Copilot | `Copilot/CopilotProviderDescriptor.swift:7` | `CopilotDeviceFlow.swift` | `CopilotUsageFetcher.swift` |
| Cursor | `Cursor/CursorProviderDescriptor.swift:44` | Cookie import in probe | `CursorStatusProbe.swift:488` |
| Gemini | `Gemini/GeminiProviderDescriptor.swift:44` | OAuth in probe | `GeminiStatusProbe.swift:108` |
| VertexAI | `VertexAI/VertexAIProviderDescriptor.swift:7` | `VertexAIOAuth/VertexAIOAuthCredentials.swift` | `VertexAIUsageFetcher.swift` |
| Augment | `Augment/AugmentProviderDescriptor.swift:67` | Cookie import in probe | `AugmentStatusProbe.swift:361` |
| Antigravity | `Antigravity/AntigravityProviderDescriptor.swift` | CSRF token from process | `AntigravityStatusProbe.swift` |
| Factory | `Factory/FactoryProviderDescriptor.swift` | Multi-layer cascade | `FactoryStatusProbe.swift` |
| MiniMax | `MiniMax/MiniMaxProviderDescriptor.swift:44` | Cookie + Bearer | `MiniMaxUsageFetcher.swift` |
| Kiro | `Kiro/KiroProviderDescriptor.swift` | CLI OAuth | `KiroStatusProbe.swift` |
| Zai | `Zai/ZaiProviderDescriptor.swift` | API token | `ZaiUsageStats.swift` |

### Presentation Layer
- `Sources/CodexBar/StatusItemController.swift:15` - Status bar management
- `Sources/CodexBar/UsageStore.swift:216` - Observable data store
- `Sources/CodexBar/MenuContent.swift:6` - SwiftUI menu views
- `Sources/CodexBarCLI/CLIRenderer.swift:5` - CLI text rendering
- `Sources/CodexBarCore/UsageFormatter.swift:8` - Data formatting utilities

## Architecture Documentation

### Provider Framework Pattern

Each provider follows a consistent architecture:

1. **ProviderDescriptor** - Static configuration:
   - Metadata (names, labels, URLs)
   - Branding (colors, icons)
   - Token cost config
   - Fetch plan (supported modes, strategy pipeline)
   - CLI config (command names, version detector)

2. **Fetch Strategies** - Implement `ProviderFetchStrategy` protocol:
   - `isAvailable(_:)` - Check if strategy can be used
   - `fetch(_:)` - Execute data retrieval
   - `shouldFallback(on:)` - Determine retry behavior

3. **Fetch Pipeline** - Executes strategies in order:
   - Resolves strategies based on context (app vs CLI, source mode)
   - Attempts each available strategy until success
   - Records all attempts for debugging
   - Returns `ProviderFetchOutcome` with result and attempt history

4. **Data Normalization** - All providers produce:
   - `UsageSnapshot` with up to 3 rate windows
   - `ProviderCostSnapshot` for billing/spend tracking
   - `ProviderIdentitySnapshot` for account info

### Source Mode Selection

Providers support multiple data source modes (`.auto`, `.web`, `.cli`, `.oauth`):

| Mode | App Runtime Order | CLI Runtime Order |
|------|-------------------|-------------------|
| `.auto` | OAuth -> Web -> CLI | Web -> CLI |
| `.oauth` | OAuth only | OAuth only |
| `.web` | Web only | Web only |
| `.cli` | CLI only | CLI only |

### Cookie Import Architecture

Browser cookie import follows a consistent pattern:

1. Query browser SQLite databases for session cookies
2. Filter by domain and cookie name
3. Build cookie header string
4. Store successful sessions to disk for fallback
5. Use stored sessions when browser import fails

## Additional Findings from Documentation

### Local Cost Usage Scanning

Both Claude and Codex support **local JSONL log scanning** for token cost tracking independent of API usage:

**Claude Cost Usage** (`docs/claude.md`):
- Source roots: `$CLAUDE_CONFIG_DIR/projects`, `~/.config/claude/projects`, `~/.claude/projects`
- Files: `**/*.jsonl`
- Parsing: Lines with `type: "assistant"` and `message.usage`
- Deduplicates streaming chunks by `message.id + requestId`
- Cache: `~/Library/Caches/CodexBar/cost-usage/claude-v1.json`

**Codex Cost Usage** (`docs/codex.md`):
- Source: `~/.codex/sessions/YYYY/MM/DD/*.jsonl` (or `$CODEX_HOME/sessions/...`)
- Parses `event_msg` token counts and `turn_context` model markers
- Computes input/cached/output token deltas and per-model cost
- Cache: `~/Library/Caches/CodexBar/cost-usage/codex-v1.json`
- Window: last 30 days (rolling), 60s minimum refresh interval

**VertexAI Cost Usage** (`docs/vertexai.md`):
- Source: `~/.claude/projects/` logs filtered to Vertex AI-tagged entries
- Token cost tracked from local Claude logs, not from VertexAI APIs

### Status Polling

CodexBar polls service status from multiple sources (`docs/status.md`):

| Provider Group | Status Source |
|---------------|---------------|
| OpenAI, Claude, Cursor, Factory, Copilot | Statuspage.io `api/v2/status.json` |
| Gemini, Antigravity | Google Workspace incidents feed for Gemini product |
| Kiro | AWS Health Dashboard (manual link, no auto-polling) |
| Zai | None |

- Toggle: Settings → Advanced → "Check provider status"
- Stores `ProviderStatus` for indicator and description
- Menu shows incident summary with freshness; icon overlays status indicator

### Refresh Loop

**Refresh Cadence** (`docs/refresh-loop.md`):
- Options: Manual, 1m, 2m, **5m (default)**, 15m
- Stored in `UserDefaults` via `SettingsStore`
- Background refresh updates `UsageStore` (usage + credits + optional web scrape)
- Manual "Refresh now" always available in menu
- Stale/error states dim the icon and surface status in-menu

### UI Details

**Icon Rendering** (`docs/ui.md`):
- 18×18 template image
- **Top bar** = 5-hour session window
- **Bottom hairline** = weekly window
- Fill represents percent remaining (flippable to "used" in settings)
- Dimmed when last refresh failed
- Status overlay renders incident indicators
- Advanced: menu bar can show provider branding icons with percent label

**Menu Card**:
- Session + weekly rows with resets (countdown default, optional absolute clock)
- Codex-only: Credits + "Buy Credits…" in-card action
- Web-only rows (when OpenAI cookies enabled): code review remaining, usage breakdown submenu

### Application Architecture

**Modules** (`docs/architecture.md`):
- `Sources/CodexBarCore`: fetch + parse (RPC, PTY, probes, web scraping, status polling)
- `Sources/CodexBar`: state + UI (UsageStore, SettingsStore, StatusItemController, menus, icon)
- `Sources/CodexBarWidget`: WidgetKit extension wired to shared snapshot
- `Sources/CodexBarCLI`: bundled CLI for `codexbar` usage/status output
- `Sources/CodexBarMacros`: SwiftSyntax macros for provider registration
- `Sources/CodexBarClaudeWatchdog`: helper process for stable Claude CLI PTY sessions
- `Sources/CodexBarClaudeWebProbe`: CLI helper to diagnose Claude web fetches

**Entry Points**:
- `CodexBarApp`: SwiftUI keepalive + Settings scene
- `AppDelegate`: wires status controller, Sparkle updater, notifications

**Data Flow**:
- Background refresh → `UsageFetcher`/provider probes → `UsageStore` → menu/icon/widgets
- Settings toggles feed `SettingsStore` → `UsageStore` refresh cadence + feature flags

**Platform**:
- Swift 6 strict concurrency enabled
- macOS 14+ targeting
- LSUIElement app (no Dock icon)

### User Configuration

**Cookie Source Picker** (available for cookie-based providers):
- Preferences → Providers → [Provider] → Cookie source
- Options: **Automatic** (imports from browsers) or **Manual** (paste Cookie header)

**Usage Source Picker** (Claude, Codex):
- Preferences → Providers → [Provider] → Usage source
- Options: Auto/OAuth/Web/CLI

## Historical Context (from docs/)

- `docs/providers.md` - Provider data sources and parsing overview
- `docs/architecture.md` - Module structure and entry points
- `docs/claude.md` - Claude provider implementation details
- `docs/codex.md` - Codex provider implementation details
- `docs/codex-oauth.md` - Codex OAuth implementation plan
- `docs/status.md` - Status polling sources and behavior
- `docs/refresh-loop.md` - Refresh cadence and background updates
- `docs/ui.md` - Menu bar UI and icon rendering details

## Open Questions

1. How does the application handle rate limiting from the various APIs?
2. What is the retry/backoff strategy for transient failures?
3. How are credentials securely transmitted between the app and CLI?
4. What telemetry or logging exists for authentication failures?
