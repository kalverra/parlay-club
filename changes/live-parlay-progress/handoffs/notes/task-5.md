# Task 5

## Done

- Exported `FindPlayer`, `PlayerMarketStat`, and `AnytimeTDCount` from `internal/grader` so live progress evaluation and final auto-grading share one canonical player lookup and stat mapping.
- Created `internal/progress` with `RemainingFraction` and `ClockLabel` handling scheduled, final, regulation `MM:SS`, halftime, end-of-period, overtime, and missing/unparseable live clocks.
- Implemented deterministic Normal-model `EvaluateLeg` evaluation for `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` anchored to `parlay.ImpliedProb(leg.Odds)`, including settled-state handling and monotonic early cash/bust for `TOTAL` and `TEAM_TOTAL`.
- Added unit tests in `internal/progress/progress_test.go` covering clock parsing, kickoff anchoring, late-game convergence, monotonic early cash/bust, and settled/fallback states.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `EvaluateLeg` currently returns `StateUnavailable` for player prop markets (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`); the next task can wire those into `EvaluateLeg` using `grader.FindPlayer`, `grader.PlayerMarketStat`, and `grader.AnytimeTDCount`, and add `EvaluateSlip`.

## Commit

Add progress package and export grader helpers

Live parlay tracking needs deterministic probability and clock evaluation shared with final grading. Exporting player stat helpers from grader and introducing internal/progress establishes the clock model and game/team market evaluators without duplicating stat rules.
