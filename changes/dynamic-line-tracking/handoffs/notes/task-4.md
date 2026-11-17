# Task 4

## Done

- Updated `pickChanged` in `internal/slips/rules.go` so `UpdateLeg` preserves `leg_votes` when the canonical selection (`db.SameSelection` on non-`OTHER` markets) is unchanged and only the line, price, or selection text synced to market drift, while still clearing votes when the selection, side, player, team, market, or `OTHER` pick changes.
- Added line-drift detection and formatting (`legDrifted`, `boardDriftNote`, `trayDriftNote`, `refreshViewFromRow`) in `internal/web/pick.go` for both board rows (`PickRowView`) and tray legs (`TrayLegView` with `LegRefreshView`), and added `from=tray` handling to `updateLeg` in `internal/web/slips.go`.
- Rendered the drift indicator and one-tap `Sync line` (`hx-put="/legs/{{...}}"`) form in `pick-option` and `pick-tray` in `web/templates/partials.html`.
- Added unit and handler tests in `internal/slips/votes_test.go` (`TestUpdateLegKeepsVotesWhenBoardLineDrifts`, `TestUpdateLegClearsVotesWhenCanonicalSelectionChanges`) and `internal/web/pick_test.go` (`TestPickBoardAndTrayShowDriftAndSyncLine`, `TestPickTraySyncLineRefreshesTrayAndBoard`).

## Deviations

- Also added `from="tray"` handling to `updateLeg` in `internal/web/slips.go` so the one-tap `Sync line` action works directly from the pick tray as well as the board.

## Open issues

- None.

## Next-phase notes

- Phase 3's implementation and tests are in place; the next task can proceed with any remaining documentation/verification work.
