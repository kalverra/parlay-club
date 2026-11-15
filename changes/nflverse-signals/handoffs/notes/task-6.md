# Task 6

## Done

- Updated `docs/concepts/analyzer.md` with a Mermaid overview diagram, leg/slip `A`–`F` grading (`ScoreLeg`/`ScoreSlip`), clickable factor breakdowns in `#slip-panel` and `#pick-tray`, the `/analyzer/guide` page, and `nflverse` team EPA / player usage inputs with their 7-day freshness guard.
- Updated `docs/reference/analyzer-rules.md` with `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` triggers and examples, plus the full `A`–`F` leg and slip grading rubric (`GradeRubric()`).
- Updated `docs/reference/routes.md` with `GET /analyzer/guide` (including `RuleDocs()`/`GradeRubric()` backing, anchor IDs, and `?from=` `safeReturnPath` behavior).
- Ran `good-vibes docs lint` (0 errors) and `good-vibes docs index` (updated `docs/llms.txt`).

## Deviations

- Removed retired `sources` and `last_verified` front-matter fields from the three updated docs so `good-vibes docs lint` passes with 0 errors.

## Open issues

- None.

## Next-phase notes

- Phase 3 will add pick-board badges (`Smash matchup`, `High volume`, `Role bump`, `Fits slip`), `SortMatchup`, and research tab `nflverse` metrics; update `docs/reference/routes.md` (pick board query contract / research section) and `docs/concepts/slips.md` if needed in Phase 3's docs task.
