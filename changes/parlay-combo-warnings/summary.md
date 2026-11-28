---
slug: parlay-combo-warnings
run: 20261128-001722-parlay-combo-warnings
outcome: done
generated: 2026-11-28T00:34:24Z
---

# Run summary: parlay-combo-warnings

## Outcome

- Status: done
- Branch: parlay-combo-warnings
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 2
- Checkpoints: 4
- Duration: 14m17s
- Cost: $—
- Fresh input tokens: 1030562
- Cache read tokens: 13380180
- Cache write tokens: 0
- Output tokens: 153556

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 11318508 | $— |
| gemini-4-neon-medium | 3245790 | $— |

## Phases

### Phase 1: Add sportsbook combination-eligibility rules to the analyzer

Tasks:

- [x] Implement `book_combo_ineligible` (`Danger`) and `book_combo_restricted` (`Warn`) plus `CheckComboConflict` in `internal/analyzer/combo_rules.go`, register them in `internal/analyzer/analyzer.go` and `internal/analyzer/docs.go`, and add table-driven unit tests in `internal/analyzer/combo_rules_test.go`.

Phase totals: 1 checkpoints, 4m43s, $—, 2180733 input / 28599 output / 2047977 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Implement `book_combo_ineligible` (`Danger`) and `book_combo_restricted` (`Warn`) plus `CheckComboConflict` in `internal/analyzer/combo_rules.go`, register them in `internal/analyzer/analyzer.go` and `internal/analyzer/docs.go`, and add table-driven unit tests in `internal/analyzer/combo_rules_test.go`.

- Implemented `book_combo_ineligible` (`Danger`), `book_combo_restricted` (`Warn`), and `CheckComboConflict` in `internal/analyzer/combo_rules.go`.
- Registered both rules in `internal/analyzer/analyzer.go` (`rules`) and documented them in `internal/analyzer/docs.go` (`RuleDocs()`).
- Added table-driven unit tests in `internal/analyzer/combo_rules_test.go` covering hard blocks, SGP restrictions, `CheckComboConflict` priority, `RuleIDs`/`RuleDocs` registration, and valid combos.
- Updated existing multi-leg fixtures in `internal/analyzer/analyzer_test.go` whose synthetic same-game legs now trigger the new combo rules.
- Extracted `slipBoardRows` and `buildSlipMemberViews` from `(*slipHandlers).slipPageData` in `internal/web/slips.go` to clear the `gocyclo` lint check.

### Phase 2: Surface inline combo heads-up on the pick board

Tasks:

- [x] Wire `analyzer.CheckComboConflict` into `internal/board/signals.go` (`Row.ComboWarning`) and render the compact non-blocking heads-up note on conflicting pick options in `web/templates/` and `internal/web/pick.go`, with tests in `internal/board/signals_test.go` and `internal/web/pick_test.go`.
- [x] Docs: update `docs/concepts/analyzer.md` to document the new combination-eligibility rules and their non-blocking behavior.

Phase totals: 3 checkpoints, 8m50s, $—, 10824919 input / 111284 output / 10074742 cache read / 0 cache write tokens

Checkpoints: task-2, task-3, final Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Wire `analyzer.CheckComboConflict` into `internal/board/signals.go` (`Row.ComboWarning`) and render the compact non-blocking heads-up note on conflicting pick options in `web/templates/` and `internal/web/pick.go`, with tests in `internal/board/signals_test.go` and `internal/web/pick_test.go`.

- Added `Row.ComboWarning` to `internal/board/board.go` and wired `analyzer.CheckComboConflict` into `ApplySignals` in `internal/board/signals.go` for unpicked board rows.
- Mapped `Row.ComboWarning` onto `PickRowView.ComboWarning` in `internal/web/pick.go` and rendered a compact non-blocking `data-combo-warning` heads-up note in `web/templates/partials.html` (`pick-option`).
- Added unit tests in `internal/board/signals_test.go` (`TestApplySignalsComboWarning`) and `internal/web/pick_test.go` (`TestPickBoardRendersInlineComboWarningWithoutBlocking`).

### Task 3: Docs: update `docs/concepts/analyzer.md` to document the new combination-eligibility rules and their non-blocking behavior.

- Added the "Sportsbook combination eligibility" section to `docs/concepts/analyzer.md` documenting `book_combo_ineligible` (Danger) and `book_combo_restricted` (Warn), their triggering conditions, and their strictly non-blocking behavior.
- Documented both surfacing paths (`AnalyzeSlip` for the slip banner and `#pick-tray`, and `analyzer.CheckComboConflict` via `board.ApplySignals` for inline pick-board `Row.ComboWarning` notes) and updated the overview diagram's warning count from 16 to 18.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
