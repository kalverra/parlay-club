# Phase 1: Phase 1: Core build-grade diagnostics, provenance math, and Process vs. Outcome classifier

## Done

### Task 1: Add human-first implied probability helpers (`FormatImpliedReality` / `1 in X`) in `internal/parlay` and propagate unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` (2-word descriptor, max 3 `SignalPills`, max 2 `ContextRadar` notes tagged priced-in, and optional 1-line `CleanupNudge`) onto `LegGrade` and `SlipGrade` in `internal/analyzer`, with comprehensive unit tests.

- Added `FormatOneIn` and `FormatImpliedReality` in `internal/parlay/parlay.go` with unit tests in `internal/parlay/parlay_test.go` covering underdog/favorite ratios and `<1%` / `>99%` / invalid bounds.
- Extended `LegGrade` and `SlipGrade` in `internal/analyzer/grade.go` with `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `Diagnostics` (`BuildDiagnostics`).
- Implemented `internal/analyzer/diagnostics.go` with `SignalPill` (capped at 3, severity-ordered), `ContextNote` (`ContextRadar` capped at 2, tagged `"Already in the line"` or `"Likely Priced In"`), `DisplayGrade` (`+`/`-` band subdivisions), dominant-driver `Descriptor` mapping, `HoldPct`, `CorrTaxPct`, `CleanupNudge` (including heavy-chalk payout impact), and `ClassifyProcessOutcome`.
- Wired unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` into `EngineV2` in `internal/analyzer/grade_v2.go`, and added unit tests in `internal/analyzer/diagnostics_test.go`.

### Task 2: Add `ClassifyProcessOutcome` (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) in `internal/analyzer` with tests covering all 4 matrix states and ungraded/push edge cases.

- Verified `ClassifyProcessOutcome(letter string, won bool) ProcessOutcome` in `internal/analyzer/diagnostics.go` covering all 4 matrix cells (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) and ungraded/unknown letters.
- Added `ClassifyProcessResult(letter string, result parlay.Result) ProcessOutcome` in `internal/analyzer/diagnostics.go` so settled-slip callers can pass `parlay.Result` directly and get zero values for `Push`, `Void`, `Pending`, and `NeedsGrading`.
- Expanded `TestClassifyProcessOutcome` and added `TestClassifyProcessResult` in `internal/analyzer/diagnostics_test.go` covering all 4 matrix states across A–F grades (including `+`/`-`, lowercase, and whitespace), tone assertions, and ungraded/push/void/pending edge cases.

### Task 3: Docs: add `Build grade`, `Correlation tax`, and `Process vs. Outcome` terms to `GLOSSARY.md`.

- Added `Build grade`, `Correlation tax`, and `Process vs. Outcome` definitions to `GLOSSARY.md`.
- Verified `good-vibes docs lint GLOSSARY.md` passes with 0 errors and 0 warnings.

## Deviations

### Task 2

`ClassifyProcessOutcome` was already scaffolded in Task 1; Task 2 added `ClassifyProcessResult` for `parlay.Result` push/void/pending handling and expanded the matrix and edge-case test suite.

## Open issues

None.

## Next-phase notes

### Task 1

- `LegGrade` and `SlipGrade` expose `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `Diagnostics` as value fields; skipped legs (`Letter == ""`) keep zero values for backwards compatibility.
- `ClassifyProcessOutcome(letter string, won bool) ProcessOutcome` is ready for `/record` and `/slips/{id}` settled-slip badges.

### Task 2

- `/record` and `/slips/{id}` can call `analyzer.ClassifyProcessResult(letter, slip.Result)` (or `ClassifyProcessOutcome(letter, won)` when already narrowed to won/lost) to render settled-slip badges without misclassifying pushes as losses.

### Task 3

- Terminology for `Build grade`, `Correlation tax`, and `Process vs. Outcome` is now locked in `GLOSSARY.md` for Phase 2 (`internal/eval`) and Phase 3 (UI copy and Layer 3 popovers).

## Files touched

- `GLOSSARY.md`
- `internal/analyzer/diagnostics.go`
- `internal/analyzer/diagnostics_test.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/grade_v2.go`
- `internal/parlay/parlay.go`
- `internal/parlay/parlay_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 600ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
