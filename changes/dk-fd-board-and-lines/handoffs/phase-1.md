# Phase 1: Phase 1: Dual-book intersection and split-line model in `internal/board`

## Done

### Task 1: Add `BookQuote`, `Row.DK`, `Row.FD`, `Row.LinesDiffer`, and dual-book filtering (`draftkings` ∩ `fanduel`) in `internal/board/board.go` (never averaging differing lines into a median), keep an explicit all-book mode for `internal/eval/market.go`, and update `lineKey` pairing so split `TOTAL`/`TEAM_TOTAL` rows stay paired; add unit tests in `internal/board/board_test.go`.

- Added `BookQuote`, `BuildOptions`, `Row.DK`, `Row.FD`, and `Row.LinesDiffer` to `internal/board/board.go`.
- Added `BuildDualBook` and `BuildWithOptions` implementing `draftkings` ∩ `fanduel` filtering, selecting `homeBook` (or `fanduel` when unset) without averaging differing lines into a median.
- Updated `internal/eval/market.go` to call `board.BuildWithOptions` with `BuildOptions{DualBook: false}` so market-wide consensus evaluation remains explicitly all-book.
- Updated `board.LineKey` (and `lineKey` via it) so `TOTAL` keys on market alone and `TEAM_TOTAL` keys on `TEAM_TOTAL|<team>`, keeping split Over/Under rows paired in one `LineGroup`.
- Updated `TestLineKeyCanonicalPlayerPropsAndGameLines` and added dual-book intersection, split-line non-averaging, and split `TOTAL`/`TEAM_TOTAL` pairing unit tests in `internal/board/board_test.go`.

### Task 2: Switch user-facing board callers in `internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, and `internal/web/slips.go` to the dual-book board builder and verify `internal/eval` and `internal/analyzer` tests stay green.

- Switched all user-facing board callers in `internal/web/pick.go` (5 sites), `internal/web/research.go` (2 sites), `internal/web/games.go` (1 site), and `internal/web/slips.go` (1 site) from `board.Build` to `board.BuildDualBook`.
- Updated single-book test fixtures and dual-book line/price expectations in `internal/web/games_test.go`, `internal/web/pick_test.go`, `internal/web/research_test.go`, and `internal/web/slips_test.go` so pickable rows include both `draftkings` and `fanduel` quotes and reflect non-averaged dual-book lines.
- Verified `go test ./internal/eval/... ./internal/analyzer/... ./internal/board/...`, `go test -short ./...`, and `golangci-lint run ./...` all pass with 0 issues.

## Deviations

### Task 1

- Kept `board.Build` in all-book mode (`BuildOptions{DualBook: false}`) for now and introduced `board.BuildDualBook` / `board.BuildWithOptions` so existing `internal/web` and `internal/refresh` callers and single-book tests continue to pass until Phase 2 wires user-facing surfaces to dual-book mode.

## Open issues

None.

## Next-phase notes

### Task 1

- Phase 2 should switch user-facing callers (`internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, `internal/web/slips.go`) to `board.BuildDualBook` and update single-book test fixtures in `internal/web/*_test.go` (and `internal/refresh/research_test.go` if needed) to provide both `draftkings` and `fanduel` quotes where pickable rows are expected.

### Task 2

- Phase 2 UI and template work can now rely on `Row.DK`, `Row.FD`, and `Row.LinesDiffer` being populated on every user-facing board row from `internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, and `internal/web/slips.go`.

## Files touched

- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/eval/market.go`
- `internal/web/games.go`
- `internal/web/games_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 600ms, lint 4.7s, test 7.8s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
