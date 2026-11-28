## Done

- Updated `pickDetailView`, `PickRowView.FromDetail`, and `pickSwap` in `internal/web/pick.go` to default to `All` categories (`Category == ""`), prepend the `All` chip to `PickDetailView.Tabs`, set `FromDetail` on `GroupByGrade` rows, and target `#pick-detail` from `pick-swap` in `GroupByGrade`.
- Updated `pick-detail`, `pick-slate`, `pick-option`, and `pick-swap` in `web/templates/partials.html` to render the `Group by` switcher (`Category`, `Player`, `Grade`), `All` + category filter chips, grouped `Sections`, `#pick-group` hidden/OOB state, `from=detail` HTMX forms for `GroupByGrade`, and sibling `<details data-leg-grade data-help-popover>` grade breakdowns anchored across full card width (`absolute left-0 right-0 z-30`).
- Added `.pick-line-group { position: relative; }` in `web/tailwind/input.css` and regenerated `web/static/app.css` via `go generate ./...`.
- Updated existing `cat=lines` default assertions and added `TestPickDetailRendersGroupBySwitcherAllChipsSectionsAndSiblingGradePopovers` and `TestPickSwapInGroupByGradeTargetsPickDetail` in `internal/web/pick_test.go`.

## Deviations

Kept `class="pick-line-group"` unchanged in `web/templates/partials.html` and added `.pick-line-group { position: relative; }` in `web/tailwind/input.css` instead of adding `relative` to the template class attribute, preserving existing `class="pick-line-group"` assertions across `internal/web/pick_test.go`.

## Open issues

None.

## Next-phase notes

None.

## Commit

Render grouped pick board and pre-pick grade popovers

Give users Category, Player, and Grade grouping alongside an All category filter and sibling Grade A-F breakdown popovers anchored to full card width at 375px without nesting interactive controls inside pick forms or buttons.
