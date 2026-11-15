# Task 1 Handoff Note

## Done

- Added `RunPrefill(ctx context.Context) (PrefillReport, error)` and `prefillProps` in `internal/refresh`: syncs scoreboard first, `odds:lines` (15-minute idempotency), free sources (`freeSourcesFor` with concurrency 4), and Parlay-only bulk props for upcoming games (`prefillConcurrency = 4`, 15-minute idempotency, skipping `FallbackProps` and kicked-off games, respecting `OddsPropsFloor`).
- Tightened free-source staleness windows in `internal/refresh/refresh.go` and `internal/refresh/rules.go`: `scoreboard` (5m live / 15m GD / 2h), `summary` (15m GD or <=4h to kickoff / 1h, skip `FINAL`), `roster` (2h GD / 6h), `weather` (15m <=6h / 30m <=48h), `nfelo` (1h GD / 4h), and `gamelog` (1h GD / 4h).
- Split `Kick()` from `RunOnce()`: `Kick()` uses `freeSourcesFor` (including `espn:summary:{gameID}` and deduped week-team `espn:roster:{teamID}` for non-final games) and omits `odds:lines`; `RunOnce()` retains `odds:lines` for sandbox and full-pass callers.
- Updated `TestRefreshArchivesEveryIngestCall` and `syncKeys` in `cmd/server/`, updated staleness rule tests in `internal/refresh/rules_test.go`, `nfelo_test.go`, `research_test.go`, and `sources_test.go`, and added `internal/refresh/prefill_test.go`.

## Deviations

- `cmd/server/sandbox_test.go` (`syncKeys`) was updated to include `espn:summary:{gameID}` and deduped `espn:roster:{teamID}` keys for non-final games, since `RunOnce()` now runs `freeSourcesFor` during sandbox morning bootstrap.
- Added a nil guard for `s.forecast` in `syncWeather` (`internal/refresh/sources.go`) matching the existing nil guards in `syncScoreboard`, `syncRoster`, `syncSummary`, and `syncGameLog`.
- Resolved pre-existing `golangci-lint` failures across `internal/record/record.go`, `internal/ingest/espn/boxscore.go`, `internal/grader/grader_test.go`, `internal/refresh/grade_test.go`, `internal/db/wagers_test.go`, `internal/db/record_test.go`, `internal/slips/wagers_test.go`, `internal/web/record_test.go`, and `internal/web/wagers_test.go` so the repository-wide lint gate passes.

## Open issues

- None.

## Next-phase notes

- `POST /internal/prefill` in Phase 2 can call `refresher.RunPrefill(ctx)` directly with a 2-minute timeout and encode the returned `PrefillReport` as JSON.
