# Task 5: Docs: update `docs/concepts/slips.md`

## Done

- Documented canonical selection identity (`db.SameSelection` in `internal/db/odds.go`) under `## Legs` in `docs/concepts/slips.md`, including player-prop matching (`PlayerID` or normalized name + team), game-market matching, and explicit preservation of `leg.Line` for grading.
- Documented `board.Row.MatchesLeg` keeping picked legs bound when lines move, drift notices on the board and pick tray (`legDrifted`), and the one-tap `Sync line` (`PUT /legs/{id}`) flow under `## The pick board`.
- Updated `## Votes` to reflect that `UpdateLeg` preserves votes when `db.SameSelection` matches on non-`OTHER` markets and clears them when the canonical selection or an `OTHER` pick changes.
- Updated four stale `internal/slips.Service*` / `internal/grader.GradeLeg` path references in `docs/concepts/slips.md` so `good-vibes docs lint` reports 0 errors and 0 warnings for `docs/concepts/slips.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

All tasks in Phase 3 are complete.
