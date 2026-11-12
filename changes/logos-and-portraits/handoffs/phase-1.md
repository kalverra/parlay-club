# Phase 1: Phase 1: Ingest, store, and wire logo and portrait metadata

## Done

### Task 1: Parse `team.logo` in `espn.Scoreboard`, `headshot.href` in `espn.Roster`, and injury `team.logo`/`athlete.headshot.href` in `espn.ParseInjuries` (normalizing to `https://` only), with fixture tests covering present and missing URLs.

- Added `normalizeHTTPSURL` in `internal/ingest/espn/client.go` to trim whitespace and accept only `https://` URLs with a non-empty host.
- Extended `espn.Team` (`Logo`), `espn.RosterPlayer` (`HeadshotURL`), `espn.TeamInjuries` (`LogoURL`), and `espn.Injury` (`HeadshotURL`) and wired parsing from `team.logo`, `headshot.href`, and injury `team.logo`/`athlete.headshot.href`.
- Updated `scoreboard_test.go`, `roster_test.go`, and `summary_test.go` to verify real fixtures (`scoreboard_week.json`, `scoreboard_live.json`, `roster.json`, `roster_falcons.json`, `summary.json`) as well as missing and invalid/non-HTTPS URLs.

### Task 2: Add `0010_logos_portraits.sql` (`games.home_logo`, `games.away_logo`, `roster_players.headshot_url`), update `db.Game`, `db.RosterPlayer`, `UpsertGame` idempotency, `RosterPlayersByIDs`/`TeamLogos`, `refresh` syncs, and `board` (`GameCard`, `Row`, `PlayerGroup`) with tests.

- Added `internal/db/migrations/0011_logos_portraits.sql` adding `games.home_logo`, `games.away_logo`, and `roster_players.headshot_url` (with `NOT NULL DEFAULT ''`), and updated schema version assertions in `migrate_test.go` and `migrate_internal_test.go`.
- Updated `db.Game` (`HomeLogo`, `AwayLogo`), `gameColumns`, `scanGame`, and `UpsertGame` (including the `WHERE ... IS NOT ...` dirty check), and added `Store.TeamLogos` in `internal/db/games.go`.
- Updated `db.RosterPlayer` (`HeadshotURL`), `rosterColumns`, `scanRosterPlayer`, and `UpsertRosterPlayers`, and added `Store.RosterPlayersByIDs` in `internal/db/players.go`.
- Wired `HomeLogo`/`AwayLogo` in `internal/refresh/sources.go` (`slateGame`) and `HeadshotURL` in `internal/refresh/research.go` (`syncRoster`).
- Added `AwayLogo`/`HomeLogo` to `board.GameCard` and `HeadshotURL` to `board.Row` and `board.PlayerGroup` via `matchPlayer` in `internal/board/board.go`, with tests across `internal/db`, `internal/refresh`, and `internal/board`.

### Task 3: Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for the new logo and headshot fields and migration `0010`.

- Updated `docs/reference/data-sources.md` (`last_verified: 2026-11-12`) to document `team.logo` in ESPN Scoreboard, `LogoURL` and `HeadshotURL` in ESPN Summary injuries, `HeadshotURL` (`headshot.href`) in ESPN Roster, and `https://`-only URL normalization.
- Updated `docs/reference/schema.md` (`last_verified: 2026-11-12`) to document `0011_logos_portraits.sql`, the `games.home_logo` / `games.away_logo` and `roster_players.headshot_url` DDL columns, and their conventions.

## Deviations

### Task 2

- Named the migration `0011_logos_portraits.sql` instead of `0010_logos_portraits.sql` because `0010_bug_reports.sql` already exists at version 10 and `migrationList` rejects duplicate version numbers.

### Task 3

- Documented migration `0011_logos_portraits.sql` (instead of `0010_logos_portraits.sql` in the task prompt) to match the actual migration file created in Task 2.

## Open issues

None.

## Next-phase notes

### Task 1

- `espn.Team.Logo`, `espn.RosterPlayer.HeadshotURL`, `espn.TeamInjuries.LogoURL`, and `espn.Injury.HeadshotURL` are ready to be persisted in `internal/db` and mapped in `internal/refresh`.
- `espn.TeamInjuries` (`logo_url`) and `espn.Injury` (`headshot_url`) are parsed directly from `games.injuries_json`, so no schema change is needed for injury media.

### Task 2

- `db.Game` (`HomeLogo`, `AwayLogo`), `Store.TeamLogos`, `Store.RosterPlayersByIDs`, `board.GameCard`, `board.Row`, and `board.PlayerGroup` now expose all logo and portrait URLs needed by `internal/web` view models and templates.

### Task 3

- Phase 1 (ingest, DB schema/migrations, `refresh`, `board`, and reference docs) is complete; Phase 2 can wire `internal/web` view models, shared media templates, and `app.js` fallback handling.

## Files touched

- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/db/games.go`
- `internal/db/games_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0011_logos_portraits.sql`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/ingest/espn/client.go`
- `internal/ingest/espn/roster.go`
- `internal/ingest/espn/roster_test.go`
- `internal/ingest/espn/scoreboard.go`
- `internal/ingest/espn/scoreboard_test.go`
- `internal/ingest/espn/summary.go`
- `internal/ingest/espn/summary_test.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
