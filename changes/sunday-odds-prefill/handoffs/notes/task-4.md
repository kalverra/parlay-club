# Task 4

## Done

- Added `SyncPickLines`, `SyncPickGame`, and `ForcePickRefresh` to `refresh.Service` in `internal/refresh/refresh.go` (using `requestTimeout` and single-flight `refreshSync`, ignoring `odds.ErrNoAPIKey` when odds providers are unconfigured).
- Implemented `oddsPickLinesStale` (15m on game days before the day's last kickoff or within 2h of next kickoff; 2h midweek), tightened `oddsPropsStale` (30m on game days or within 6h of kickoff; 2h earlier in the week), and `pickCooldownStale` (2m manual refresh cooldown) in `internal/refresh/rules.go`.
- Updated `TestOddsPropsStaleRules` in `internal/refresh/rules_test.go` and `TestOpeningSameGameTwiceWithinCacheWindowSpendsCreditsOnce` in `internal/refresh/research_test.go` for the 30m game-day props window, and added `TestOddsPickLinesStaleRules` and `TestPickCooldownStaleRules`.
- Added unit and single-flight tests in `internal/refresh/pick_test.go` covering `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, cooldown enforcement, and concurrent caller coalescing.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `web.Refresher` in `internal/web/routes.go` can now be extended with `SyncPickLines(ctx context.Context, weekID int64) error`, `SyncPickGame(ctx context.Context, weekID int64, gameID string) error`, and `ForcePickRefresh(ctx context.Context, weekID int64, gameID string) error`, which `*refresh.Service` already satisfies.
