# Phase 1: Phase 1: Sunday window rules, persistence, and `Kick` orchestration

## Done

### Task 1: Add `SundayPrefillSource`, `sundayPrefillStartHour`, `sundayPrefillWindow`, and `sundayPrefillStale` in `internal/refresh/rules.go` and `refresh.go` with table-driven unit tests in `internal/refresh/rules_test.go`

- Added `SundayPrefillSource = "prefill:sunday"` and `sundayPrefillStartHour = 6` in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go).
- Added `sundayPrefillWindow` and `sundayPrefillStale` in [rules.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/rules.go) using the injected `eastern` (`America/New_York`) location and `lastKickoffOn`.
- Added table-driven unit tests `TestSundayPrefillWindow` and `TestSundayPrefillStale` in [rules_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/rules_test.go), covering weekday/Saturday/early-Sunday exclusions, last-Sunday-kickoff cutoffs with remaining Monday games, empty/Sunday-less slates, and DST calendar-date boundaries.

### Task 2: Update `RunPrefill` (ignore `odds.ErrNoAPIKey` on lines, persist `SundayPrefillSource` under `s.stopCtx` on success or partial odds progress) and wire `maybeRunSundayPrefill` into `s.kick(ctx)` with `singleflight`, `prefillKickTimeout`, 5-minute failure backoff, and `freeSourcesFor` fallback, updating `TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines` and adding orchestration, cold-start persistence, concurrency, and failure tests in `internal/refresh/prefill_test.go`

- Added `prefillKickTimeout = 2 * time.Minute`, updated `RunPrefill` to ignore `odds.ErrNoAPIKey` on `linesSrc`, and persisted `SundayPrefillSource` (`"prefill:sunday"`) under `s.stopCtx` with `syncTimeout` when `!s.stopping()` and either `len(errs) == 0` or partial odds progress (`report.LinesSynced || report.PropsSynced > 0`) occurred in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go).
- Wired `maybeRunSundayPrefill` into `s.kick(ctx)` in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go), guarding with `sundayPrefillWindow`, 5-minute `paused`/`backOff`, `shouldRun`, `singleflight` re-check, `prefillKickTimeout`, and `freeSourcesFor` fallback on zero-progress failures.
- Updated `TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines` (using a Saturday 10:00 AM ET clock outside the Sunday pre-fill window) and added `TestKickRunsSundayPrefillOnceAndPersistsAcrossRestart`, `TestKickSundayPrefillSingleflightConcurrency`, `TestKickSundayPrefillZeroProgressBackoffAndFallback`, `TestKickSundayPrefillPartialFailureRecordsCompletion`, and `TestRunPrefillIgnoresMissingOddsAPIKeyAndMarksSundayDone` in [prefill_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/prefill_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `SundayPrefillSource`, `sundayPrefillWindow`, and `sundayPrefillStale` are ready for `Service.kick` and `Service.RunPrefill` in Task 2.

### Task 2

- `Service.kick(ctx)` now triggers `RunPrefill` once per Sunday 06:00+ ET when upcoming Sunday games remain, and `RunPrefill` records `SundayPrefillSource` in `sync_state`.
- Task 3 / Phase 2 can add the `cmd/server/main_test.go` route/auth isolation integration tests and update `docs/`.

## Files touched

- `internal/refresh/prefill_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: lint reused, test reused
- Rounds used: 1 of 2; verify failures: 0
