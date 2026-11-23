## Done

- Added migration `internal/db/migrations/0014_roster_injuries.sql` adding `injury_status` and `injury_date` (`TEXT NOT NULL DEFAULT ''`) to `roster_players`.
- Extended `db.RosterPlayer`, `rosterColumns`, `UpsertRosterPlayers`, and `scanRosterPlayer` in `internal/db/players.go` to persist and read `InjuryStatus` and `InjuryDate`.
- Wired `syncRoster` in `internal/refresh/research.go` to pass `p.InjuryStatus` and `p.InjuryDate` through to `db.RosterPlayer`.
- Added unit and migration tests in `internal/db/migrate_test.go` (plus schema version updates in `internal/db/migrate_test.go` and `internal/db/migrate_internal_test.go`), `internal/db/players_test.go`, and `internal/refresh/research_test.go` covering persistence and clearing on healthy refresh.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`db.RosterPlayer.InjuryStatus` and `db.RosterPlayer.InjuryDate` are populated by `syncRoster` and cleared back to `""` when ESPN's roster entry has no injuries, ready for `espn.MergeTeamInjuries` and `analyzer.MergeGameInjuries` in Phase 2.
