---
slug: dynamic-line-tracking
run: 20261117-025912-dynamic-line-tracking
outcome: done
generated: 2026-11-17T03:38:04Z
---

# Run summary: dynamic-line-tracking

## Outcome

- Status: done
- Branch: dynamic-line-tracking
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 3
- Checkpoints: 5
- Duration: 16m8s
- Cost: $—
- Fresh input tokens: 364752
- Cache read tokens: 4032276
- Cache write tokens: 0
- Output tokens: 68036

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 4465064 | $— |

## Phases

### Phase 1: Canonical selection identity in `db` and `board`

Tasks:

- [x] Add `SameSelection` in `internal/db/odds.go` (matching props by canonical player + market + side, ignoring `Line`) and update `board.Row.MatchesLeg`, `sameBoardLine`, and `groupHoldsLeg` in `internal/board/board.go` and `internal/web/pick.go` so picked legs stay bound when lines move, with tests.

Phase totals: 1 checkpoints, 3m55s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `SameSelection` in `internal/db/odds.go` (matching props by canonical player + market + side, ignoring `Line`) and update `board.Row.MatchesLeg`, `sameBoardLine`, and `groupHoldsLeg` in `internal/board/board.go` and `internal/web/pick.go` so picked legs stay bound when lines move, with tests.

- Added `SelectionKey`, `IsPlayerMarket`, `LegSelection`, `OddsSelection`, and `SameSelection` in `internal/db/odds.go`, and updated `Odds.MatchesSelection` to delegate to `SameSelection`.
- Updated `board.Row.MatchesLeg` in `internal/board/board.go` to match via `db.SameSelection` (ignoring `Line` for both player props and game markets) and delegated `isPlayerMarket` in `internal/board/board.go` and `internal/web/slips.go` to `db.IsPlayerMarket`.
- Updated `sameBoardLine` in `internal/web/pick.go` to match board line groups via `db.SameSelection` (with `Side` cleared for props and `TEAM_TOTAL`, and matching `GameID` + `MarketType` for `SPREAD`, `ML`, `TOTAL`) so picked legs stay bound to their `LineGroup` when lines move (`groupHoldsLeg` continues delegating to `option.MatchesLeg`).
- Added unit tests in `internal/db/odds_test.go`, `internal/board/board_test.go`, and `internal/web/pick_test.go`.

### Phase 2: Dynamic sportsbook export links with drift callout

Tasks:

- [x] Update `internal/export/export.go` (`matchOdds`, `planLeg`, `LegLink.LineMoved`) to resolve deep links by `SameSelection` across line moves and name variants, surface drift in `internal/web/export.go` and `web/templates/partials.html`, and add tests in `internal/export/export_test.go` and `internal/web/export_test.go`.
- [x] Docs: update `docs/concepts/export.md` to describe dynamic line linking and drift notes.

Phase totals: 2 checkpoints, 5m9s, $—, 2198514 input / 34018 output / 2016138 cache read / 0 cache write tokens

Checkpoints: task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Update `internal/export/export.go` (`matchOdds`, `planLeg`, `LegLink.LineMoved`) to resolve deep links by `SameSelection` across line moves and name variants, surface drift in `internal/web/export.go` and `web/templates/partials.html`, and add tests in `internal/export/export_test.go` and `internal/web/export_test.go`.

- Updated `internal/export/export.go` (`LegLink.LineMoved`, `planLeg`, `matchOdds`) to resolve deep links by `db.SameSelection` via `row.MatchesSelection(leg)` across line moves and player name variants, preferring an exact line match when multiple rows match and setting `LineMoved: !db.SameLine(row.Line, leg.Line)` instead of rejecting moved lines with `ReasonLineMoved`.
- Added `LineMoved` and `DriftNote` (`"Book at <live> · slip <saved>"`) to `ExportLinkView` in `internal/web/export.go` and rendered the drift callout in `web/templates/partials.html` for both per-leg links and combined `OpenURL` betslip links.
- Updated existing export tests and added prod Leg 19 (`"Ollie Gordon"` -> `"Ollie Gordon II"`) and prod Leg 20 (`248.5` -> `254.5` with `LineMoved: true` and combined betslip URL) coverage in `internal/export/export_test.go` and `internal/web/export_test.go`.

