# Phase 3: Phase 3: Open-slip line-drift indicator and one-tap refresh

## Done

### Task 4: Update `internal/slips/rules.go` (`pickChanged` / `UpdateLeg`) so syncing a moved line on an open slip preserves votes unless the canonical selection changed, and add a line-drift indicator + refresh action in `internal/web/pick.go` and `web/templates/partials.html`, with tests in `internal/slips/votes_test.go` and `internal/web/pick_test.go`.

- Updated `pickChanged` in `internal/slips/rules.go` so `UpdateLeg` preserves `leg_votes` when the canonical selection (`db.SameSelection` on non-`OTHER` markets) is unchanged and only the line, price, or selection text synced to market drift, while still clearing votes when the selection, side, player, team, market, or `OTHER` pick changes.
- Added line-drift detection and formatting (`legDrifted`, `boardDriftNote`, `trayDriftNote`, `refreshViewFromRow`) in `internal/web/pick.go` for both board rows (`PickRowView`) and tray legs (`TrayLegView` with `LegRefreshView`), and added `from=tray` handling to `updateLeg` in `internal/web/slips.go`.
- Rendered the drift indicator and one-tap `Sync line` (`hx-put="/legs/{{...}}"`) form in `pick-option` and `pick-tray` in `web/templates/partials.html`.
- Added unit and handler tests in `internal/slips/votes_test.go` (`TestUpdateLegKeepsVotesWhenBoardLineDrifts`, `TestUpdateLegClearsVotesWhenCanonicalSelectionChanges`) and `internal/web/pick_test.go` (`TestPickBoardAndTrayShowDriftAndSyncLine`, `TestPickTraySyncLineRefreshesTrayAndBoard`).

### Task 5: Docs: update `docs/concepts/slips.md` to document selection identity and open-slip line updates.

- Documented canonical selection identity (`db.SameSelection` in `internal/db/odds.go`) under `## Legs` in `docs/concepts/slips.md`, including player-prop matching (`PlayerID` or normalized name + team), game-market matching, and explicit preservation of `leg.Line` for grading.
- Documented `board.Row.MatchesLeg` keeping picked legs bound when lines move, drift notices on the board and pick tray (`legDrifted`), and the one-tap `Sync line` (`PUT /legs/{id}`) flow under `## The pick board`.
- Updated `## Votes` to reflect that `UpdateLeg` preserves votes when `db.SameSelection` matches on non-`OTHER` markets and clears them when the canonical selection or an `OTHER` pick changes.
- Updated four stale `internal/slips.Service*` / `internal/grader.GradeLeg` path references in `docs/concepts/slips.md` so `good-vibes docs lint` reports 0 errors and 0 warnings for `docs/concepts/slips.md`.

## Deviations

### Task 4

- Also added `from="tray"` handling to `updateLeg` in `internal/web/slips.go` so the one-tap `Sync line` action works directly from the pick tray as well as the board.

## Open issues

### Task 4

- None.

## Next-phase notes

### Task 4

- Phase 3's implementation and tests are in place; the next task can proceed with any remaining documentation/verification work.

### Task 5

All tasks in Phase 3 are complete.

## Files touched

- `docs/concepts/slips.md`
- `internal/slips/rules.go`
- `internal/slips/votes_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
