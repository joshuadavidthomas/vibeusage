# Idiomatic Go Refactoring Plan

Findings from reviewing the codebase against canonical Go best practices (Effective Go, Google Go Style Guide, 100 Go Mistakes, Go Proverbs).

## High Impact

### 1. Extract Business Logic from `cmd/`

> *"The main package should be completely devoid of business logic."*

**`cmd/route.go` (~660 lines)**
- [x] Move `routeModel()`, `routeByRole()`, `buildModelRolesMap()` orchestration to an `internal/routing/` service (or new `internal/route/` package)
- [x] Move `configuredProviders()` to `internal/provider/`
- [x] Extract duplicated cost formatting (`0x → "free"`, int check, `%.2gx`) from `displayRecommendation` and `displayRoleRecommendation` into a shared helper
- [x] Extract the shared table-rendering pattern (hasMultiplier column toggle, unavailable dim rows) into a common function

**`cmd/auth.go` (~360 lines)**
- [x] Define an `Authenticator` interface (or add `Auth()` to `Provider`) so each provider owns its auth flow
- [x] Eliminate the `switch providerID` dispatch — the command layer should just call `provider.Auth(id)`
- [x] The manual-key providers (claude, cursor, zai, minimax) should implement the same pattern that antigravity/copilot/kimi already use (delegating to `RunAuthFlow`/`RunDeviceFlow`)

**`cmd/root.go`**
- [x] Replace hardcoded provider list on line 73 (`[]string{"antigravity", "claude", ...}`) with `provider.ListIDs()`

### 2. Inject Config Instead of Global Singleton

> *"Idiomatic Go relies heavily on explicit dependency injection."*

`config.Get()` is called as a hidden dependency in 13+ sites across `internal/fetch/`, `internal/provider/`, and `cmd/`.

- [x] `ExecutePipeline`: accept timeout and stale threshold as parameters (or a `PipelineConfig` struct) instead of calling `config.Get()` internally
- [x] `FetchAllProviders` / `FetchEnabledProviders`: accept max concurrency as a parameter
- [x] Provider strategies: inject HTTP timeout rather than each provider reading `config.Get().Fetch.Timeout` internally
- [x] `config.CacheSnapshot` / `config.LoadCachedSnapshot` calls inside `pipeline.go`: pass cache dir or a cache interface

## Medium Impact

### 3. Replace `map[string]any` with Typed Structs for JSON

> *"`any` says nothing" — it destroys compile-time verification.*

- [x] `internal/display/json.go`: Replace `SnapshotToJSON() map[string]any` with a `SnapshotJSON` struct using `json` tags
- [x] `cmd/auth.go`: Replace `map[string]any` auth status output with a typed struct
- [x] `cmd/config.go`: Replace `map[string]any` config output with a typed struct
- [x] `cmd/key.go`: Replace `map[string]any` key status output with a typed struct
- [x] `cmd/init.go`: Replace `map[string]any` init output with a typed struct
- [x] `cmd/cache.go`: Replace `map[string]any` cache output with a typed struct
- [x] `display.OutputMultiProviderJSON`: Replace internal `map[string]any` construction with typed structs

### 4. Explicit Logger Initialization

> *"Relying on init functions is highly discouraged."*

- [ ] Remove `init()` from `internal/logging/logging.go`
- [ ] Create the logger explicitly in `cmd/` (or pass it from `main`)
- [ ] Consider making the logger injectable rather than a package-level global (longer term)

### 5. Add `context.Context` to `FetchStatus()`

> *"Idiomatic Go uses the context package to propagate cancellation signals."*

- [ ] Add `context.Context` parameter to `Provider.FetchStatus()`
- [ ] Update all provider implementations
- [ ] Add concurrency bound (semaphore channel) to `fetchAllStatuses()` in `cmd/status.go` — currently unbounded unlike the well-done `FetchAllProviders`

### 6. `OutputJSON` Should Return `error`

> *"Don't just check errors, handle them gracefully."*

- [ ] Change `OutputJSON(w io.Writer, data any)` to return `error`
- [ ] Update all call sites to handle the returned error

## Low Impact

### 7. Warn on Malformed Config

- [ ] `config.Load()`: When `toml.Decode` fails, log a warning (via the logger or return the error) instead of silently falling back to defaults

### 8. Make `modelmap` Init Explicit

> *"Dependencies are passed explicitly, granting the caller complete control."*

- [ ] Consider making the initial `models.dev` fetch explicit (called during startup with spinner feedback) rather than lazy-loading on first `Lookup()` with no user feedback
- [ ] Replace hand-rolled YAML parser in `multipliers.go` with a proper YAML library

### 9. Reduce Display Logic Duplication in `cmd/route.go`

- [ ] `displayRecommendation` and `displayRoleRecommendation` are ~80% identical
- [ ] Extract `formatCost(*float64) string` helper
- [ ] Unify the table construction into a single function parameterized by whether a "Model" column is present

## Things Already Done Well ✅

- Thin `main.go` with signal handling and context propagation
- 3 small, consumer-side interfaces (`Strategy`, `Provider`, `Prompter`)
- Functional options pattern in `httpclient.RequestOption`
- Stateless provider structs with useful zero values
- Bounded concurrency with semaphore channel in `FetchAllProviders`
- Error wrapping with `%w` in infrastructure code
- Intentional non-wrapping of terminal user-facing errors
- No variable shadowing issues
- Good naming: short receivers, descriptive exports, consistent patterns
- `outWriter` swappable for tests
- Config `.clone()` returning copies from singleton to prevent shared state mutation
- Provider registration via `init()` + blank imports (standard `database/sql` driver pattern)
