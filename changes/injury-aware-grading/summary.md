---
slug: injury-aware-grading
run: 20261123-183606-injury-aware-grading
outcome: done
generated: 2026-11-23T19:01:05Z
---

# Run summary: injury-aware-grading

## Outcome

- Status: done
- Branch: injury-aware-grading
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 8 of 8
- Phases: 3
- Checkpoints: 9
- Duration: 21m34s
- Cost: $—
- Fresh input tokens: 1157168
- Cache read tokens: 8556287
- Cache write tokens: 0
- Output tokens: 132941

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 9846396 | $— |

## Phases

### Phase 1: Ingest and persist ESPN roster injuries

Tasks:

- [x] Parse `injuries` (`status`, `date`) into `espn.RosterPlayer` in `internal/ingest/espn/roster.go` and add unit tests against real fixtures in `internal/ingest/espn/roster_test.go`.
- [x] Add migration `internal/db/migrations/0014_roster_injuries.sql`, extend `db.RosterPlayer` queries in `internal/db/players.go`, and wire `syncRoster` in `internal/refresh/research.go`, with tests in `internal/db/players_test.go`, `internal/db/migrate_test.go`, and `internal/refresh/research_test.go`.
- [x] Docs: update `docs/reference/schema.md` and `docs/reference/data-sources.md` for `roster_players.injury_status` / `injury_date` and ESPN roster injury ingestion.

Phase totals: 3 checkpoints, 6m30s, $—, 3068796 input / 47106 output / 2716984 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Parse `injuries` (`status`, `date`) into `espn.RosterPlayer` in `internal/ingest/espn/roster.go` and add unit tests against real fixtures in `internal/ingest/espn/roster_test.go`.

- Added `InjuryStatus` and `InjuryDate` fields to `espn.RosterPlayer` in `internal/ingest/espn/roster.go`.
- Decoded athlete `injuries` (`status`, `date`) in `Client.Roster` and selected the highest-priority non-empty status (`Out` > `Doubtful` > `Questionable` > `Injured Reserve` > first non-empty) via `selectRosterInjury`.
- Added `TestRosterInjuriesFromFixtures` against `roster.json`, `roster_bengals.json`, and `roster_falcons.json`, plus `TestRosterInjuryPriority` in `internal/ingest/espn/roster_test.go`.

### Task 2: Add migration `internal/db/migrations/0014_roster_injuries.sql`, extend `db.RosterPlayer` queries in `internal/db/players.go`, and wire `syncRoster` in `internal/refresh/research.go`, with tests in `internal/db/players_test.go`, `internal/db/migrate_test.go`, and `internal/refresh/research_test.go`.

- Added migration `internal/db/migrations/0014_roster_injuries.sql` adding `injury_status` and `injury_date` (`TEXT NOT NULL DEFAULT ''`) to `roster_players`.
- Extended `db.RosterPlayer`, `rosterColumns`, `UpsertRosterPlayers`, and `scanRosterPlayer` in `internal/db/players.go` to persist and read `InjuryStatus` and `InjuryDate`.
- Wired `syncRoster` in `internal/refresh/research.go` to pass `p.InjuryStatus` and `p.InjuryDate` through to `db.RosterPlayer`.
- Added unit and migration tests in `internal/db/migrate_test.go` (plus schema version updates in `internal/db/migrate_test.go` and `internal/db/migrate_internal_test.go`), `internal/db/players_test.go`, and `internal/refresh/research_test.go` covering persistence and clearing on healthy refresh.

### Task 3: Docs: update `docs/reference/schema.md` and `docs/reference/data-sources.md` for `roster_players.injury_status` / `injury_date` and ESPN roster injury ingestion.

- Updated `docs/reference/schema.md` for `0014_roster_injuries.sql` (version 14), adding `injury_status` and `injury_date` to the `roster_players` DDL, summary, migrations list, tables summary, and conventions.
- Updated `docs/reference/data-sources.md` under ESPN `Roster` to document `InjuryStatus`, `InjuryDate`, status-priority selection (`Out` > `Doubtful` > `Questionable` > `Injured Reserve` > first non-empty), and clearing on empty `injuries`.
- Shortened both docs' front-matter `summary` fields to pass `good-vibes docs lint` and ran `good-vibes docs index`.

