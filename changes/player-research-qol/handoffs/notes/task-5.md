# Task 5

## Done

- Added `Store.LatestPropForPlayerMarket(ctx, athleteID, playerName, team, market)` in `internal/db/odds.go` to return the newest prop odds row with a non-nil line filtered by `market_type` when `market` is non-empty.
- Delegated `Store.LatestPropForPlayer` to `LatestPropForPlayerMarket(..., "")` so candidate resolution and query logic stay in one place.
- Added `TestLatestPropForPlayerMarket` in `internal/db/players_test.go` covering empty-market fallback, market-specific filtering without cross-market bleed, newest-row selection, `player_matches` lookup, nil-line exclusion, and team filtering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 3 (`internal/web/research.go`) can switch to `Store.LatestPropForPlayerMarket(ctx, athleteID, player.FullName, player.Team, activeMarket)` and drop `&line=...` from `buildMarketOptions` tab links so each market tab resolves its own O/U line.

## Commit

Add market-specific prop lookup to odds store

Player research previously looked up the newest prop line across all markets,
which caused lines from one market to bleed onto other market tabs. Filtering by
market type in the store lets each market tab resolve its own current O/U line.
