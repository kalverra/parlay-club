## Done

- Added `LinesForMarkets(ctx, markets, bookmakers)` in `internal/ingest/odds/lines.go` and refactored `Lines(ctx)` to delegate to it, using shared `buildOddsQuery` and `cleanCSVTerms` helpers to support `bookmakers` (falling back to `regions=us` when empty) and deduplicated `markets` with `includeLinks=true` and `includeSids=true`.
- Added `APIMarketKey(marketType)` and `EventPropsForMarkets(ctx, eventID, markets, bookmakers)` in `internal/ingest/odds/props.go`, and refactored `EventProps(ctx, eventID)` to delegate to `EventPropsForMarkets`.
- Added unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go` covering `APIMarketKey` for all 12 market types, scoped `markets` and `bookmakers` query params, `regions=us` fallback, and empty-argument validation.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/refresh` can now import `odds.APIMarketKey`, `LinesForMarkets`, and `EventPropsForMarkets` without duplicating Odds API market key strings.
