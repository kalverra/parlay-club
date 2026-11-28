# Phase 1: Phase 1: Add sportsbook combination-eligibility rules to the analyzer

## Done

### Task 1: Implement `book_combo_ineligible` (`Danger`) and `book_combo_restricted` (`Warn`) plus `CheckComboConflict` in `internal/analyzer/combo_rules.go`, register them in `internal/analyzer/analyzer.go` and `internal/analyzer/docs.go`, and add table-driven unit tests in `internal/analyzer/combo_rules_test.go`.

- Implemented `book_combo_ineligible` (`Danger`), `book_combo_restricted` (`Warn`), and `CheckComboConflict` in `internal/analyzer/combo_rules.go`.
- Registered both rules in `internal/analyzer/analyzer.go` (`rules`) and documented them in `internal/analyzer/docs.go` (`RuleDocs()`).
- Added table-driven unit tests in `internal/analyzer/combo_rules_test.go` covering hard blocks, SGP restrictions, `CheckComboConflict` priority, `RuleIDs`/`RuleDocs` registration, and valid combos.
- Updated existing multi-leg fixtures in `internal/analyzer/analyzer_test.go` whose synthetic same-game legs now trigger the new combo rules.
- Extracted `slipBoardRows` and `buildSlipMemberViews` from `(*slipHandlers).slipPageData` in `internal/web/slips.go` to clear the `gocyclo` lint check.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `analyzer.CheckComboConflict(candidate Leg, existing []Leg) (Warning, bool)` is exported and ready for `internal/board/signals.go` (`ApplySignals`).
- `CheckComboConflict` short-circuits on `Danger` (`book_combo_ineligible`) before `Warn` (`book_combo_restricted`), and returns `LegIDs` with non-zero IDs (omitting `candidate.ID` when `0`).

## Files touched

- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/combo_rules.go`
- `internal/analyzer/combo_rules_test.go`
- `internal/analyzer/docs.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 500ms, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 1