### Phase 2: Merge summary + roster injuries and wire into analyzer, board, and research

Tasks:

- [x] Add `espn.IsKeyInjuryPosition` and `espn.MergeTeamInjuries` in `internal/ingest/espn/summary.go` with unit tests in `internal/ingest/espn/summary_test.go`.
- [x] Add `analyzer.Injury`, `Game.Injuries`, `PlayerStats.Position`, and `analyzer.MergeGameInjuries`, and populate them in `analyzer.LoadSlipSignals` (`internal/analyzer/signals.go`) from `games.injuries_json` + `roster_players`, with tests in `internal/analyzer/signals_test.go`.
- [x] Reuse `analyzer.MergeGameInjuries` in `web.slipHandlers.boardSignalsForGame` (`internal/web/pick.go`) and `web.researchHandlers.loadGameResearch` (`internal/web/research.go`) so both the pick board's `Role bump` badge and Game Research see truncated roster injuries, with tests in `internal/web/pick_test.go` and `internal/web/research_test.go`.

Phase totals: 3 checkpoints, 6m41s, $—, 1919748 input / 20868 output / 1694364 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Add deterministic teammate-injury adjustments to `ScoreLeg`

Tasks:

- [x] Update `scoreFormAndUsage` in `internal/analyzer/grade.go` to soften `L5 median trap` from `-15` to `-5` and award `teammate_out_bump` (`+5`) when a same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out` on an Over/Anytime TD prop, and update `GradeRubric()` in `internal/analyzer/docs.go`, with unit tests in `internal/analyzer/analyzer_test.go` and `internal/analyzer/nflverse_grade_test.go`.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to document the roster+summary injury merge and the teammate-out `ScoreLeg` adjustments.

Phase totals: 3 checkpoints, 7m40s, $—, 3990834 input / 57408 output / 3578398 cache read / 0 cache write tokens

Checkpoints: task-7, task-8, final Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Update `scoreFormAndUsage` in `internal/analyzer/grade.go` to soften `L5 median trap` from `-15` to `-5` and award `teammate_out_bump` (`+5`) when a same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out` on an Over/Anytime TD prop, and update `GradeRubric()` in `internal/analyzer/docs.go`, with unit tests in `internal/analyzer/analyzer_test.go` and `internal/analyzer/nflverse_grade_test.go`.

- Updated `scoreFormAndUsage` in `internal/analyzer/grade.go` to accept `game Game`, soften `L5 median trap` from `-15` to `-5` when an eligible same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out`, and award `teammate_out_bump` (`+5`) on qualifying Over and Anytime TD Yes props.
- Added `outSamePositionTeammate`, `isSkillVolumePosition`, and `isEligibleTeammateOutLeg` helpers in `internal/analyzer/grade.go`, ignoring `Questionable`, `Doubtful`, `Injured Reserve`, non-skill positions, self-injuries, and Under props.
- Updated `GradeRubric()` in `internal/analyzer/docs.go` to document the softened `L5 median trap` (`-15 (-5 w/ teammate Out)`) and `teammate_out_bump` (`+5`).
- Added unit tests in `internal/analyzer/analyzer_test.go` (`TestScoreLegTeammateOut`) and `internal/analyzer/nflverse_grade_test.go` (`TestScoreLeg` and `TestRuleDocsCoverageAndGradeRubric`).

### Task 8: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to document the roster+summary injury merge and the teammate-out `ScoreLeg` adjustments.

- Updated `docs/concepts/analyzer.md` to document the `LoadSlipSignals` roster+summary injury merge (`MergeGameInjuries` / `espn.MergeTeamInjuries`), `PlayerStats.Position`, `teammate_out_bump` (`+5`), and the softened `median_trap` (`-15`, or `-5` with a same-position `Out` teammate).
- Trimmed excess inline code spans in `docs/concepts/analyzer.md` so `good-vibes docs lint` passes with 0 errors on changed docs.
- Updated `docs/reference/analyzer-rules.md` to document the roster+summary injury merge under `How rules see a slip`, update `median_trap` (`-15` / `-5` w/ teammate Out), and add `teammate_out_bump` (`+5`) to the `ScoreLeg` rubric table.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