### Task 3: Docs: update `docs/concepts/export.md` to describe dynamic line linking and drift notes.

- Updated `docs/concepts/export.md` to document dynamic selection matching via `db.Odds.MatchesSelection` (`db.SameSelection`) across line moves and player name variants, preferring exact `db.SameLine` matches when multiple rows exist.
- Added a Mermaid flowchart in `docs/concepts/export.md` illustrating kickoff, `SameSelection`, `Link`, and `LineMoved` drift checks, and removed the retired `line moved` manual-reason row from the table.
- Documented how `LegLink.LineMoved` and `DriftNote` (`Book at <live> · slip <saved>`) appear in `docs/concepts/export.md` for both combined betslip buttons and per-leg tap lists, and removed retired `sources`/`last_verified` front matter fields so `good-vibes docs lint` passes with 0 errors.

### Phase 3: Open-slip line-drift indicator and one-tap refresh

Tasks:

- [x] Update `internal/slips/rules.go` (`pickChanged` / `UpdateLeg`) so syncing a moved line on an open slip preserves votes unless the canonical selection changed, and add a line-drift indicator + refresh action in `internal/web/pick.go` and `web/templates/partials.html`, with tests in `internal/slips/votes_test.go` and `internal/web/pick_test.go`.
- [x] Docs: update `docs/concepts/slips.md` to document selection identity and open-slip line updates.

Phase totals: 2 checkpoints, 7m4s, $—, 2198514 input / 34018 output / 2016138 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Update `internal/slips/rules.go` (`pickChanged` / `UpdateLeg`) so syncing a moved line on an open slip preserves votes unless the canonical selection changed, and add a line-drift indicator + refresh action in `internal/web/pick.go` and `web/templates/partials.html`, with tests in `internal/slips/votes_test.go` and `internal/web/pick_test.go`.

- Updated `pickChanged` in `internal/slips/rules.go` so `UpdateLeg` preserves `leg_votes` when the canonical selection (`db.SameSelection` on non-`OTHER` markets) is unchanged and only the line, price, or selection text synced to market drift, while still clearing votes when the selection, side, player, team, market, or `OTHER` pick changes.
- Added line-drift detection and formatting (`legDrifted`, `boardDriftNote`, `trayDriftNote`, `refreshViewFromRow`) in `internal/web/pick.go` for both board rows (`PickRowView`) and tray legs (`TrayLegView` with `LegRefreshView`), and added `from=tray` handling to `updateLeg` in `internal/web/slips.go`.
- Rendered the drift indicator and one-tap `Sync line` (`hx-put="/legs/{{...}}"`) form in `pick-option` and `pick-tray` in `web/templates/partials.html`.
- Added unit and handler tests in `internal/slips/votes_test.go` (`TestUpdateLegKeepsVotesWhenBoardLineDrifts`, `TestUpdateLegClearsVotesWhenCanonicalSelectionChanges`) and `internal/web/pick_test.go` (`TestPickBoardAndTrayShowDriftAndSyncLine`, `TestPickTraySyncLineRefreshesTrayAndBoard`).

### Task 5: Docs: update `docs/concepts/slips.md` to document selection identity and open-slip line updates.

- Documented canonical selection identity (`db.SameSelection` in `internal/db/odds.go`) under `## Legs` in `docs/concepts/slips.md`, including player-prop matching (`PlayerID` or normalized name + team), game-market matching, and explicit preservation of `leg.Line` for grading.
- Documented `board.Row.MatchesLeg` keeping picked legs bound when lines move, drift notices on the board and pick tray (`legDrifted`), and the one-tap `Sync line` (`PUT /legs/{id}`) flow under `## The pick board`.
- Updated `## Votes` to reflect that `UpdateLeg` preserves votes when `db.SameSelection` matches on non-`OTHER` markets and clears them when the canonical selection or an `OTHER` pick changes.
- Updated four stale `internal/slips.Service*` / `internal/grader.GradeLeg` path references in `docs/concepts/slips.md` so `good-vibes docs lint` reports 0 errors and 0 warnings for `docs/concepts/slips.md`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
