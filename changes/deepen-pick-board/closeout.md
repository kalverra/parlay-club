---
draft: true
---

# Close-out

## Consequences

- **Easier by construction:** Callers of `Store.OddsForGame` and `Store.OddsForWeek` receive only the current sync group per `(game, isPropRow)`, eliminating manual `currentOdds` filtering across web handlers and preventing stale prop players from leaking into game research.
- **Direct domain testing:** Picked-leg drift, slate resolution, `Unloaded` card state, and signal retention on partial line-group swaps are tested directly against `internal/board` rather than through rendered HTML.
- **Thinner web layer:** `internal/web/pick.go` maps `board.Slate`, `board.Row` drift fields, and `board.LineGroupForLeg` directly to view structs instead of re-indexing legs or scanning rows in multiple handlers.
- **Trade-off at the store seam:** Because `Store.OddsForGame` and `Store.OddsForWeek` now filter via `db.CurrentOdds`, any future feature needing superseded odds snapshots from earlier syncs will need a separate unfiltered query.
- **Display coupling in `board`:** `board.Build` now formats `BoardDriftNote` and `TrayDriftNote`, trading a small amount of UI copy in `internal/board` to remove duplicate formatting in `internal/web`.

## Alternatives considered

- **Keep `currentOdds` filtering and board assembly in `internal/web`:** Rejected because callers had to remember to filter after every odds query (which `loadGameResearch` missed), and partial-swap behavior like signal attachment and leg drift had to be verified through HTML tests.
- **Have `internal/board` query `db.Store` directly:** Not chosen because slate-only and single-game HTMX endpoints rely on scoped queries (`GameByID`, `OddsForGame`, two-team `RosterForTeams`) for performance; keeping `board.Build`, `board.BuildSlate`, and `board.LineGroupForLeg` over caller-supplied slices preserves that scoping.

## Implementation notes

- No deviations from the design.
- `LineGroup.HoldsLeg` guards `option.Picked == leg.ID` with `leg.ID != 0` so an unpersisted zero-ID leg cannot match unpicked options.
- `db.CurrentOdds` filters superseded sync rows on read rather than deleting them during `db.UpsertOdds`, so historical rows from dropped markets remain in SQLite.

## Follow-ups

None.
