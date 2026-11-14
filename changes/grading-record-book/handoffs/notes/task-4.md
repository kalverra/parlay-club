# Task 4

## Done

- Added `GradedBy` to `LegView` and populated it in `legViews` via `legGradedBy` (`internal/web/slips.go`), mapping `slips.AutoGraderName` (`"auto"`) to `"Auto"` and preserving member names.
- Updated `resultClass` in `internal/web/slips.go` to return `border-amber-700 bg-amber-950 text-amber-300` for `NEEDS_GRADING`, and added `resultLabel` (registered in `templateFuncs` in `internal/web/render.go`) to render `"Needs grading"` for `NEEDS_GRADING` and `"{RESULT} · {GradedBy}"` when graded.
- Updated both the interactive `<summary>` chip and read-only `<span>` chip in `web/templates/partials.html` to render `{{resultLabel .Result .GradedBy}}`, keeping the five override options (`PENDING`, `WON`, `LOST`, `PUSH`, `VOID`) in the menu.
- Added `TestSlipPanelRendersGradedByAndNeedsGradingAmberChip` in `internal/web/slips_test.go` verifying `Auto` vs member attribution, `NEEDS_GRADING` amber styling and label, the five override menu values, and manual override updating the chip.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `resultLabel` and `resultClass` are registered on `templateFuncs()` in `internal/web/render.go` and shared across both member and non-member leg views in `web/templates/partials.html`.
