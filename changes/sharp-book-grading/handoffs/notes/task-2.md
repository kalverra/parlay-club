# Task 2

## Done

- Added `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with bookmaker tiering (Tier 1: `pinnacle`/`circa`/`bookmaker`, Tier 2: `lowvig`/`betonlineag`, Tier 3: non-DFS/non-exchange consensus with `>= 3` books on the same line).
- Implemented two-way quote pairing and de-vigging across `ML`, `SPREAD`, `TOTAL`, `TEAM_TOTAL`, and player props using `parlay.DevigTwoWay`, `parlay.DecimalToAmerican`, and `db.NormalizePlayerName`, while failing soft on one-sided markets (`ANYTIME_TD`), mismatched lines, and corrupt overrounds.
- Added unit tests in `internal/analyzer/sharp_test.go` covering tier precedence, all market types and player-name normalization, one-sided/invalid fail-soft behavior, and captured `regions=us` fixture payloads from `internal/ingest/odds/testdata/odds_nfl.json`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- `SharpKey` normalizes `Team` to uppercase (and `""` for `TOTAL`), `Side` to uppercase, and `PlayerName` via `db.NormalizePlayerName`; `sharpKeyForLeg` in `grade.go` should apply the same normalization so `Leg` lookups match `Game.Sharp`.

## Commit

Add tiered sharp-book benchmark builder to analyzer

Extracts de-vigged fair probabilities and lines from existing US-region and flat-rate prop quotes without adding paid API calls or extra regions. This gives the leg grader deterministic sharp-market benchmarks while failing soft on one-sided or incomplete markets.
