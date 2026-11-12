# Task 3 Notes

## Done

- Updated `docs/reference/data-sources.md` (`last_verified: 2026-11-12`) to document `team.logo` in ESPN Scoreboard, `LogoURL` and `HeadshotURL` in ESPN Summary injuries, `HeadshotURL` (`headshot.href`) in ESPN Roster, and `https://`-only URL normalization.
- Updated `docs/reference/schema.md` (`last_verified: 2026-11-12`) to document `0011_logos_portraits.sql`, the `games.home_logo` / `games.away_logo` and `roster_players.headshot_url` DDL columns, and their conventions.

## Deviations

- Documented migration `0011_logos_portraits.sql` (instead of `0010_logos_portraits.sql` in the task prompt) to match the actual migration file created in Task 2.

## Open issues

None.

## Next-phase notes

- Phase 1 (ingest, DB schema/migrations, `refresh`, `board`, and reference docs) is complete; Phase 2 can wire `internal/web` view models, shared media templates, and `app.js` fallback handling.
