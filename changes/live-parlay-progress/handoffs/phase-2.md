# Phase 2: Phase 2: Build deterministic live progress and hit probability engine

## Done

### Task 5: Export shared player lookup and market stat extraction from `internal/grader` and create `internal/progress` with game-clock parsing (`RemainingFraction`) and live evaluation for game/team markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`), with unit tests covering kickoff anchoring, halftime, OT, missing clock, and monotonic early cash/bust.

- Exported `FindPlayer`, `PlayerMarketStat`, and `AnytimeTDCount` from `internal/grader` so live progress evaluation and final auto-grading share one canonical player lookup and stat mapping.
- Created `internal/progress` with `RemainingFraction` and `ClockLabel` handling scheduled, final, regulation `MM:SS`, halftime, end-of-period, overtime, and missing/unparseable live clocks.
- Implemented deterministic Normal-model `EvaluateLeg` evaluation for `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` anchored to `parlay.ImpliedProb(leg.Odds)`, including settled-state handling and monotonic early cash/bust for `TOTAL` and `TEAM_TOTAL`.
- Added unit tests in `internal/progress/progress_test.go` covering clock parsing, kickoff anchoring, late-game convergence, monotonic early cash/bust, and settled/fallback states.

### Task 6: Add live evaluation for all player prop markets (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, `ANYTIME_TD`), `OTHER`/unlinked fallbacks, and full-parlay aggregation (`EvaluateSlip`) in `internal/progress`, with tests covering yardage non-monotonicity, absent players during live games, pushes/voids, and busted parlays.

- Added live player prop evaluation in `internal/progress/props.go` for `PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, plus `OTHER` and unlinked fallbacks in `internal/progress/progress.go`.
- Handled non-monotonic yardage props (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) as `StateLive` until final, monotonic early cash/bust for `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, absent players in populated live box scores as `0`, and unpopulated box scores (`box == nil || !box.HasStats`) as `"Live · stats pending"` at pre-game probability.
- Implemented `EvaluateSlip(legs []LegProgress) SlipProgress` in `internal/progress/slip.go`, multiplying live probabilities across counting legs, excluding `PUSH`/`VOID` legs, averaging leg progress, and collapsing busted parlays to `0%`.
- Added unit tests in `internal/progress/props_test.go` covering kickoff anchoring for all 8 player prop markets, yardage non-monotonicity, monotonic early settlement, absent players, pending box scores, unlinked legs, pushes/voids, and busted/cashed parlays.

### Task 7: Docs: document the live progress and hit probability formulas in `docs/reference/odds-math.md`.

- Updated `docs/reference/odds-math.md` front matter, summary, and API table to cover `internal/progress` alongside `internal/parlay`.
- Documented `RemainingFraction` and `ClockLabel` game-clock parsing rules, regulation/OT/halftime clamping, and missing-clock fallbacks.
- Documented `EvaluateLeg` anchoring, settled/scheduled/unavailable/pending-box-score states, Normal-model formulas (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, and yardage/volume props with market CVs and monotonic early settlement), and Poisson models (`ANYTIME_TD`, `PASS_TDS`).
- Documented `EvaluateSlip` parlay aggregation, `PUSH`/`VOID` exclusion, progress averaging, and tone rules.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 5

- `EvaluateLeg` currently returns `StateUnavailable` for player prop markets (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`); the next task can wire those into `EvaluateLeg` using `grader.FindPlayer`, `grader.PlayerMarketStat`, and `grader.AnytimeTDCount`, and add `EvaluateSlip`.

### Task 6

- `internal/progress` now exports `EvaluateLeg(leg db.Leg, game *db.Game, box *espn.BoxScore) LegProgress` and `EvaluateSlip(legs []LegProgress) SlipProgress`, ready for Phase 3 wiring in `internal/web` (`slips.go`, `home.go`, and templates).

### Task 7

- Phase 2 is complete; Phase 3 can wire `progress.EvaluateLeg` and `progress.EvaluateSlip` into `internal/web` (`slips.go`, `home.go`, and templates) and update `docs/` for the UI behavior if needed.

## Files touched

- `docs/reference/odds-math.md`
- `internal/grader/grader.go`
- `internal/progress/clock.go`
- `internal/progress/math.go`
- `internal/progress/progress.go`
- `internal/progress/progress_test.go`
- `internal/progress/props.go`
- `internal/progress/props_test.go`
- `internal/progress/slip.go`

## Run log

- Mode: 
- Checkpoints: task-5, task-6, task-7
- Verify: verify passed: test 1.2s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
