# Phase 1: Phase 1: Add market- and bookmaker-scoped Odds API queries

## Done

### Task 1: Add `APIMarketKey`, `LinesForMarkets`, and `EventPropsForMarkets` in `internal/ingest/odds/lines.go` and `internal/ingest/odds/props.go` (supporting `markets` and `bookmakers` query params), with unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go`.

- Added `LinesForMarkets(ctx, markets, bookmakers)` in `internal/ingest/odds/lines.go` and refactored `Lines(ctx)` to delegate to it, using shared `buildOddsQuery` and `cleanCSVTerms` helpers to support `bookmakers` (falling back to `regions=us` when empty) and deduplicated `markets` with `includeLinks=true` and `includeSids=true`.
- Added `APIMarketKey(marketType)` and `EventPropsForMarkets(ctx, eventID, markets, bookmakers)` in `internal/ingest/odds/props.go`, and refactored `EventProps(ctx, eventID)` to delegate to `EventPropsForMarkets`.
- Added unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go` covering `APIMarketKey` for all 12 market types, scoped `markets` and `bookmakers` query params, `regions=us` fallback, and empty-argument validation.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `internal/refresh` can now import `odds.APIMarketKey`, `LinesForMarkets`, and `EventPropsForMarkets` without duplicating Odds API market key strings.

## Files touched

- `internal/ingest/odds/lines.go`
- `internal/ingest/odds/lines_test.go`
- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 800ms, lint 7.8s, test 10.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
