# Phase 2: Phase 2: Multi-game DraftKings grouping and clearer manual prop guidance

## Done

### Task 5: Extend `export.BookPlan` in `internal/export/export.go` with `LinkGroup` per-event grouping, `MultiGameNote` for multi-game DraftKings slips, and synced-prop awareness on `ReasonNoLine` manual legs, with unit tests in `internal/export/export_test.go`.

- Added `LinkGroup` and `BookPlan.Groups` / `BookPlan.MultiGameNote` in `internal/export/export.go` with per-event grouping in `draftKingsCombiner.CombineGroups` when linked DraftKings legs span multiple games.
- Added `ManualLeg.PropsSynced` in `internal/export/export.go` so `ReasonNoLine` on player and team-total prop legs indicates whether the game already has stored prop rows for other markets or books.
- Added unit tests in `internal/export/export_test.go` covering single-game vs multi-game DraftKings grouping, first-appearance order, missing `Sid` handling, and `ReasonNoLine` `PropsSynced` detection.

### Task 6: Update `internal/web/export.go` and `web/templates/partials.html` (`slip-export`) to render DraftKings' single-game explanation, per-game combined taps when a multi-game slip has 2+ legs in one game, and clearer unquoted-prop manual notes, with tests in `internal/web/export_test.go`.

- Added `ExportBookView.MultiGameNote`, `ExportBookView.Groups`, and `ExportGroupView` in `internal/web/export.go` to surface DraftKings' single-game explanation and per-game combined groups (with `AwayTeam @ HomeTeam` matchup labels) when a multi-game slip has 2+ linked legs in a game.
- Updated `manualNote` in `internal/web/export.go` to distinguish `ReasonNoLine` with `PropsSynced: true` ("isn't offering this prop right now, so add it by hand if it appears on their board.") from unsynced games ("has no line for this pick yet, so add it by hand.").
- Updated `slip-export` in `web/templates/partials.html` to render `MultiGameNote`, per-game combined taps with leg selections, prices, and drift notes alongside single-leg taps when `Groups` is populated, and the flat numbered list when every game has a single leg.
- Updated `TestExportCardIgnoresStaleOddsRows` and added `TestExportCardDraftKingsMultiGameWithCombinedGroup`, `TestExportCardDraftKingsMultiGameAllSingleLegs`, and `TestExportCardUnquotedPropVsUnsyncedGame` in `internal/web/export_test.go`.

### Task 7: Docs: update `docs/concepts/export.md` to document multi-game DraftKings per-game grouping, the single-game note, and the unquoted-prop manual note.

- Updated `docs/concepts/export.md` to document `BookPlan.Groups` (`LinkGroup`) and `BookPlan.MultiGameNote` for multi-game DraftKings per-game grouping.
- Documented the DraftKings single-game heads-up note and per-game combined button layout in the export card section.
- Documented `ManualLeg.PropsSynced` and the unquoted-prop manual note that distinguishes an unquoted market from an unsynced game.
- Added the `## Overview` Mermaid diagram and trimmed redundant inline code spans so `good-vibes docs lint` passes on `docs/concepts/export.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 5

`internal/web/export.go` and `web/templates/partials.html` can now render `BookPlan.MultiGameNote`, `BookPlan.Groups` (per-game combined taps and single-leg taps), and `ManualLeg.PropsSynced` for `ReasonNoLine` manual notes.

### Task 6

Task 6 is complete; the remaining task in Phase 2 can proceed with `docs/` updates and verification.

### Task 7

Phase 2 is complete.

## Files touched

- `docs/concepts/export.md`
- `internal/export/export.go`
- `internal/export/export_test.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `web/static/app.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6, task-7
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
