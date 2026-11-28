# Phase 2: Phase 2: Surface inline combo heads-up on the pick board

## Done

### Task 2: Wire `analyzer.CheckComboConflict` into `internal/board/signals.go` (`Row.ComboWarning`) and render the compact non-blocking heads-up note on conflicting pick options in `web/templates/` and `internal/web/pick.go`, with tests in `internal/board/signals_test.go` and `internal/web/pick_test.go`.

- Added `Row.ComboWarning` to `internal/board/board.go` and wired `analyzer.CheckComboConflict` into `ApplySignals` in `internal/board/signals.go` for unpicked board rows.
- Mapped `Row.ComboWarning` onto `PickRowView.ComboWarning` in `internal/web/pick.go` and rendered a compact non-blocking `data-combo-warning` heads-up note in `web/templates/partials.html` (`pick-option`).
- Added unit tests in `internal/board/signals_test.go` (`TestApplySignalsComboWarning`) and `internal/web/pick_test.go` (`TestPickBoardRendersInlineComboWarningWithoutBlocking`).

### Task 3: Docs: update `docs/concepts/analyzer.md` to document the new combination-eligibility rules and their non-blocking behavior.

- Added the "Sportsbook combination eligibility" section to `docs/concepts/analyzer.md` documenting `book_combo_ineligible` (Danger) and `book_combo_restricted` (Warn), their triggering conditions, and their strictly non-blocking behavior.
- Documented both surfacing paths (`AnalyzeSlip` for the slip banner and `#pick-tray`, and `analyzer.CheckComboConflict` via `board.ApplySignals` for inline pick-board `Row.ComboWarning` notes) and updated the overview diagram's warning count from 16 to 18.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 2

- `Row.ComboWarning` is populated automatically in `board.ApplySignals` whenever `BoardSignals.SlipLegs` is present and `row.Picked == 0`.
- Both `pick-detail` and single-line-group HTMX refreshes (`board.LineGroupForLeg`) go through `ApplySignals`, so inline combo warnings stay current as legs are added or removed.

## Files touched

- `docs/concepts/analyzer.md`
- `internal/board/board.go`
- `internal/board/signals.go`
- `internal/board/signals_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
