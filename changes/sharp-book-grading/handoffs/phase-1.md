# Phase 1: Phase 1: De-vig math and sharp benchmark builder

## Done

### Task 1: Add `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with table-driven tests in `internal/parlay/parlay_test.go`, and document the formulas in `docs/reference/odds-math.md`.

- Added `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with overround bounds `[0.95, 1.25]` and invalid-input guards.
- Added table-driven unit tests `TestImpliedProb`, `TestDevigTwoWay`, and `TestPriceEdge` to `internal/parlay/parlay_test.go`.
- Documented the new formulas and API entries in `docs/reference/odds-math.md` and removed deprecated `sources`/`last_verified` front-matter fields so `good-vibes docs lint` passes cleanly.

### Task 2: Add `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with unit tests covering tier precedence, two-way pairing for all markets, one-sided fail-soft, and captured `regions=us` fixture payloads.

- Added `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with bookmaker tiering (Tier 1: `pinnacle`/`circa`/`bookmaker`, Tier 2: `lowvig`/`betonlineag`, Tier 3: non-DFS/non-exchange consensus with `>= 3` books on the same line).
- Implemented two-way quote pairing and de-vigging across `ML`, `SPREAD`, `TOTAL`, `TEAM_TOTAL`, and player props using `parlay.DevigTwoWay`, `parlay.DecimalToAmerican`, and `db.NormalizePlayerName`, while failing soft on one-sided markets (`ANYTIME_TD`), mismatched lines, and corrupt overrounds.
- Added unit tests in `internal/analyzer/sharp_test.go` covering tier precedence, all market types and player-name normalization, one-sided/invalid fail-soft behavior, and captured `regions=us` fixture payloads from `internal/ingest/odds/testdata/odds_nfl.json`.

## Deviations

### Task 1

- `PriceEdge` returns `0` when `retailAmerican == 0` or `fairProb` is outside `(0, 1)` rather than returning a raw `fairProb - 0` difference, preventing invalid quotes from producing false positive edges.

### Task 2

- None.

## Open issues

### Task 1

- None.

### Task 2

- None.

## Next-phase notes

### Task 1

- `DevigTwoWay` returns `ok=false` when either price is `0` or the overround falls outside `[0.95, 1.25]`; `BuildSharpBenchmarks` can rely on `ok` directly to skip corrupt or mispaired quotes.

### Task 2

- `SharpKey` normalizes `Team` to uppercase (and `""` for `TOTAL`), `Side` to uppercase, and `PlayerName` via `db.NormalizePlayerName`; `sharpKeyForLeg` in `grade.go` should apply the same normalization so `Leg` lookups match `Game.Sharp`.

## Files touched

- `docs/reference/odds-math.md`
- `internal/analyzer/sharp.go`
- `internal/analyzer/sharp_test.go`
- `internal/parlay/parlay.go`
- `internal/parlay/parlay_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 300ms, lint 4.2s, test 8.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
