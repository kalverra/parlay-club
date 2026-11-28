## Done

- Wired `GroupBy`, `SortGrade`, `PickDetailView.Groups`, `PickDetailView.Sections` (`PickDetailSectionView`), `PickRowView.Grade`, and `PickSwapView.Group` through `internal/web/pick.go` (`pickQuery`, `pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickDetail`, `pickDetailView`, `pickHref`, `detailHref`, `swapURL`, `pickSwap`).
- Called `board.ScoreRows` in `pickPageData`, `pickSlateData`, `pickDetailData` (including at-limit `remainingSlots == 0`), and `pickPairRow` using `h.deps.GradingEngine`.
- Added `from=detail` handling (`renderPickDetailSwap`) to `addLeg`, `updateLeg`, and `deleteLeg` in `internal/web/slips.go`, returning `#pick-detail` with `OOB = true` and `Hx-Trigger: tray-updated, slip-updated`.
- Added handler and zero-extra-refresh tests in `internal/web/pick_test.go` covering `?group=`, `?sort=grade`, at-limit `swapURL`, `from=detail` add/update/delete, and `from=board` (`pickPairRow`).

## Deviations

Kept `PickDetailView.Category` mapped to `model.Category` (defaulting to `lines` when `cat` is omitted) and `linkQuery.GroupBy` mapped to `q.GroupBy` (omitting `group` from URLs when defaulted) so existing `pick-detail` template rendering and `pick_test.go` assertions continue passing until Task 5 updates `web/templates/partials.html`.

## Open issues

`web/templates/partials.html` still renders `Tabs`, `Lines`, and `Players` and does not yet render `PickDetailView.Groups`, `PickDetailView.Sections`, `PickRowView.Grade`, `#pick-group`, or `from=detail` forms in `GroupByGrade`.

## Next-phase notes

Task 5 should update `pick-detail`, `pick-line-group`, `pick-option`, and `pick-swap` in `web/templates/partials.html` to render `Groups`, `Sections`, `Grade` popovers, `All` + category filter chips (switching `PickDetailView.Category` to `string(q.Category)`), `#pick-group`, and `from=detail` in `GroupByGrade`.
