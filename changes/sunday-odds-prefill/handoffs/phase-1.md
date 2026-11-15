# Phase 1: Phase 1: Sunday pre-fill engine and authenticated endpoint

## Done

### Task 1: Add `RunPrefill` (scoreboard -> lines -> free summaries/rosters + bounded parallel Parlay-only bulk props for upcoming games with 15m idempotency), tighten free-source staleness windows (`scoreboard`, `summary`, `roster`, `weather`, `nfelo`, `gamelog`), promote `summary` and unique week-team `roster` syncs into `Kick()`, and split `Kick` from `RunOnce` so `Kick` omits `odds:lines` in `internal/refresh` (updating `TestRefreshArchivesEveryIngestCall` and adding unit/integration tests)

- Added `RunPrefill(ctx context.Context) (PrefillReport, error)` and `prefillProps` in `internal/refresh`: syncs
  scoreboard first, `odds:lines` (15-minute idempotency), free sources (`freeSourcesFor` with concurrency 4), and
  Parlay-only bulk props for upcoming games (`prefillConcurrency = 4`, 15-minute idempotency, skipping `FallbackProps`
  and kicked-off games, respecting `OddsPropsFloor`).
- Tightened free-source staleness windows in `internal/refresh/refresh.go` and `internal/refresh/rules.go`: `scoreboard`
  (5m live / 15m GD / 2h), `summary` (15m GD or <=4h to kickoff / 1h, skip `FINAL`), `roster` (2h GD / 6h), `weather`
  (15m <=6h / 30m <=48h), `nfelo` (1h GD / 4h), and `gamelog` (1h GD / 4h).
- Split `Kick()` from `RunOnce()`: `Kick()` uses `freeSourcesFor` (including `espn:summary:{gameID}` and deduped
  week-team `espn:roster:{teamID}` for non-final games) and omits `odds:lines`; `RunOnce()` retains `odds:lines` for
  sandbox and full-pass callers.
- Updated `TestRefreshArchivesEveryIngestCall` and `syncKeys` in `cmd/server/`, updated staleness rule tests in
  `internal/refresh/rules_test.go`, `nfelo_test.go`, `research_test.go`, and `sources_test.go`, and added
  `internal/refresh/prefill_test.go`.

### Task 2: Add optional `PrefillToken` (`PREFILL_TOKEN`) to `internal/config` and mount `POST /internal/prefill` outside cookie/CSRF middleware in `cmd/server` with constant-time bearer auth, synchronous execution, and handler/config tests

- Added optional `PrefillToken` (`PREFILL_TOKEN`, whitespace-trimmed) to `config.Config` and `config.Load` in
  `internal/config/config.go`, with tests in `internal/config/config_test.go`.
- Added `newPrefillHandler`, `validBearerToken` (using `subtle.ConstantTimeCompare`), and `prefillTimeout` (2 minutes,
  linked to `stopCtx`) in `cmd/server/main.go`.
- Mounted `POST /internal/prefill` on an outer `http.ServeMux` in `newMux` outside `handlers.CSRF`, `RequireUser`,
  `Scope`, and `kickOnPage`, returning `404` when `PREFILL_TOKEN` is empty, `401` on missing/invalid bearer token, `503`
  in sandbox mode, `200` JSON `PrefillReport` on success, and `500` JSON on `RunPrefill` error.
- Updated `newMux` callers in `cmd/server/main_test.go` and `cmd/server/sandbox_test.go` and added `TestPrefillEndpoint`
  covering `404`, `401`, `503`, synchronous `200` JSON without cookies/CSRF, `500`, and `stopCtx` cancellation.

### Task 3: Docs: amend the lazy-only sync guardrail in `AGENTS.md` and update `docs/concepts/data-refresh.md`, `docs/reference/config.md`, and `docs/reference/routes.md` for pre-fill and `Kick` scope

- Amended the free-tier guardrails in `AGENTS.md` to allow the Sunday morning pre-fill (`POST /internal/prefill`) and
  pick-time line/prop refreshes while scoping `Kick()` to free/unmetered sources only.
- Updated `docs/concepts/data-refresh.md` for `RunPrefill` (scoreboard -> `odds:lines` -> free sources -> Parlay-only
  bulk props with 15m idempotency), `Kick()` vs `RunOnce()` scope, promoted `espn:summary` and deduped week-team
  `espn:roster` syncs, and tightened free-source staleness windows.
- Updated `docs/reference/config.md` to document optional `PREFILL_TOKEN` and its fail-closed bearer auth behavior.
- Updated `docs/reference/routes.md` to document `POST /internal/prefill`, its outer-mux placement outside cookie/CSRF
  middleware, and its `401`/`404`/`503`/`200`/`500` responses.
- Verified documentation with `good-vibes docs lint` (`ok`).

## Deviations

### Task 1

- `cmd/server/sandbox_test.go` (`syncKeys`) was updated to include `espn:summary:{gameID}` and deduped
  `espn:roster:{teamID}` keys for non-final games, since `RunOnce()` now runs `freeSourcesFor` during sandbox morning
  bootstrap.
- Added a nil guard for `s.forecast` in `syncWeather` (`internal/refresh/sources.go`) matching the existing nil guards
  in `syncScoreboard`, `syncRoster`, `syncSummary`, and `syncGameLog`.
- Resolved pre-existing `golangci-lint` failures across `internal/record/record.go`, `internal/ingest/espn/boxscore.go`,
  `internal/grader/grader_test.go`, `internal/refresh/grade_test.go`, `internal/db/wagers_test.go`,
  `internal/db/record_test.go`, `internal/slips/wagers_test.go`, `internal/web/record_test.go`, and
  `internal/web/wagers_test.go` so the repository-wide lint gate passes.

## Open issues

### Task 1

- None.

## Next-phase notes

### Task 1

- `POST /internal/prefill` in Phase 2 can call `refresher.RunPrefill(ctx)` directly with a 2-minute timeout and encode
  the returned `PrefillReport` as JSON.

### Task 2

- Deployment scripts in the next task can wire the optional `prefill-token` secret as `PREFILL_TOKEN` and configure
  Cloud Scheduler to `POST /internal/prefill` with `Authorization: Bearer <token>`.

### Task 3

- Phase 2 will add pick-time on-demand refreshes (`SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`,
  `POST /slips/{id}/pick/refresh`) and update `docs/concepts/data-refresh.md` and `docs/reference/routes.md` for those
  endpoints and staleness windows.

## Files touched

- `AGENTS.md`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/data-refresh.md`
- `docs/reference/config.md`
- `docs/reference/routes.md`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/record_test.go`
- `internal/db/wagers.go`
- `internal/db/wagers_test.go`
- `internal/grader/grader.go`
- `internal/grader/grader_test.go`
- `internal/ingest/espn/boxscore.go`
- `internal/record/record.go`
- `internal/record/record_test.go`
- `internal/refresh/grade_test.go`
- `internal/refresh/nfelo_test.go`
- `internal/refresh/prefill_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`
- `internal/slips/rules.go`
- `internal/slips/wagers_test.go`
- `internal/web/record_test.go`
- `internal/web/wagers_test.go`

## Run log

- Mode:
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 1
