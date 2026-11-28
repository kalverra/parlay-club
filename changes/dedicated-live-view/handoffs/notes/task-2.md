# Task 2

## Done

- Extended `LegProgress` with `GameProgressPct`, `ProbDelta`, and `ProbTrend` in `internal/progress/progress.go`.
- Extended `SlipProgress` with `ScheduledCount`, `ProbDelta`, and `ProbTrend` in `internal/progress/progress.go`.
- Updated `EvaluateLeg` to populate `GameProgressPct` from `RemainingFraction` and `ProbDelta`/`ProbTrend` (with a
  `0.005` deadband, `0`/`"flat"` for non-counting `PUSH`/`VOID` legs) across every leg state.
- Updated `EvaluateSlip` in `internal/progress/slip.go` to track `ScheduledCount`, format live `Summary` strings with
  upcoming counts (e.g. `"1/4 hit · 2 live · 1 upcoming"`), and compute slip-level `ProbDelta` and `ProbTrend`.
- Added unit tests for `GameProgressPct`, `ProbDelta`, `ProbTrend`, and `ScheduledCount` in
  `internal/progress/progress_test.go` and updated the existing mixed-slip summary assertion in
  `internal/progress/props_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `EvaluateLeg` now populates `GameProgressPct`, `ProbDelta`, `ProbTrend`, `PreGameProb`, and `LiveProb` on all states
  (including `StateScheduled` and `StateUnavailable`), so `internal/web` can build `LegLiveView` for every leg in Live
  mode without special-casing missing fields.
- `EvaluateSlip` includes `"· N upcoming"` in `Summary` when `Show` is true and `ScheduledCount > 0`.

## Commit

Extend progress model with game pct and prob delta

Add GameProgressPct, ProbDelta, ProbTrend, and ScheduledCount to LegProgress and SlipProgress so the dedicated Live view
can display game-elapsed progress and before-vs-current probability deltas for every leg and slip state.
