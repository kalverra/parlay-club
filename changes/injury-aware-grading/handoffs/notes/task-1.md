## Done

- Added `InjuryStatus` and `InjuryDate` fields to `espn.RosterPlayer` in `internal/ingest/espn/roster.go`.
- Decoded athlete `injuries` (`status`, `date`) in `Client.Roster` and selected the highest-priority non-empty status (`Out` > `Doubtful` > `Questionable` > `Injured Reserve` > first non-empty) via `selectRosterInjury`.
- Added `TestRosterInjuriesFromFixtures` against `roster.json`, `roster_bengals.json`, and `roster_falcons.json`, plus `TestRosterInjuryPriority` in `internal/ingest/espn/roster_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`RosterPlayer.InjuryStatus` and `RosterPlayer.InjuryDate` are empty strings when an athlete has no non-empty injury entries, ready for `0014_roster_injuries.sql` and `UpsertRosterPlayers` to persist and clear statuses on refresh.
