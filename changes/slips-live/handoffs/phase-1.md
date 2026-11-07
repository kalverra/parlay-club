# Phase 1: Parlay math

## Done

- Added `internal/parlay`, a pure package with no database or clock:
  - `AmericanToDecimal` and `DecimalToAmerican`, both treating `0` / `d ≤ 1` as "not a price" rather than erroring.
  - `CombinedOdds`, which multiplies counting leg decimals and returns `1` for no counting legs.
  - `ReferencePrice`, which returns the median of caller-supplied candidates, the mean of the middle two for an even
    count, and the leg's own price when there are no candidates. It sorts a clone, so callers keep their input order.
  - `Payout` with a profit-boost percentage, and `Units` for a graded result.
  - `Result` constants and `Result.Counts()`, used by `CombinedOdds` to drop `PUSH` and `VOID` legs.
- Table tests cover both conversion directions, a round trip, a 3-leg combo, push and void legs dropped, payout with and
  without boost, odd and even-count medians, unsorted candidates, units for every result, and `Counts()`.
- Added `docs/reference/odds-math.md` with the formulas, worked examples, and a function table.
- Deleted "Odds and payout" from `plan/product.md` and pointed the quick-add sentence at the new reference doc.
- Regenerated `docs/llms.txt`.

## Deviations

- `Result` and `Counts()` live in `parlay`, not `internal/slips`, so combined odds and units can decide what counts
  without taking a dependency on slip rules. Phase 2 should reuse this type rather than define a second one.
- `plan/product.md` only spells out `PUSH → 0` for units. The implementation also returns `0` for `VOID`, `PENDING`, and
  `NEEDS_GRADING`, which matches the grading rule that pushed and voided legs count as neither a hit nor a miss. The doc
  says so explicitly.
- No `ReferenceOdds` helper was added. The design's "reference price takes candidates" already makes the median pure, so
  reference odds are composed by the caller: `CombinedOdds` over legs whose decimal is `ReferencePrice(...)`. This keeps
  the package from knowing which books or members exist.
- Invalid prices return `0` instead of an error. There is no validation of negative stakes or impossible boosts in this
  phase; callers own input validation.

## Open issues

- `legs.odds` and `wagers.odds` are stored as American integers, while `parlay` works in decimal. Phase 2/4 must convert
  at the store and handler boundaries; do not store the converted value.
- `ReferencePrice` needs the candidate prices from the `odds` rows, filtered to the same game, market, player, side, and
  exact line, from each member's home book once. That query belongs to `internal/slips`, not this package.
- A slip with every leg pushed or voided combines to even money (`1`). Grading still gives the slip result `PUSH`, so
  callers should not read `CombinedOdds == 1` as a win condition.

## Next-phase notes

- Phase 2 can map the database's `PENDING`, `NEEDS_GRADING`, `WON`, `LOST`, `PUSH`, and `VOID` strings straight onto
  `parlay.Result`.
- Slip result recomputation needs only results, not prices, so it does not have to call this package beyond `Counts()`.
- When phase 2 moves "Editing and locking" into `docs/concepts/slips.md`, leave the odds formulas here and keep the
  `plan/product.md` link pointing at `docs/reference/odds-math.md`.
- Phase 4 owns the payout calculator and the record-book P/L. Both use the same profit term already documented:
  `stake × (D − 1) × (1 + boost/100)`.

## Files touched

- `docs/llms.txt`
- `docs/reference/odds-math.md`
- `internal/parlay/parlay.go`
- `internal/parlay/parlay_test.go`
- `plan/product.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
