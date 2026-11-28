# Phase 3: Phase 3: Slip pick board grouped UI, pre-pick grade pills, and HTMX wiring

## Done

### Task 4: Wire `GroupBy`, `SortGrade`, and `board.ScoreRows` through `internal/web/pick.go` and `internal/web/slips.go` (including at-limit `pickDetailData`, `pickPairRow`, and `from=detail` add/update/delete swaps for `GroupByGrade`), with handler and zero-extra-refresh tests in `internal/web/pick_test.go`.

- Wired `GroupBy`, `SortGrade`, `PickDetailView.Groups`, `PickDetailView.Sections` (`PickDetailSectionView`), `PickRowView.Grade`, and `PickSwapView.Group` through `internal/web/pick.go` (`pickQuery`, `pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickDetail`, `pickDetailView`, `pickHref`, `detailHref`, `swapURL`, `pickSwap`).
- Called `board.ScoreRows` in `pickPageData`, `pickSlateData`, `pickDetailData` (including at-limit `remainingSlots == 0`), and `pickPairRow` using `h.deps.GradingEngine`.
- Added `from=detail` handling (`renderPickDetailSwap`) to `addLeg`, `updateLeg`, and `deleteLeg` in `internal/web/slips.go`, returning `#pick-detail` with `OOB = true` and `Hx-Trigger: tray-updated, slip-updated`.
- Added handler and zero-extra-refresh tests in `internal/web/pick_test.go` covering `?group=`, `?sort=grade`, at-limit `swapURL`, `from=detail` add/update/delete, and `from=board` (`pickPairRow`).

### Task 5: Update `pick-detail`, `pick-line-group`, and `pick-option` in `web/templates/partials.html` (and regenerate Tailwind CSS if needed) to render the `Group by` switcher (`Category` default, `Player`, `Grade`), `All` + category filter chips, grouped sections, and sibling pre-pick `Grade A–F` popovers anchored to full card width at 375px, with template/DOM assertions in `internal/web/pick_test.go`.

- Updated `pickDetailView`, `PickRowView.FromDetail`, and `pickSwap` in `internal/web/pick.go` to default to `All` categories (`Category == ""`), prepend the `All` chip to `PickDetailView.Tabs`, set `FromDetail` on `GroupByGrade` rows, and target `#pick-detail` from `pick-swap` in `GroupByGrade`.
- Updated `pick-detail`, `pick-slate`, `pick-option`, and `pick-swap` in `web/templates/partials.html` to render the `Group by` switcher (`Category`, `Player`, `Grade`), `All` + category filter chips, grouped `Sections`, `#pick-group` hidden/OOB state, `from=detail` HTMX forms for `GroupByGrade`, and sibling `<details data-leg-grade data-help-popover>` grade breakdowns anchored across full card width (`absolute left-0 right-0 z-30`).
- Added `.pick-line-group { position: relative; }` in `web/tailwind/input.css` and regenerated `web/static/app.css` via `go generate ./...`.
- Updated existing `cat=lines` default assertions and added `TestPickDetailRendersGroupBySwitcherAllChipsSectionsAndSiblingGradePopovers` and `TestPickSwapInGroupByGradeTargetsPickDetail` in `internal/web/pick_test.go`.

### Task 6: Docs: update `docs/concepts/slips.md` (staying within its line budget) and `docs/reference/routes.md` to document default pick-board grouping (`?group=`), pre-pick grades, and `from=detail`.

- Updated `docs/concepts/slips.md` (579 lines, within `max_lines: 585` and `max_spans_per_100_lines: 105`) to document the `All` + category chip bar, **Group by** switcher (`?group=` with Category default, Player, Grade), Grade sort, sibling Grade A–F popovers backed by `board.ScoreRows`, and `from=detail` OOB `#pick-detail` swaps in Grade mode.
- Updated `docs/reference/routes.md` (617 lines) to document `?group=` in the pick-board query contract, optional `?cat=` defaulting to `All`, `sort=grade`, pre-pick `Grade A–F` popovers (including at-limit `pickDetailData` and `pickPairRow`), `#pick-group`/`#pick-cat`/`#pick-sort` OOB sync, and `from=detail` on `POST /slips/{id}/legs`, `PUT /legs/{id}`, `DELETE /legs/{id}`, and `GET /slips/{id}/pick/swap`.
- Verified `good-vibes docs lint` passes with `0 error(s)`.

## Deviations

### Task 4

Kept `PickDetailView.Category` mapped to `model.Category` (defaulting to `lines` when `cat` is omitted) and `linkQuery.GroupBy` mapped to `q.GroupBy` (omitting `group` from URLs when defaulted) so existing `pick-detail` template rendering and `pick_test.go` assertions continue passing until Task 5 updates `web/templates/partials.html`.

### Task 5

Kept `class="pick-line-group"` unchanged in `web/templates/partials.html` and added `.pick-line-group { position: relative; }` in `web/tailwind/input.css` instead of adding `relative` to the template class attribute, preserving existing `class="pick-line-group"` assertions across `internal/web/pick_test.go`.

## Open issues

### Task 4

`web/templates/partials.html` still renders `Tabs`, `Lines`, and `Players` and does not yet render `PickDetailView.Groups`, `PickDetailView.Sections`, `PickRowView.Grade`, `#pick-group`, or `from=detail` forms in `GroupByGrade`.

## Next-phase notes

### Task 4

Task 5 should update `pick-detail`, `pick-line-group`, `pick-option`, and `pick-swap` in `web/templates/partials.html` to render `Groups`, `Sections`, `Grade` popovers, `All` + category filter chips (switching `PickDetailView.Category` to `string(q.Category)`), `#pick-group`, and `from=detail` in `GroupByGrade`.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/tailwind/input.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
