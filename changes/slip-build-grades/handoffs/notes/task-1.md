## Done

- Added `FormatOneIn` and `FormatImpliedReality` in `internal/parlay/parlay.go` with unit tests in `internal/parlay/parlay_test.go` covering underdog/favorite ratios and `<1%` / `>99%` / invalid bounds.
- Extended `LegGrade` and `SlipGrade` in `internal/analyzer/grade.go` with `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `Diagnostics` (`BuildDiagnostics`).
- Implemented `internal/analyzer/diagnostics.go` with `SignalPill` (capped at 3, severity-ordered), `ContextNote` (`ContextRadar` capped at 2, tagged `"Already in the line"` or `"Likely Priced In"`), `DisplayGrade` (`+`/`-` band subdivisions), dominant-driver `Descriptor` mapping, `HoldPct`, `CorrTaxPct`, `CleanupNudge` (including heavy-chalk payout impact), and `ClassifyProcessOutcome`.
- Wired unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` into `EngineV2` in `internal/analyzer/grade_v2.go`, and added unit tests in `internal/analyzer/diagnostics_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `LegGrade` and `SlipGrade` expose `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `Diagnostics` as value fields; skipped legs (`Letter == ""`) keep zero values for backwards compatibility.
- `ClassifyProcessOutcome(letter string, won bool) ProcessOutcome` is ready for `/record` and `/slips/{id}` settled-slip badges.
