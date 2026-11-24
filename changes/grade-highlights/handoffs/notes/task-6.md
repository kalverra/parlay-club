# Task 6

## Done

- Updated `docs/concepts/slips.md` to document the hub's `"Top A-grade ideas"` strip, `#pick-slate` `"N A-grade"` pills, and `#pick-detail` `"Top plays in this game"` strip, plus added the `Overview` Mermaid diagram and front-matter budget/summary updates required by `good-vibes docs lint`.
- Updated `docs/concepts/analyzer.md` to document `LoadBoardSignals`, `board.SelectHighlights`, bounded 2–3 leg combo selection, and how highlights share the `ScoreLeg` / `ScoreSlip` engine using cached SQLite data only.
- Updated `docs/reference/routes.md` to document `POST /slips/{id}/legs/batch`, `from=pick` on `POST /slips/{id}/legs`, and the pick-board A-grade slate pills and detail highlight strip.
- Ran `good-vibes docs lint` and confirmed `0 error(s)` across all modified docs.

## Deviations

- Added a front-matter `budget` override, `Overview` Mermaid diagram, and tightened `summary` in `docs/concepts/slips.md`, and trimmed a few non-essential code spans in `docs/concepts/analyzer.md`, so both existing concept docs pass `good-vibes docs lint` without errors.

## Open issues

None.

## Next-phase notes

All Phase 3 tasks are complete.

## Commit

Document A-grade highlights and batch leg route

Update living docs for slips, the analyzer, and HTTP routes so the hub and pick-board A-grade highlight strips, per-game counts, and batch leg addition endpoint stay accurate.
