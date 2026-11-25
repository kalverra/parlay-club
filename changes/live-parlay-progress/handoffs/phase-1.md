# Phase 1: Phase 1: Capture live game clock and box scores from ESPN

## Done

### Task 1: Extend `internal/ingest/espn` (`scoreboard.go`, `summary.go`, `boxscore.go`) to parse live `period`, `displayClock` / break detail, and combined `Summary` (`injuries` + `boxscore`) from one `/summary` request, with unit tests for pre-game, live regulation, halftime, overtime, and final payloads.

- Added `Period *int` and `Clock *string` to `espn.Game` in `internal/ingest/espn/scoreboard.go`, populated only for `Live` games (`nil` for `Scheduled` and `Final`) with break normalization for `"Halftime"` and `"End of ..."` strings.
- Added `GameSummary` and `Client.Summary(ctx, eventID)` in `internal/ingest/espn/summary.go` to parse both `InjuriesJSON` and `BoxScore` from a single `GET /summary?event={id}` request while keeping `SummaryInjuries` and `BoxScore` intact.
- Added unit tests in `internal/ingest/espn/scoreboard_test.go` and `internal/ingest/espn/summary_test.go` covering pre-game, live regulation, halftime, end-of-period break, overtime, and final payloads.

### Task 2: Add `internal/db/migrations/0018_live_game_state.sql` (`period`, `clock`, `boxscore_json` on `games`) and update `internal/db/games.go` (`Game`, `UpsertGame`, `SetGameSummary`, `LiveGameIDsWithSlipLegs`) with migration and store tests verifying idempotent change detection.

- Added `internal/db/migrations/0018_live_game_state.sql` adding nullable `period`, `clock`, and `boxscore_json` columns to `games`.
- Extended `db.Game`, `gameColumns`, `scanGame`, and `UpsertGame` in `internal/db/games.go` to store and idempotently update `Period` and `Clock` while preserving `BoxScoreJSON` and `InjuriesJSON`.
- Added `Store.SetGameSummary(ctx, gameID, injuriesJSON, boxscoreJSON)` with NULL-safe change detection (preserving any existing `boxscore_json` when `boxscoreJSON` is `nil`) and `Store.LiveGameIDsWithSlipLegs(ctx, weekID)` returning `LIVE` game IDs in the week referenced by at least one slip leg.
- Added migration and store tests in `internal/db/migrate_test.go`, `internal/db/migrate_internal_test.go`, and `internal/db/games_test.go` covering schema version 18, `Period`/`Clock` upsert idempotency, `SetGameSummary` change detection, and `LiveGameIDsWithSlipLegs` filtering.

### Task 3: Update `internal/refresh` (`rules.go`, `research.go`, `refresh.go`, `sources.go`) so `espn:summary:{gameID}` refreshes every 2 minutes for `LIVE` games with active slip legs, persists live `boxscore_json` alongside `injuries_json`, and publishes `week:{weekID}` / `slip:{id}` on changes, with staleness and publish tests.

- Added `summaryLiveAge = 2 * time.Minute` and updated `summaryStale` / `summaryAgeFor` in `internal/refresh/rules.go` and `internal/refresh/refresh.go` so `espn:summary:{gameID}` refreshes every 2 minutes only for `LIVE` games with active slip legs, keeping the 15-minute game-day and 1-hour idle windows otherwise.
- Added an optional `summaryClient` interface in `internal/refresh/refresh.go` and updated `syncSummary` in `internal/refresh/research.go` to call `Summary` when available (falling back to `SummaryInjuries` for narrow test doubles), persist marshaled `boxscore_json` alongside `injuries_json` for `LIVE` games via `SetGameSummary`, and publish `week:{weekID}` and `slip:{id}` on changes.
- Updated `slateGame` in `internal/refresh/sources.go` to map `Period` and `Clock` from `espn.Game` so `syncScoreboard` persists live quarter/clock updates and publishes on changes.
- Added staleness, persistence, and publish tests in `internal/refresh/rules_test.go`, `internal/refresh/research_test.go`, and `internal/refresh/publish_test.go`.

### Task 4: Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, and `docs/concepts/data-refresh.md` for live clock/period fields, `boxscore_json`, and live summary cadence.

- Updated `docs/reference/data-sources.md` to document ESPN scoreboard `period` and `clock` fields (populated only while `LIVE`) and `Client.Summary` returning both `InjuriesJSON` and `BoxScore` from one `/summary` request.
- Updated `docs/reference/schema.md` for migration `0018_live_game_state.sql` (`period`, `clock`, and `boxscore_json` on `games`), bumping the fresh database version to 18 and noting `SetGameSummary` preservation semantics.
- Updated `docs/concepts/data-refresh.md` for the 2-minute `espn:summary:{gameID}` live cadence on `LIVE` games with active slip legs, `boxscore_json` persistence, and `week:{weekID}` / `slip:{id}` publishing on summary changes.

## Deviations

### Task 1

`internal/ingest/espn/boxscore.go` required no edits because `rawSummaryBoxScore` and `parseBoxScore` already resided in `package espn` and were reused directly by `Client.Summary`.

## Open issues

None.

## Next-phase notes

### Task 1

`espn.Game.Period` and `espn.Game.Clock` are ready for `internal/db` migration `0018_live_game_state.sql`, `UpsertGame`, and `SetGameSummary`, and `Client.Summary` is ready for `internal/refresh/research.go`.

### Task 2

`SetGameSummary(ctx, gameID, injuriesJSON, boxscoreJSON *string)` takes a `*string` for `boxscoreJSON` and preserves any existing `boxscore_json` when `nil` is passed, so `syncSummary` in `internal/refresh/research.go` can pass `nil` for `SCHEDULED` games (or when falling back to `SummaryInjuries`) and a marshaled `BoxScore` JSON pointer for `LIVE` games.

### Task 3

`games.period`, `games.clock`, and `games.boxscore_json` are now kept fresh during `LIVE` games and trigger `week:{weekID}` / `slip:{id}` SSE notifications on changes, ready for `internal/grader` helper exports and `internal/progress` evaluation in Phase 2.

### Task 4

Phase 1 is complete; Phase 2 can export `FindPlayer` and `PlayerMarketStat` from `internal/grader` and implement `internal/progress` using the documented `games.period`, `games.clock`, and `games.boxscore_json` fields.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/games.go`
- `internal/db/games_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0018_live_game_state.sql`
- `internal/ingest/espn/scoreboard.go`
- `internal/ingest/espn/scoreboard_test.go`
- `internal/ingest/espn/summary.go`
- `internal/ingest/espn/summary_test.go`
- `internal/refresh/publish_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/refresh/sources.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3, task-4
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
