## Done

- Added `Period *int` and `Clock *string` to `espn.Game` in `internal/ingest/espn/scoreboard.go`, populated only for `Live` games (`nil` for `Scheduled` and `Final`) with break normalization for `"Halftime"` and `"End of ..."` strings.
- Added `GameSummary` and `Client.Summary(ctx, eventID)` in `internal/ingest/espn/summary.go` to parse both `InjuriesJSON` and `BoxScore` from a single `GET /summary?event={id}` request while keeping `SummaryInjuries` and `BoxScore` intact.
- Added unit tests in `internal/ingest/espn/scoreboard_test.go` and `internal/ingest/espn/summary_test.go` covering pre-game, live regulation, halftime, end-of-period break, overtime, and final payloads.

## Deviations

`internal/ingest/espn/boxscore.go` required no edits because `rawSummaryBoxScore` and `parseBoxScore` already resided in `package espn` and were reused directly by `Client.Summary`.

## Open issues

None.

## Next-phase notes

`espn.Game.Period` and `espn.Game.Clock` are ready for `internal/db` migration `0018_live_game_state.sql`, `UpsertGame`, and `SetGameSummary`, and `Client.Summary` is ready for `internal/refresh/research.go`.
