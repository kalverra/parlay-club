# Task 3

## Done

- Updated `docs/concepts/analyzer.md` to reflect that `internal/analyzer` (`LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`) now owns signal loading, freshness checks, team EPA aggregation, and player usage aggregation, while `AnalyzeSlip`, `ScoreLeg`, and `ScoreSlip` remain pure Go.
- Updated `docs/reference/analyzer-rules.md` (`How rules see a slip`) to point model, player stats, and `nflverse` EPA/usage loading references to `internal/analyzer` (`LoadSlipSignals`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Both `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` now match the Phase 2 signal-loading boundary in `internal/analyzer`.
