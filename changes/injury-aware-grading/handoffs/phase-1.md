# Phase 1: Phase 1: Ingest and persist ESPN roster injuries

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`RosterPlayer.InjuryStatus` and `RosterPlayer.InjuryDate` are empty strings when an athlete has no non-empty injury entries, ready for `0014_roster_injuries.sql` and `UpsertRosterPlayers` to persist and clear statuses on refresh.

### Task 2

`db.RosterPlayer.InjuryStatus` and `db.RosterPlayer.InjuryDate` are populated by `syncRoster` and cleared back to `""` when ESPN's roster entry has no injuries, ready for `espn.MergeTeamInjuries` and `analyzer.MergeGameInjuries` in Phase 2.

### Task 3

Phase 1 is complete; Phase 2 can build `espn.MergeTeamInjuries`, `analyzer.MergeGameInjuries`, and the `web.boardSignalsForGame` / `web.loadGameResearch` integrations on top of the documented `roster_players` injury columns.

## Files touched

- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0014_roster_injuries.sql`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/ingest/espn/roster.go`
- `internal/ingest/espn/roster_test.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
