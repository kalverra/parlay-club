## Done

- Added `prefillKickTimeout = 2 * time.Minute`, updated `RunPrefill` to ignore `odds.ErrNoAPIKey` on `linesSrc`, and persisted `SundayPrefillSource` (`"prefill:sunday"`) under `s.stopCtx` with `syncTimeout` when `!s.stopping()` and either `len(errs) == 0` or partial odds progress (`report.LinesSynced || report.PropsSynced > 0`) occurred in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go).
- Wired `maybeRunSundayPrefill` into `s.kick(ctx)` in [refresh.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/refresh.go), guarding with `sundayPrefillWindow`, 5-minute `paused`/`backOff`, `shouldRun`, `singleflight` re-check, `prefillKickTimeout`, and `freeSourcesFor` fallback on zero-progress failures.
- Updated `TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines` (using a Saturday 10:00 AM ET clock outside the Sunday pre-fill window) and added `TestKickRunsSundayPrefillOnceAndPersistsAcrossRestart`, `TestKickSundayPrefillSingleflightConcurrency`, `TestKickSundayPrefillZeroProgressBackoffAndFallback`, `TestKickSundayPrefillPartialFailureRecordsCompletion`, and `TestRunPrefillIgnoresMissingOddsAPIKeyAndMarksSundayDone` in [prefill_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/first-sunday-visit-prefill/internal/refresh/prefill_test.go).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Service.kick(ctx)` now triggers `RunPrefill` once per Sunday 06:00+ ET when upcoming Sunday games remain, and `RunPrefill` records `SundayPrefillSource` in `sync_state`.
- Task 3 / Phase 2 can add the `cmd/server/main_test.go` route/auth isolation integration tests and update `docs/`.
