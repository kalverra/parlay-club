## Done

- Replaced `parseGameInjuries` in `internal/web/pick.go` with `analyzer.MergeGameInjuries(game, rosters)` in `boardSignalsForGame`, loading team rosters via `RosterForTeams` so the pick board's `Role bump` badge sees roster-only `Out` teammates.
- Updated `buildInjuryViews` in `internal/web/research.go` to merge `games.injuries_json` with `roster_players` via `analyzer.MergeGameInjuries` while preserving team metadata and logos, and adding team cards for roster-only injuries when summary injuries omit a team.
- Added `TestPickBoardRoleBumpFromRosterInjury` in `internal/web/pick_test.go` and `TestResearchGameIncludesTruncatedRosterInjuries` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 2 is complete; Phase 3 can implement teammate-injury grading in `ScoreLeg` (`internal/analyzer/grade.go`), update `GradeRubric()` (`internal/analyzer/docs.go`), and update `docs/`.

## Commit

Reuse merged roster injuries in board and research

Summary injury feeds from ESPN are capped at five players per team, which hid truncated key-position injuries from the pick board's Role bump badge and Game Research. Reusing `analyzer.MergeGameInjuries` in both web handlers ensures both views surface roster-backed injuries consistently.
