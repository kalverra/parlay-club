## Done

- Added the "Sportsbook combination eligibility" section to `docs/concepts/analyzer.md` documenting `book_combo_ineligible` (Danger) and `book_combo_restricted` (Warn), their triggering conditions, and their strictly non-blocking behavior.
- Documented both surfacing paths (`AnalyzeSlip` for the slip banner and `#pick-tray`, and `analyzer.CheckComboConflict` via `board.ApplySignals` for inline pick-board `Row.ComboWarning` notes) and updated the overview diagram's warning count from 16 to 18.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Document sportsbook combo-eligibility rules in analyzer concept doc

Explain the new hard-block and SGP-restriction rules, where they surface across the slip banner and pick board, and their non-blocking guarantee so future contributors and readers have an accurate reference.
