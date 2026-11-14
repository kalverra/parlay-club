# Phase 2: Phase 2: Lazy `grade` refresh and manual override UI

## Done

### Task 3: Add `PendingLegsOnFinalGames` in `internal/db`, `slips.Service.AutoGradeLegs` (`WHERE result = 'PENDING'`, `graded_by = "auto"`, system activity, `slip:{id}` notify), and the `grade` source in `internal/refresh` (with game-only score optimization, error backoff, and integration tests proving manual overrides survive re-runs)

- Added `PendingFinalLeg`, `Store.PendingLegsOnFinalGames`, and `Store.SetLegResultIfPending` (`WHERE id = ? AND result = 'PENDING'`) in `internal/db/legs.go` with unit tests in `internal/db/legs_test.go`.
- Added `slips.AutoGraderName` (`"auto"`), `slips.AutoGrade`, and `slips.Service.AutoGradeLegs` in `internal/slips/rules.go`, recomputing slip results/statuses via `gradeSlip`, writing system activity (`user_id = NULL`), and publishing `slip:{id}` only for slips with updated legs, with unit tests in `internal/slips/grade_test.go`.
- Added `GradeSource` (`"grade"`), `BoxScoreClient`, `SlipGrader`, and `grade.go` in `internal/refresh` with game-only score optimization (skipping `BoxScore` when a final game has no pending player-prop legs), per-game 5-minute error backoff, and singleflight + archive-keyed `BoxScore` fetches.
- Added integration tests in `internal/refresh/grade_test.go` covering game-only optimization, player-prop grading and zero-waste re-runs, manual override durability across re-runs, and per-game `BoxScore` error backoff.

### Task 4: Surface `GradedBy` and `NEEDS_GRADING` amber styling on `LegView` in `internal/web/slips.go` and `web/templates/partials.html`, with handler/render tests

- Added `GradedBy` to `LegView` and populated it in `legViews` via `legGradedBy` (`internal/web/slips.go`), mapping `slips.AutoGraderName` (`"auto"`) to `"Auto"` and preserving member names.
- Updated `resultClass` in `internal/web/slips.go` to return `border-amber-700 bg-amber-950 text-amber-300` for `NEEDS_GRADING`, and added `resultLabel` (registered in `templateFuncs` in `internal/web/render.go`) to render `"Needs grading"` for `NEEDS_GRADING` and `"{RESULT} · {GradedBy}"` when graded.
- Updated both the interactive `<summary>` chip and read-only `<span>` chip in `web/templates/partials.html` to render `{{resultLabel .Result .GradedBy}}`, keeping the five override options (`PENDING`, `WON`, `LOST`, `PUSH`, `VOID`) in the menu.
- Added `TestSlipPanelRendersGradedByAndNeedsGradingAmberChip` in `internal/web/slips_test.go` verifying `Auto` vs member attribution, `NEEDS_GRADING` amber styling and label, the five override menu values, and manual override updating the chip.

### Task 5: Docs: update `docs/concepts/data-refresh.md` and `docs/concepts/slips.md` for `grade` and auto-grading behavior

- Updated `docs/concepts/data-refresh.md` to include the `grade` source in the pass flow and staleness table, and added an `## Auto-grading` section covering `PendingLegsOnFinalGames`, game-only score optimization, singleflight + per-game 5-minute `BoxScore` error backoff under `SummarySource(gameID)`, and `slips.Service.AutoGradeLegs`.
- Updated `docs/concepts/slips.md` (`## Grading`) to replace the future-tense auto-grader note with the implemented `grade` + `AutoGradeLegs` behavior, `WHERE result = 'PENDING'` override protection, `NEEDS_GRADING` handling, `GradedBy` (`"Auto"` vs member name) attribution, amber chip styling, and the five-option manual override menu.
- Updated frontmatter `sources` and `last_verified: 2026-11-14` in both docs and verified with `good-vibes docs lint` (`ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

- `refresh.New` automatically wires `BoxScore` from `Deps.Scoreboard` (which is `*espn.Client` in `cmd/server/main.go` and `cmd/server/sandbox.go`) and defaults `Deps.Slips` to `slips.New(deps.Store, deps.Publisher, now)` when omitted, so callers do not need extra wiring unless passing a custom mock.
- Failed `BoxScore` fetches back off per-game under `SummarySource(gameID)` for `failureBackoff` (`5m`) without blocking other `FINAL` games from grading.

### Task 4

- `resultLabel` and `resultClass` are registered on `templateFuncs()` in `internal/web/render.go` and shared across both member and non-member leg views in `web/templates/partials.html`.

### Task 5

- No code changes were made in this task; all docs reflect the Phase 2 implementation in `internal/refresh/grade.go`, `internal/slips/rules.go`, `internal/web/slips.go`, and `web/templates/partials.html`.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/concepts/slips.md`
- `internal/db/legs.go`
- `internal/db/legs_test.go`
- `internal/refresh/grade.go`
- `internal/refresh/grade_test.go`
- `internal/refresh/refresh.go`
- `internal/slips/grade_test.go`
- `internal/slips/rules.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
