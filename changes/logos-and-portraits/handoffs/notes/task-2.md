# Task 2 Notes

## Done

- Added `internal/db/migrations/0011_logos_portraits.sql` adding `games.home_logo`, `games.away_logo`, and `roster_players.headshot_url` (with `NOT NULL DEFAULT ''`), and updated schema version assertions in `migrate_test.go` and `migrate_internal_test.go`.
- Updated `db.Game` (`HomeLogo`, `AwayLogo`), `gameColumns`, `scanGame`, and `UpsertGame` (including the `WHERE ... IS NOT ...` dirty check), and added `Store.TeamLogos` in `internal/db/games.go`.
- Updated `db.RosterPlayer` (`HeadshotURL`), `rosterColumns`, `scanRosterPlayer`, and `UpsertRosterPlayers`, and added `Store.RosterPlayersByIDs` in `internal/db/players.go`.
- Wired `HomeLogo`/`AwayLogo` in `internal/refresh/sources.go` (`slateGame`) and `HeadshotURL` in `internal/refresh/research.go` (`syncRoster`).
- Added `AwayLogo`/`HomeLogo` to `board.GameCard` and `HeadshotURL` to `board.Row` and `board.PlayerGroup` via `matchPlayer` in `internal/board/board.go`, with tests across `internal/db`, `internal/refresh`, and `internal/board`.

## Deviations

- Named the migration `0011_logos_portraits.sql` instead of `0010_logos_portraits.sql` because `0010_bug_reports.sql` already exists at version 10 and `migrationList` rejects duplicate version numbers.

## Open issues

None.

## Next-phase notes

- `db.Game` (`HomeLogo`, `AwayLogo`), `Store.TeamLogos`, `Store.RosterPlayersByIDs`, `board.GameCard`, `board.Row`, and `board.PlayerGroup` now expose all logo and portrait URLs needed by `internal/web` view models and templates.
