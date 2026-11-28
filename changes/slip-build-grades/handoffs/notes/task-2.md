## Done

- Verified `ClassifyProcessOutcome(letter string, won bool) ProcessOutcome` in `internal/analyzer/diagnostics.go` covering all 4 matrix cells (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) and ungraded/unknown letters.
- Added `ClassifyProcessResult(letter string, result parlay.Result) ProcessOutcome` in `internal/analyzer/diagnostics.go` so settled-slip callers can pass `parlay.Result` directly and get zero values for `Push`, `Void`, `Pending`, and `NeedsGrading`.
- Expanded `TestClassifyProcessOutcome` and added `TestClassifyProcessResult` in `internal/analyzer/diagnostics_test.go` covering all 4 matrix states across A–F grades (including `+`/`-`, lowercase, and whitespace), tone assertions, and ungraded/push/void/pending edge cases.

## Deviations

`ClassifyProcessOutcome` was already scaffolded in Task 1; Task 2 added `ClassifyProcessResult` for `parlay.Result` push/void/pending handling and expanded the matrix and edge-case test suite.

## Open issues

None.

## Next-phase notes

- `/record` and `/slips/{id}` can call `analyzer.ClassifyProcessResult(letter, slip.Result)` (or `ClassifyProcessOutcome(letter, won)` when already narrowed to won/lost) to render settled-slip badges without misclassifying pushes as losses.
