# Task 7

## Done

- Updated `docs/concepts/export.md` to document `BookPlan.Groups` (`LinkGroup`) and `BookPlan.MultiGameNote` for multi-game DraftKings per-game grouping.
- Documented the DraftKings single-game heads-up note and per-game combined button layout in the export card section.
- Documented `ManualLeg.PropsSynced` and the unquoted-prop manual note that distinguishes an unquoted market from an unsynced game.
- Added the `## Overview` Mermaid diagram and trimmed redundant inline code spans so `good-vibes docs lint` passes on `docs/concepts/export.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 2 is complete.

## Commit

Document DraftKings grouping and prop notes

DraftKings combined links are scoped to one game, and prop legs on already-synced games distinguish unquoted markets from unsynced games. Updating the export concept doc keeps the living documentation aligned with the new BookPlan grouping, card layout, and manual notes.
