# Phase 2: Phase 2: Dynamic sportsbook export links with drift callout

## Done

### Task 2: Update `internal/export/export.go` (`matchOdds`, `planLeg`, `LegLink.LineMoved`) to resolve deep links by `SameSelection` across line moves and name variants, surface drift in `internal/web/export.go` and `web/templates/partials.html`, and add tests in `internal/export/export_test.go` and `internal/web/export_test.go`.

- Updated `internal/export/export.go` (`LegLink.LineMoved`, `planLeg`, `matchOdds`) to resolve deep links by `db.SameSelection` via `row.MatchesSelection(leg)` across line moves and player name variants, preferring an exact line match when multiple rows match and setting `LineMoved: !db.SameLine(row.Line, leg.Line)` instead of rejecting moved lines with `ReasonLineMoved`.
- Added `LineMoved` and `DriftNote` (`"Book at <live> · slip <saved>"`) to `ExportLinkView` in `internal/web/export.go` and rendered the drift callout in `web/templates/partials.html` for both per-leg links and combined `OpenURL` betslip links.
- Updated existing export tests and added prod Leg 19 (`"Ollie Gordon"` -> `"Ollie Gordon II"`) and prod Leg 20 (`248.5` -> `254.5` with `LineMoved: true` and combined betslip URL) coverage in `internal/export/export_test.go` and `internal/web/export_test.go`.

### Task 3: Docs: update `docs/concepts/export.md` to describe dynamic line linking and drift notes.

- Updated `docs/concepts/export.md` to document dynamic selection matching via `db.Odds.MatchesSelection` (`db.SameSelection`) across line moves and player name variants, preferring exact `db.SameLine` matches when multiple rows exist.
- Added a Mermaid flowchart in `docs/concepts/export.md` illustrating kickoff, `SameSelection`, `Link`, and `LineMoved` drift checks, and removed the retired `line moved` manual-reason row from the table.
- Documented how `LegLink.LineMoved` and `DriftNote` (`Book at <live> · slip <saved>`) appear in `docs/concepts/export.md` for both combined betslip buttons and per-leg tap lists, and removed retired `sources`/`last_verified` front matter fields so `good-vibes docs lint` passes with 0 errors.

## Deviations

### Task 2

- None.

### Task 3

- None.

## Open issues

### Task 2

- None.

### Task 3

- None.

## Next-phase notes

### Task 2

- Phase 3 can now focus on board selection controls (`internal/board/board.go`, `internal/web/pick.go`), one-tap open-slip line refresh, and preserving `leg_votes` on line-drift updates in `internal/slips/rules.go`.

### Task 3

- Phase 2 is complete. Phase 3 can proceed with board selection controls (`internal/board/board.go`, `internal/web/pick.go`), one-tap open-slip line refresh, and preserving `leg_votes` on line-drift updates in `internal/slips/rules.go`.

## Files touched

- `docs/concepts/export.md`
- `internal/export/export.go`
- `internal/export/export_test.go`
- `internal/web/export.go`
- `internal/web/export_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
