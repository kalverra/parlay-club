# Task 1

## Done

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

- `analyzer.CheckComboConflict(candidate Leg, existing []Leg) (Warning, bool)` is exported and ready for `internal/board/signals.go` (`ApplySignals`).
- `CheckComboConflict` short-circuits on `Danger` (`book_combo_ineligible`) before `Warn` (`book_combo_restricted`), and returns `LegIDs` with non-zero IDs (omitting `candidate.ID` when `0`).

## Commit

Add sportsbook combo eligibility rules to analyzer

DraftKings and FanDuel reject mutually exclusive, duplicate, and
redundant same-game legs and restrict several SGP prop combinations.
Detecting these combinations in the analyzer lets slips and the pick
board surface non-blocking warnings before users build an unplaceable
parlay.
