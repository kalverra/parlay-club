## Done

- Added `analyzer.Injury`, `Game.Injuries`, and `PlayerStats.Position` in `internal/analyzer/analyzer.go`.
- Added `analyzer.MergeGameInjuries` in `internal/analyzer/signals.go` to parse `games.injuries_json` and backfill key-position injured players from `roster_players` via `espn.MergeTeamInjuries`.
- Updated `analyzer.LoadSlipSignals` to load `store.RosterForTeams` once for the slip's games, attach merged team injuries to `Game.Injuries`, and populate `PlayerStats.Position`.
- Added `TestMergeGameInjuries` and `TestLoadSlipSignalsPopulatesInjuriesAndPositions` in `internal/analyzer/signals_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`analyzer.MergeGameInjuries` is ready for `web.boardSignalsForGame` and `web.loadGameResearch`, and `Game.Injuries` / `PlayerStats.Position` are populated for teammate-injury grading in `ScoreLeg`.

## Commit

Wire merged team injuries and positions into analyzer signals

ESPN summary reports cap injuries at five players per team, which hides key-position inactive teammates from slip grading. Merging stored summary injuries with roster injury statuses in `LoadSlipSignals` gives `ScoreLeg` complete team injury and player position context.
