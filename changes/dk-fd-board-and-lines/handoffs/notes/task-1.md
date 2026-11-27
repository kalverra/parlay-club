## Done

- Added `BookQuote`, `BuildOptions`, `Row.DK`, `Row.FD`, and `Row.LinesDiffer` to `internal/board/board.go`.
- Added `BuildDualBook` and `BuildWithOptions` implementing `draftkings` ∩ `fanduel` filtering, selecting `homeBook` (or `fanduel` when unset) without averaging differing lines into a median.
- Updated `internal/eval/market.go` to call `board.BuildWithOptions` with `BuildOptions{DualBook: false}` so market-wide consensus evaluation remains explicitly all-book.
- Updated `board.LineKey` (and `lineKey` via it) so `TOTAL` keys on market alone and `TEAM_TOTAL` keys on `TEAM_TOTAL|<team>`, keeping split Over/Under rows paired in one `LineGroup`.
- Updated `TestLineKeyCanonicalPlayerPropsAndGameLines` and added dual-book intersection, split-line non-averaging, and split `TOTAL`/`TEAM_TOTAL` pairing unit tests in `internal/board/board_test.go`.

## Deviations

- Kept `board.Build` in all-book mode (`BuildOptions{DualBook: false}`) for now and introduced `board.BuildDualBook` / `board.BuildWithOptions` so existing `internal/web` and `internal/refresh` callers and single-book tests continue to pass until Phase 2 wires user-facing surfaces to dual-book mode.

## Open issues

None.

## Next-phase notes

- Phase 2 should switch user-facing callers (`internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, `internal/web/slips.go`) to `board.BuildDualBook` and update single-book test fixtures in `internal/web/*_test.go` (and `internal/refresh/research_test.go` if needed) to provide both `draftkings` and `fanduel` quotes where pickable rows are expected.
