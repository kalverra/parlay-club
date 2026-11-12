# Task 1 Notes

## Done

- Added `normalizeHTTPSURL` in `internal/ingest/espn/client.go` to trim whitespace and accept only `https://` URLs with a non-empty host.
- Extended `espn.Team` (`Logo`), `espn.RosterPlayer` (`HeadshotURL`), `espn.TeamInjuries` (`LogoURL`), and `espn.Injury` (`HeadshotURL`) and wired parsing from `team.logo`, `headshot.href`, and injury `team.logo`/`athlete.headshot.href`.
- Updated `scoreboard_test.go`, `roster_test.go`, and `summary_test.go` to verify real fixtures (`scoreboard_week.json`, `scoreboard_live.json`, `roster.json`, `roster_falcons.json`, `summary.json`) as well as missing and invalid/non-HTTPS URLs.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `espn.Team.Logo`, `espn.RosterPlayer.HeadshotURL`, `espn.TeamInjuries.LogoURL`, and `espn.Injury.HeadshotURL` are ready to be persisted in `internal/db` and mapped in `internal/refresh`.
- `espn.TeamInjuries` (`logo_url`) and `espn.Injury` (`headshot_url`) are parsed directly from `games.injuries_json`, so no schema change is needed for injury media.
