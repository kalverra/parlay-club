## Done

- Added `ExportBookView.MultiGameNote`, `ExportBookView.Groups`, and `ExportGroupView` in `internal/web/export.go` to surface DraftKings' single-game explanation and per-game combined groups (with `AwayTeam @ HomeTeam` matchup labels) when a multi-game slip has 2+ linked legs in a game.
- Updated `manualNote` in `internal/web/export.go` to distinguish `ReasonNoLine` with `PropsSynced: true` ("isn't offering this prop right now, so add it by hand if it appears on their board.") from unsynced games ("has no line for this pick yet, so add it by hand.").
- Updated `slip-export` in `web/templates/partials.html` to render `MultiGameNote`, per-game combined taps with leg selections, prices, and drift notes alongside single-leg taps when `Groups` is populated, and the flat numbered list when every game has a single leg.
- Updated `TestExportCardIgnoresStaleOddsRows` and added `TestExportCardDraftKingsMultiGameWithCombinedGroup`, `TestExportCardDraftKingsMultiGameAllSingleLegs`, and `TestExportCardUnquotedPropVsUnsyncedGame` in `internal/web/export_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Task 6 is complete; the remaining task in Phase 2 can proceed with `docs/` updates and verification.

## Commit

Render multi-game DraftKings groups and prop notes

Multi-game DraftKings slips now explain the single-game URL limit, combine games with multiple legs into per-game taps, and tell users when a book simply does not quote a synced prop market.
