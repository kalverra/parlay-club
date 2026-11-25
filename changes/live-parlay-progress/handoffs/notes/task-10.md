# Task 10: Document live parlay and leg progress bars in `docs/concepts/slips.md`

## Done

- Updated `docs/concepts/slips.md` front matter (`summary` and `tags`) to include live progress tracking.
- Documented spotlight card compact live progress bars and `Live X%` display on `GET /groups/{id}/week`.
- Added a `## Live progress` section covering when progress bars appear, the slip hub parlay card (`#slip-panel`), per-leg progress strips, `internal/progress` deterministic hit-chance and early-settlement rules, and automatic `slip-updated` SSE refreshes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 3 is complete; `docs/concepts/slips.md` now reflects the live parlay and leg progress UI and hit-chance behavior across the slip hub and group overview.

## Commit

Document live parlay progress in slips concept doc

Keep the living slip documentation accurate after adding live parlay and leg progress bars and deterministic hit-chance tracking to the slip hub and week spotlight cards.
