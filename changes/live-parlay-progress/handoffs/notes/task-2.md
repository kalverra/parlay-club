## Done

- Added `internal/db/migrations/0018_live_game_state.sql` adding nullable `period`, `clock`, and `boxscore_json` columns to `games`.
- Extended `db.Game`, `gameColumns`, `scanGame`, and `UpsertGame` in `internal/db/games.go` to store and idempotently update `Period` and `Clock` while preserving `BoxScoreJSON` and `InjuriesJSON`.
- Added `Store.SetGameSummary(ctx, gameID, injuriesJSON, boxscoreJSON)` with NULL-safe change detection (preserving any existing `boxscore_json` when `boxscoreJSON` is `nil`) and `Store.LiveGameIDsWithSlipLegs(ctx, weekID)` returning `LIVE` game IDs in the week referenced by at least one slip leg.
- Added migration and store tests in `internal/db/migrate_test.go`, `internal/db/migrate_internal_test.go`, and `internal/db/games_test.go` covering schema version 18, `Period`/`Clock` upsert idempotency, `SetGameSummary` change detection, and `LiveGameIDsWithSlipLegs` filtering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`SetGameSummary(ctx, gameID, injuriesJSON, boxscoreJSON *string)` takes a `*string` for `boxscoreJSON` and preserves any existing `boxscore_json` when `nil` is passed, so `syncSummary` in `internal/refresh/research.go` can pass `nil` for `SCHEDULED` games (or when falling back to `SummaryInjuries`) and a marshaled `BoxScore` JSON pointer for `LIVE` games.

## Commit

Add live game state columns and store methods

SQLite needs to persist live quarter, game clock, and cached box scores alongside existing game state so live parlay progress can be computed on read and SSE updates only fire on real changes. Storing `boxscore_json` on `games` and querying active live games with slip legs lets the refresh loop keep 2-minute summary polling scoped to games that matter.
