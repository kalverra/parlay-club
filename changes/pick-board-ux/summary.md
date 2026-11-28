---
slug: pick-board-ux
run: 20261128-004054-pick-board-ux
outcome: done
generated: 2026-11-28T01:07:54Z
---

# Run summary: pick-board-ux

## Outcome

- Status: done
- Branch: pick-board-ux
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 3
- Checkpoints: 7
- Duration: 24m16s
- Cost: $—
- Fresh input tokens: 1721133
- Cache read tokens: 21505362
- Cache write tokens: 0
- Output tokens: 265701

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 20408289 | $— |
| gemini-4-neon-medium | 3083907 | $— |

## Phases

### Phase 1: Overall, hit chance, and payout value in the grade dropdown

Tasks:

- [x] Extend `GradeView`, `LegGradeView`, and `SlipGradeView` in `internal/web/analyzer.go` to compute `HitChance` (from `CalibratedProb`) and `PayoutLabel`/`PayoutToneClass` (from the `Price & Vig` pillar, free of `+EV %` copy), render the 3-column **Overall / Hit chance / Payout value** summary strip in `grade-breakdown` (`web/templates/partials.html`), and cover v2 legs, v2 multi-leg slips, and v1 fallback in `internal/web/analyzer_test.go` and `internal/web/slips_test.go`.
- [x] Docs: update `docs/concepts/analyzer.md` to document the Overall, Hit chance, and Payout value summary strip inside the grade breakdown popover.

Phase totals: 2 checkpoints, 3m18s, $—, 1873642 input / 29066 output / 1702836 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Extend `GradeView`, `LegGradeView`, and `SlipGradeView` in `internal/web/analyzer.go` to compute `HitChance` (from `CalibratedProb`) and `PayoutLabel`/`PayoutToneClass` (from the `Price & Vig` pillar, free of `+EV %` copy), render the 3-column **Overall / Hit chance / Payout value** summary strip in `grade-breakdown` (`web/templates/partials.html`), and cover v2 legs, v2 multi-leg slips, and v1 fallback in `internal/web/analyzer_test.go` and `internal/web/slips_test.go`.

- Extended `GradeView`, `LegGradeView`, `SlipGradeView`, and `buildGradeView` in `internal/web/analyzer.go` to populate `HitChance` from `CalibratedProb` via `formatHitChance` and `PayoutLabel`/`PayoutToneClass` from the `Price & Vig` pillar via `payoutValueFromPillars` without `+EV %` copy.
- Added the 3-column `data-grade-summary` strip (**Overall / Hit chance / Payout value**) to `grade-breakdown` in `web/templates/partials.html`.
- Updated `internal/web/analyzer_test.go` and `internal/web/slips_test.go` to cover v2 legs, v2 multi-leg slips, v1 fallback, and absence of `+EV` copy.

### Task 2: Docs: update `docs/concepts/analyzer.md` to document the Overall, Hit chance, and Payout value summary strip inside the grade breakdown popover.

- Updated `docs/concepts/analyzer.md` under "What the banner shows" to document the 3-column **Overall / Hit chance / Payout value** summary strip inside the `grade-breakdown` popover, including `CalibratedProb` formatting (`~N%`, `<1%`, `>99%`), `Price & Vig` payout labels (`Strong value`, `Fair value`, `Slight juice`, `Steep juice`) without `+EV %` copy, and `EngineV1` fallback behavior.

### Phase 2: Board grouping modes and pre-pick row scoring

Tasks:

- [x] Add `GroupBy` (`category`, `player`, `grade`), `SortGrade`, `Row.Grade`, `DetailSection`, and `ScoreRows` to `internal/board/board.go`, updating `board.Detail` so empty `GroupBy` defaults to `GroupByCategory` across all non-empty categories (with optional `Category` narrowing), `GroupByPlayer` merges each player's props across categories alongside a `Game lines` section, and `GroupByGrade` buckets individual sides by `A`–`F` with tests in `internal/board/board_test.go`.

Phase totals: 1 checkpoints, 3m25s, $—, 2769544 input / 37773 output / 2558464 cache read / 0 cache write tokens

Checkpoints: task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `GroupBy` (`category`, `player`, `grade`), `SortGrade`, `Row.Grade`, `DetailSection`, and `ScoreRows` to `internal/board/board.go`, updating `board.Detail` so empty `GroupBy` defaults to `GroupByCategory` across all non-empty categories (with optional `Category` narrowing), `GroupByPlayer` merges each player's props across categories alongside a `Game lines` section, and `GroupByGrade` buckets individual sides by `A`–`F` with tests in `internal/board/board_test.go`.

- Added `GroupBy` (`GroupByCategory`, `GroupByPlayer`, `GroupByGrade`), `DetailGroupBys`, `SortGrade` (and added it to `DetailSorts`), `Row.Grade`, `Query.GroupBy`, `DetailSection`, and `GameDetail.GroupBy` / `GameDetail.Sections` in `internal/board/board.go`.
- Added `board.ScoreRows(rows, games, stats, engine)` to score non-`OTHER` rows whose game is in `games`, defaulting `nil` engine to `analyzer.ActiveEngine`.
- Updated `board.Detail` to populate `Detail.Sections` across `GroupByCategory` (default, all non-empty categories in tab order with optional `Category` narrowing), `GroupByPlayer` (merging each player's props across categories alongside a `Game lines` section), and `GroupByGrade` (bucketing individual sides by `A`–`F` plus `Ungraded`, omitting empty buckets, and sorting by descending `Grade.Score` with active sort tiebreakers).
- Updated `sortLineGroups` and `sortPlayers` to support `SortGrade`.
- Added unit tests in `internal/board/board_test.go` covering `GroupBy` / `SortGrade` labels, `ScoreRows`, `GroupByCategory` (default and narrowing), `GroupByPlayer`, `GroupByGrade`, and `SortGrade`.

### Phase 3: Slip pick board grouped UI, pre-pick grade pills, and HTMX wiring

Tasks:

- [x] Wire `GroupBy`, `SortGrade`, and `board.ScoreRows` through `internal/web/pick.go` and `internal/web/slips.go` (including at-limit `pickDetailData`, `pickPairRow`, and `from=detail` add/update/delete swaps for `GroupByGrade`), with handler and zero-extra-refresh tests in `internal/web/pick_test.go`.
- [x] Update `pick-detail`, `pick-line-group`, and `pick-option` in `web/templates/partials.html` (and regenerate Tailwind CSS if needed) to render the `Group by` switcher (`Category` default, `Player`, `Grade`), `All` + category filter chips, grouped sections, and sibling pre-pick `Grade A–F` popovers anchored to full card width at 375px, with template/DOM assertions in `internal/web/pick_test.go`.
- [x] Docs: update `docs/concepts/slips.md` (staying within its line budget) and `docs/reference/routes.md` to document default pick-board grouping (`?group=`), pre-pick grades, and `from=detail`.

Phase totals: 4 checkpoints, 16m50s, $—, 16940427 input / 185076 output / 15793770 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
