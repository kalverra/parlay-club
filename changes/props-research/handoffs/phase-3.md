# Phase 3: median_trap analyzer rule

## Done

- Implemented `median_trap` analyzer rule in `internal/analyzer`:
  - Evaluates `OVER` yardage legs (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) with a matched player (`PlayerID != ""`) and at least 3 games in the season log.
  - Computes the median of the player's last 5 games and the season mean across all games.
  - Triggers a `Warn` alert when `*leg.Line > median`. Formats message: `"{Player}'s last-5 median is {median} yards, so {line} is a stretch."`
  - When `*leg.Line < mean`, appends `" His {mean}-yard average is propped up by one big game."`
- Table-driven unit tests in `internal/analyzer/analyzer_test.go`:
  - Covered line above median, line at or below median, fewer than 3 games, unmatched player, `UNDER` side, and non-yardage markets (`RECEPTIONS`, `PASS_TDS`).
  - Verified presence and absence of the propped-up average sentence based on `line < mean`.
- Stored game log wiring in `internal/web`:
  - Added `playerStatsForLegs` to fetch stored `player_gamelogs` for all distinct matched player IDs on a slip and unmarshal their stat series.
  - Wired player stats into `analyzerView` in `internal/web/slips.go` and `internal/web/analyzer.go`.
  - Added handler tests `TestSlipPanelBannerMedianTrap` and `TestSlipPanelBannerMedianTrapQuietCases` in `internal/web/analyzer_test.go`.
- Documentation updates:
  - Updated `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to document `median_trap`, required stats inputs, and quiet conditions.
  - Pruned the built `median_trap` section from `plan/analyzer.md`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- Phase 4 implements the player prop quick-add pills, player form autocomplete, and the Research tab:
  - Add `GET /games/{id}/props` for on-demand prop quick-add pills on expanded game cards.
  - Add player `<datalist>` autocomplete in the leg form, with automatic `player_id` and `team` resolution when saving legs.
  - Build `GET /research`, `GET /research/games/{id}`, `GET /research/players/{espnID}`, and `POST /research/matches` (hit bars with median/mean markers, season games played, position-filtered key injuries, manual match resolution).
  - Update `docs/concepts/slips.md` and `docs/reference/routes.md`, and prune built sections from `plan/product.md` and `plan/architecture.md`.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/rules.go`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/slips.go`
- `plan/analyzer.md`

## Run log

- Mode: 
- Checkpoints: task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
