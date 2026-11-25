## Done

- Added `summaryLiveAge = 2 * time.Minute` and updated `summaryStale` / `summaryAgeFor` in `internal/refresh/rules.go` and `internal/refresh/refresh.go` so `espn:summary:{gameID}` refreshes every 2 minutes only for `LIVE` games with active slip legs, keeping the 15-minute game-day and 1-hour idle windows otherwise.
- Added an optional `summaryClient` interface in `internal/refresh/refresh.go` and updated `syncSummary` in `internal/refresh/research.go` to call `Summary` when available (falling back to `SummaryInjuries` for narrow test doubles), persist marshaled `boxscore_json` alongside `injuries_json` for `LIVE` games via `SetGameSummary`, and publish `week:{weekID}` and `slip:{id}` on changes.
- Updated `slateGame` in `internal/refresh/sources.go` to map `Period` and `Clock` from `espn.Game` so `syncScoreboard` persists live quarter/clock updates and publishes on changes.
- Added staleness, persistence, and publish tests in `internal/refresh/rules_test.go`, `internal/refresh/research_test.go`, and `internal/refresh/publish_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`games.period`, `games.clock`, and `games.boxscore_json` are now kept fresh during `LIVE` games and trigger `week:{weekID}` / `slip:{id}` SSE notifications on changes, ready for `internal/grader` helper exports and `internal/progress` evaluation in Phase 2.

## Commit

Refresh live ESPN summaries and clock every two minutes

Live parlay progress needs up-to-date quarter, clock, and player box scores without burning paid odds credits or adding scheduled jobs. Scoping the 2-minute summary cadence to live games with active slip legs keeps free ESPN traffic bounded while pushing updates over SSE whenever game state or stats change.
