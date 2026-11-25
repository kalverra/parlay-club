## Done

- Updated `docs/reference/data-sources.md` to document ESPN scoreboard `period` and `clock` fields (populated only while `LIVE`) and `Client.Summary` returning both `InjuriesJSON` and `BoxScore` from one `/summary` request.
- Updated `docs/reference/schema.md` for migration `0018_live_game_state.sql` (`period`, `clock`, and `boxscore_json` on `games`), bumping the fresh database version to 18 and noting `SetGameSummary` preservation semantics.
- Updated `docs/concepts/data-refresh.md` for the 2-minute `espn:summary:{gameID}` live cadence on `LIVE` games with active slip legs, `boxscore_json` persistence, and `week:{weekID}` / `slip:{id}` publishing on summary changes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 1 is complete; Phase 2 can export `FindPlayer` and `PlayerMarketStat` from `internal/grader` and implement `internal/progress` using the documented `games.period`, `games.clock`, and `games.boxscore_json` fields.

## Commit

Document live clock, boxscore_json, and summary cadence

Keep the reference and concept docs aligned with the Phase 1 ESPN ingest, SQLite schema, and 2-minute live summary refresh changes.
