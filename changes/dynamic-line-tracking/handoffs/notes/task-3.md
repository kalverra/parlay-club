## Done

- Updated `docs/concepts/export.md` to document dynamic selection matching via `db.Odds.MatchesSelection` (`db.SameSelection`) across line moves and player name variants, preferring exact `db.SameLine` matches when multiple rows exist.
- Added a Mermaid flowchart in `docs/concepts/export.md` illustrating kickoff, `SameSelection`, `Link`, and `LineMoved` drift checks, and removed the retired `line moved` manual-reason row from the table.
- Documented how `LegLink.LineMoved` and `DriftNote` (`Book at <live> · slip <saved>`) appear in `docs/concepts/export.md` for both combined betslip buttons and per-leg tap lists, and removed retired `sources`/`last_verified` front matter fields so `good-vibes docs lint` passes with 0 errors.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- Phase 2 is complete. Phase 3 can proceed with board selection controls (`internal/board/board.go`, `internal/web/pick.go`), one-tap open-slip line refresh, and preserving `leg_votes` on line-drift updates in `internal/slips/rules.go`.
