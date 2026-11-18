# Task 4

## Done

- Updated `docs/concepts/slips.md` to reflect `db.CurrentOdds` filtering at the storage seam (`Store.OddsForWeek` / `Store.OddsForGame`), picked-leg drift fields on `board.Row` (`PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, `TrayDriftNote`), `board.BuildSlate`, and `board.LineGroupForLeg` / `LineGroup.HoldsLeg`.
- Replaced the stale `legDrifted` reference in `docs/concepts/slips.md` with `board.Row.Drifted`, `BoardDriftNote`, and `TrayDriftNote`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 2 is complete and ready for the binary verification gates.
