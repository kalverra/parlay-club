# Phase 1: Phase 1: Slip live step and expanded progress model

## Done

### Task 1: Slip live step

- Added `StepLive` (`"live"`, label `"Live"`) to `internal/slips/next.go`.
- Updated `NextStep` to transition locked, non-graded slips to `StepLive` via `legGamesStarted` when any leg references
  a `LIVE`, `FINAL`, or at/past-kickoff game, while keeping `StepExport` for pre-kickoff and unlinked-only slips.
- Added `TestNextStepLiveTransitions` in `internal/slips/next_test.go` covering `LIVE`, `FINAL`, at-kickoff,
  past-kickoff, `everyoneLocked`, late-game-only, unlinked-only, graded, and unlocked cases.

### Task 2: Expanded progress model

- Extended `LegProgress` with `GameProgressPct`, `ProbDelta`, and `ProbTrend` in `internal/progress/progress.go`.
- Extended `SlipProgress` with `ScheduledCount`, `ProbDelta`, and `ProbTrend` in `internal/progress/progress.go`.
- Updated `EvaluateLeg` to populate `GameProgressPct` from `RemainingFraction` and `ProbDelta`/`ProbTrend` (with a
  `0.005` deadband, `0`/`"flat"` for non-counting `PUSH`/`VOID` legs) across every leg state.
- Updated `EvaluateSlip` in `internal/progress/slip.go` to track `ScheduledCount`, format live `Summary` strings with
  upcoming counts (e.g. `"1/4 hit · 2 live · 1 upcoming"`), and compute slip-level `ProbDelta` and `ProbTrend`.
- Added unit tests for `GameProgressPct`, `ProbDelta`, `ProbTrend`, and `ScheduledCount` in
  `internal/progress/progress_test.go` and updated the existing mixed-slip summary assertion in
  `internal/progress/props_test.go`.

### Task 3: Lifecycle and progress docs

- Updated `docs/concepts/slips.md` to add `live` (`StepLive`) to the hub step table and document when a locked,
  non-graded slip transitions from `export` to `live` versus staying in `export`.
- Updated `docs/concepts/slips.md` live progress section for per-leg game-elapsed progress, before-vs-current
  probability delta and trend, and slip-level upcoming leg counts.
- Updated `docs/reference/odds-math.md` to document `GameProgressPct`, `ProbDelta`, `ProbTrend` (`0.005` deadband),
  `ScheduledCount`, updated live `Summary` formatting, and the `internal/progress` API table entries.
- Verified with `good-vibes docs lint` (`0 error(s)`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`NextStep` checks `legGamesStarted` against the already-filtered `games` slice for the slip, so callers must continue
passing the week's games. Downstream `internal/web` handlers (`slips.go`, `home.go`) will need to handle
`slips.StepLive` in later tasks.

### Task 2

- `EvaluateLeg` now populates `GameProgressPct`, `ProbDelta`, `ProbTrend`, `PreGameProb`, and `LiveProb` on all states
  (including `StateScheduled` and `StateUnavailable`), so `internal/web` can build `LegLiveView` for every leg in Live
  mode without special-casing missing fields.
- `EvaluateSlip` includes `"· N upcoming"` in `Summary` when `Show` is true and `ScheduledCount > 0`.

### Task 3

`docs/concepts/slips.md` is at its `585`-line budget; Phase 2 and Phase 3 doc updates to `slips.md` should either
tighten existing prose or update `budget.max_lines` in front matter.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/odds-math.md`
- `internal/progress/progress.go`
- `internal/progress/progress_test.go`
- `internal/progress/props_test.go`
- `internal/progress/slip.go`
- `internal/slips/next.go`
- `internal/slips/next_test.go`

## Run log

- Mode:
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 600ms; skipped: generate (no matching files), lint (no matching files), actionlint (no
  matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
