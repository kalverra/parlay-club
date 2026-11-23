# Phase 2: Phase 2: Merge summary + roster injuries and wire into analyzer, board, and research

## Done

### Task 4: Add `espn.IsKeyInjuryPosition` and `espn.MergeTeamInjuries` in `internal/ingest/espn/summary.go` with unit tests in `internal/ingest/espn/summary_test.go`.

- Added `espn.IsKeyInjuryPosition` in `internal/ingest/espn/summary.go` and reused it in `ParseInjuries` to normalize and check key injury positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`).
- Added `espn.MergeTeamInjuries` in `internal/ingest/espn/summary.go` to keep summary injuries first and append key-position fallback injuries not already present by `AthleteID` or case-insensitive `FullName`.
- Added `TestIsKeyInjuryPosition` and `TestMergeTeamInjuries` in `internal/ingest/espn/summary_test.go`.

### Task 5: Add `analyzer.Injury`, `Game.Injuries`, `PlayerStats.Position`, and `analyzer.MergeGameInjuries`, and populate them in `analyzer.LoadSlipSignals` (`internal/analyzer/signals.go`) from `games.injuries_json` + `roster_players`, with tests in `internal/analyzer/signals_test.go`.

- Added `analyzer.Injury`, `Game.Injuries`, and `PlayerStats.Position` in `internal/analyzer/analyzer.go`.
- Added `analyzer.MergeGameInjuries` in `internal/analyzer/signals.go` to parse `games.injuries_json` and backfill key-position injured players from `roster_players` via `espn.MergeTeamInjuries`.
- Updated `analyzer.LoadSlipSignals` to load `store.RosterForTeams` once for the slip's games, attach merged team injuries to `Game.Injuries`, and populate `PlayerStats.Position`.
- Added `TestMergeGameInjuries` and `TestLoadSlipSignalsPopulatesInjuriesAndPositions` in `internal/analyzer/signals_test.go`.

### Task 6: Reuse `analyzer.MergeGameInjuries` in `web.slipHandlers.boardSignalsForGame` (`internal/web/pick.go`) and `web.researchHandlers.loadGameResearch` (`internal/web/research.go`) so both the pick board's `Role bump` badge and Game Research see truncated roster injuries, with tests in `internal/web/pick_test.go` and `internal/web/research_test.go`.

- Replaced `parseGameInjuries` in `internal/web/pick.go` with `analyzer.MergeGameInjuries(game, rosters)` in `boardSignalsForGame`, loading team rosters via `RosterForTeams` so the pick board's `Role bump` badge sees roster-only `Out` teammates.
- Updated `buildInjuryViews` in `internal/web/research.go` to merge `games.injuries_json` with `roster_players` via `analyzer.MergeGameInjuries` while preserving team metadata and logos, and adding team cards for roster-only injuries when summary injuries omit a team.
- Added `TestPickBoardRoleBumpFromRosterInjury` in `internal/web/pick_test.go` and `TestResearchGameIncludesTruncatedRosterInjuries` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

`espn.IsKeyInjuryPosition` and `espn.MergeTeamInjuries` are ready for `analyzer.MergeGameInjuries` in `internal/analyzer/signals.go`.

### Task 5

`analyzer.MergeGameInjuries` is ready for `web.boardSignalsForGame` and `web.loadGameResearch`, and `Game.Injuries` / `PlayerStats.Position` are populated for teammate-injury grading in `ScoreLeg`.

### Task 6

Phase 2 is complete; Phase 3 can implement teammate-injury grading in `ScoreLeg` (`internal/analyzer/grade.go`), update `GradeRubric()` (`internal/analyzer/docs.go`), and update `docs/`.

## Files touched

- `internal/analyzer/analyzer.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/ingest/espn/summary.go`
- `internal/ingest/espn/summary_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: generate 500ms, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
