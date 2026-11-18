## Done

- Added `MarketLabel`, `FormatSelection`, and `BuildLegSelection` (with shared `titleSide`, `strictTitleSide`, `signedLine`, and `formatLine` helpers) in `internal/db/odds.go`.
- Replaced `rowText`, `marketLabel`, and `titleSide` in `internal/board/board.go` with calls to `db.FormatSelection` and `db.MarketLabel`.
- Replaced `buildSelection` and `marketLabels` in `internal/web/slips.go` with `db.BuildLegSelection`.
- Replaced `marketLabel` in `internal/web/research.go` with `db.MarketLabel` (falling back to the raw market string when empty).
- Added unit tests `TestMarketLabel`, `TestFormatSelection`, and `TestBuildLegSelection` in `internal/db/odds_test.go`.

## Deviations

- `internal/web/research.go` previously used long market names (`"Passing Yards"`, `"Anytime Touchdown"`, `"Kicker Points"`) in its local `marketLabel`, whereas `db.MarketLabel` uses the shared short labels (`"Pass Yds"`, `"Anytime TD"`, `"Kicking Pts"`). Replaced `research.marketLabel` with `db.MarketLabel` per the design.

## Open issues

None.

## Next-phase notes

- Phase 3 market label and selection formatting centralization is in place for Task 5 (verification/docs).
