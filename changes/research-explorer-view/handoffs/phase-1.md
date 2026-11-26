# Phase 1: Phase 1: Pure slate-wide candidate explorer in `internal/board`

## Done

### Task 1: Implement `ExploreQuery`, `ScoredRow`, `ExploreResult`, and `Explore` in `internal/board/explorer.go`, supporting targeted search (`Q`, `GameID`, `PlayerID`), open-exploration filters (`Category`, `Grades`, `Signal`, `Window`), facet counts, all `ExploreSort` orders, and `Limit`/`HasMore`, with comprehensive unit tests in `internal/board/explorer_test.go`.

- `board.Explore` scores cached `Row`s with `analyzer.ScoreLeg` and returns `ScoredRow`s carrying the grade, a concise top-factor `Reason`, and `ImpliedPct`.
- Targeted filters (`Q`, `GameID`, `PlayerID`) and open-browse chips (`Category`, `Grades`, `Signal`, `Window`) work together, with facet counts tallied as if only that chip's own filter were cleared.
- All six `ExploreSort` orders are deterministic, with a `Limit` default of 50 and `HasMore` for the pre-slice total.
- Split `badgesFor` out of `ApplySignals` so the explorer can evaluate every signal badge for a row, not just the one it displays.
- `internal/board/explorer_test.go` covers every filter, every sort order, facet counts, paging, and the score fields.

## Deviations

### Task 1

- `ExploreQuery` gained a `Signals BoardSignals` field. The design sketch did not include one, but signal chips and their counts need the full badge set; with an empty `Signals` the explorer falls back to each row's applied `Badge`.
- The design named `parlay.ImpliedProbability`; the real function is `parlay.ImpliedProb`. Design prose corrected in place.
- Tie-breakers extend the four named keys (`GameID`, `Market`, `PlayerName`, `Side`) with team, line, and price so two rows never tie fully. Sorts are independent of input order.
- An empty or unknown `Sort` reads as `grade`; unknown `Category`, `Window`, and `Signal` values are ignored so a stale shared link never empties the list.
- Signal filters match any badge a row earns, so a row showing "Smash matchup" still counts and filters under "High volume".

## Open issues

### Task 1

- `Explore` has no clock. Callers must hand it rows from `board.Build`, which already drops kicked-off games; the explorer itself does not re-check kickoff time.
- Signal counts under-report when `Signals` is empty, because only the single applied `Badge` is visible then. The web layer should pass `BoardSignals`.

## Next-phase notes

### Task 1

- Build the request's `BoardSignals` once and pass it in; `Explore` sets each returned row's `Badge` to its top badge, so templates can print the pill directly.
- `Limit <= 0` means the 50-row phone default; `HasMore` is computed before slicing, so "Show more" can re-query with a larger limit.
- The category filter is exact, with team totals separate from lines, matching the design's explorer chip list (unlike the game-detail tabs, which fold team totals into lines).
- `SignalLabel` and `ExploreSort.Label()` supply the chip text so the web templates do not repeat the badge wording.

## Files touched

- `internal/board/explorer.go`
- `internal/board/explorer_test.go`
- `internal/board/signals.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 700ms, lint 5.9s, test 8.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
