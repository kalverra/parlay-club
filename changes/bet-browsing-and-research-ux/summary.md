---
slug: bet-browsing-and-research-ux
run: 20261128-014022-bet-browsing-and-research-ux
outcome: done
generated: 2026-11-28T02:20:05Z
---

# Run summary: bet-browsing-and-research-ux

## Outcome

- Status: done
- Branch: bet-browsing-and-research-ux
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 8 of 8
- Phases: 4
- Checkpoints: 8
- Duration: 34m21s
- Cost: $—
- Fresh input tokens: 2136727
- Cache read tokens: 31837315
- Cache write tokens: 0
- Output tokens: 303288

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 34277330 | $— |

## Phases

### Phase 1: Popularity sorting and collapsible pick-board groups

Tasks:

- [x] Add `SortPopular` as the default pick-board detail sort (with `SortGrade` second) in `internal/board/board.go` using cached `PlayerUsage`, market breadth, and line prominence signals, and update `internal/web/pick.go` and `internal/board/board_test.go` + `internal/web/pick_test.go`.
- [x] Convert `pick-detail` sections in `web/templates/partials.html` to collapsed-by-default `<details>` headers (auto-opened when searching) and preserve expanded section keys across `#pick-detail` HTMX swaps in `web/static/app.js`, with template tests in `internal/web/pick_test.go`.

Phase totals: 2 checkpoints, 8m9s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `SortPopular` as the default pick-board detail sort (with `SortGrade` second) in `internal/board/board.go` using cached `PlayerUsage`, market breadth, and line prominence signals, and update `internal/web/pick.go` and `internal/board/board_test.go` + `internal/web/pick_test.go`.

- Added `SortPopular` (`"popular"`) as the default pick-board detail sort in `internal/board/board.go` and reordered `DetailSorts` to `[SortPopular, SortGrade, SortPlayer, SortOdds, SortLine, SortMatchup]`.
- Implemented deterministic popularity scoring for game lines (`SPREAD` 400 > `TOTAL` 300 > `ML` 200 > `TEAM_TOTAL` 100) and player props (0–60 from cached `PlayerUsage`, up to 25 from game-wide prop market breadth, and up to 15 from primary line / implied probability prominence), with `Grade.Score` and alphabetical tie-breakers.
- Verified `internal/web/pick.go` routes, query parsing, and sort chips delegate cleanly to `board.DetailSorts` and `board.Detail` with `SortPopular` first and `SortGrade` second.
- Updated existing default-sort assertions and added unit/handler tests for popularity ranking in `internal/board/board_test.go` and `internal/web/pick_test.go`.

### Task 2: Convert `pick-detail` sections in `web/templates/partials.html` to collapsed-by-default `<details>` headers (auto-opened when searching) and preserve expanded section keys across `#pick-detail` HTMX swaps in `web/static/app.js`, with template tests in `internal/web/pick_test.go`.

- Converted `pick-detail` sections in `web/templates/partials.html` from `<section>` to native `<details class="group/section space-y-2" data-section="{{.Key}}" data-keep-open="{{.Key}}">` with `<summary>` headers containing the section badge, label, count, and rotating chevron.
- Set sections to render collapsed by default when `$.Search` is empty and auto-open (`open`) when `$.Search` is non-empty, and added `data-game="{{.GameID}}"` on `#pick-detail`.
- Added `snapshotPickDetailOpenState` and `restorePickDetailOpenState` in `web/static/app.js` across `htmx:beforeSwap` and `htmx:afterSwap` to preserve expanded `[data-keep-open]` section keys across `#pick-detail` swaps on the same game while clearing state when switching games.
- Added `TestPickDetailSectionsCollapseByDefaultAndAutoOpenWhenSearching` in `internal/web/pick_test.go` and regenerated `web/static/app.css` via `good-vibes verify generate`.

### Phase 2: Compact O/U pick rows and clear O/U slip badges

Tasks:

- [x] Add `IsOU`, `SharedLine`, and hoisted `SplitLine` fields to `PickLineGroupView` in `internal/web/pick.go` and render compact side-by-side Over/Under controls in `web/templates/partials.html`, verifying add/switch/remove/swap markup in `internal/web/pick_test.go`.
- [x] Add structured `SideBadge` and `SideClass` fields to `LegView` and `PickTrayLegView` in `internal/web/slips.go` and `internal/web/pick.go` and render prominent `OVER`/`UNDER` visual badges on `#slip-panel` and `#pick-tray` in `web/templates/partials.html` (with semantic theme styles in `web/tailwind/` and `go generate ./...`), covered by tests in `internal/web/slips_test.go` and `internal/web/pick_test.go`.

Phase totals: 2 checkpoints, 6m56s, $—, 5380168 input / 68178 output / 5014481 cache read / 0 cache write tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `IsOU`, `SharedLine`, and hoisted `SplitLine` fields to `PickLineGroupView` in `internal/web/pick.go` and render compact side-by-side Over/Under controls in `web/templates/partials.html`, verifying add/switch/remove/swap markup in `internal/web/pick_test.go`.

- Added `IsOU`, `SharedLine`, and hoisted `SplitLine` to `PickLineGroupView` in `internal/web/pick.go` and populated them in `pickLineGroupView` for paired `OVER`/`UNDER` line groups.
- Added compact side-by-side Over/Under controls (`pick-ou-option`) in `web/templates/partials.html` under `pick-line-group`, rendering one header (`Label · SharedLine` + hoisted `SplitLine` badge), directional `▲ Over` / `▼ Under` buttons with checkmark and `aria-pressed="true"` when picked, sibling `<details data-leg-grade>` popovers, and `closest .pick-line-group` HTMX swap targets across add, switch, remove, drift sync, and at-limit swap states.
- Updated existing O/U pick-board assertions and added `TestPickBoardCompactOUControlsAndHoistedSplitLine` in `internal/web/pick_test.go`.

### Task 4: Add structured `SideBadge` and `SideClass` fields to `LegView` and `PickTrayLegView` in `internal/web/slips.go` and `internal/web/pick.go` and render prominent `OVER`/`UNDER` visual badges on `#slip-panel` and `#pick-tray` in `web/templates/partials.html` (with semantic theme styles in `web/tailwind/` and `go generate ./...`), covered by tests in `internal/web/slips_test.go` and `internal/web/pick_test.go`.

- Added `SideBadge` and `SideClass` fields to `LegView` in `internal/web/slips.go` and `TrayLegView` in `internal/web/pick.go`, populated via `legSideBadge` for `OVER` and `UNDER` legs (empty for `SPREAD`, `ML`, `ANYTIME_TD`, and `OTHER`).
- Added semantic `.badge-over` and `.badge-under` styles in `web/tailwind/input.css` using theme tokens (`--positive*` and `--warning*`) and regenerated `web/static/app.css` via `go generate ./...`.
- Added the `side-badge` partial in `web/templates/partials.html` and rendered it on `#slip-panel` and `#pick-tray` leg rows.
- Added `TestSlipPanelRendersSideBadgesForOULegs` in `internal/web/slips_test.go` and `TestPickTrayRendersSideBadgesForOULegs` in `internal/web/pick_test.go`.

### Phase 3: Visual-first bet research hierarchy with FD/DK focus

Tasks:

- [x] Extract explicit FanDuel and DraftKings quotes (ordered with `homeBook` first) and visual pillar summary data on `BetResearchView` in `internal/web/research.go`, and reorder `web/templates/research_bet.html` to lead with the FD/DK snapshot, visual grade/warnings, and player/team graphics while moving all-book prices into a collapsed `<details>` below the fold, with tests in `internal/web/research_test.go`.

Phase totals: 1 checkpoints, 3m8s, $—, 1319314 input / 16126 output / 1215083 cache read / 0 cache write tokens

Checkpoints: task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Extract explicit FanDuel and DraftKings quotes (ordered with `homeBook` first) and visual pillar summary data on `BetResearchView` in `internal/web/research.go`, and reorder `web/templates/research_bet.html` to lead with the FD/DK snapshot, visual grade/warnings, and player/team graphics while moving all-book prices into a collapsed `<details>` below the fold, with tests in `internal/web/research_test.go`.

- Added `SideBadge` and `SideClass` to `BetResearchView` via `legSideBadge` and `SnapshotBooks` / `BookCount` to `BetPricingView` in `internal/web/research.go` via `extractSnapshotBooks` (ordering FanDuel and DraftKings with `homeBook` first).
- Reordered `web/templates/research_bet.html` to lead with the bet summary card (with `side-badge`), the FD vs DK snapshot + de-vig edge + 4-pillar visual bars + analyzer warnings, and player/team graphics, while moving the analyzer factor breakdown and a collapsed `<details>` (`Compare all N books`) below the fold.
- Added `TestResearchBetVisualHierarchyAndFDDKSnapshot` in `internal/web/research_test.go` verifying section order, `homeBook`-first FD/DK snapshot ordering, O/U side badge, pillar bars, and collapsed all-book details.

### Phase 4: Universal Research + Add actions and eligible-slip chooser

Tasks:

- [x] Share one `betResearchHref` helper across pick-board options, highlights legs/parlays, slip legs (with fallback for kicked-off or `OTHER` legs), research cards, and related bets in `internal/web/` and `web/templates/`, adding compact Research links to every bet tile with handler tests.
- [x] Add `GET /bets/add-sheet` in `internal/web/research.go`, mount `<dialog id="bet-add-dialog">` in `web/templates/layout.html` and `web/static/app.js`, and wire `+ Add` buttons on research cards, related bets, and cross-slip leg mentions to open the chooser (or add directly when picking for an active slip), with tests in `internal/web/research_test.go` and `internal/web/slips_test.go`.
- [x] Docs: update `docs/concepts/slips.md` and `docs/reference/routes.md` for collapsed groups, popularity-first sorting, compact O/U controls and slip badges, visual-first bet research, and `GET /bets/add-sheet`, then run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 15m30s, $—, 21715734 input / 180286 output / 20431503 cache read / 0 cache write tokens

Checkpoints: task-6, task-7, task-8 Review fixes: 0 Integration checks: 0

#### Done

### Task 6: Share one `betResearchHref` helper across pick-board options, highlights legs/parlays, slip legs (with fallback for kicked-off or `OTHER` legs), research cards, and related bets in `internal/web/` and `web/templates/`, adding compact Research links to every bet tile with handler tests.

- Replaced `researchBetHref` with a shared `betResearchHref(sel db.SelectionKey, from string, slipID int64) string` helper in `internal/web/research.go` that encodes `game_id`, `market`, `side`, `team`, `player_id`, `player_name`, `from`, and optional `slip_id`, returning `""` for missing game/market or `OTHER` legs.
- Wired `betResearchHref` into `PickRowView` (`pickRowView`, `pickRowFromLeg`), `LegHighlightView` (`highlightsView` and `legHighlightView` for both single-leg and parlay highlights), `LegView` (`legResearchURL`), `ResearchCardView` (`researchCards`), and related bets (`relatedBetCards`, preserving `slip_id`).
- Updated `legResearchURL` in `internal/web/slips.go` to link active non-`OTHER` legs to `/research/bet` when their game has not kicked off and fall back to `/research` player/game sheets for kicked-off or `OTHER` legs.
- Added compact `Research ->` links to `pick-option`, `pick-ou-option`, and `highlights-strip` (single legs and parlay legs) in `web/templates/partials.html`, plus related bet cards in `web/templates/research_bet.html`, keeping links outside `<form>` and `<button>` elements.
- Updated `TestResearchLinksOnSlipLegs` (covering upcoming, `OTHER`, and kicked-off legs) and added `TestBetResearchHrefSharedAcrossSurfaces` in `internal/web/research_test.go`.

### Task 7: Add `GET /bets/add-sheet` in `internal/web/research.go`, mount `<dialog id="bet-add-dialog">` in `web/templates/layout.html` and `web/static/app.js`, and wire `+ Add` buttons on research cards, related bets, and cross-slip leg mentions to open the chooser (or add directly when picking for an active slip), with tests in `internal/web/research_test.go` and `internal/web/slips_test.go`.

- Added `GET /bets/add-sheet` (`betAddSheet`, `BetAddSheetView`, `betAddSheetHref`) in `internal/web/research.go`, registered `"bet-add-sheet"` in `internal/web/render.go`, and added the `"bet-add-sheet"` partial in `web/templates/partials.html` to list eligible open slips in `StepPick` or render an empty/kicked-off state with a `+ New slip` link.
- Mounted `<dialog id="bet-add-dialog" class="leg-sheet">` with `#bet-add-dialog-body` in `web/templates/layout.html` and wired open/close/reset/HTMX `afterSwap` and `slip-updated` handlers in `web/static/app.js`.
- Wired `+ Add` buttons on Research Explorer cards (`web/templates/partials.html`), Related Bets (`web/templates/research_bet.html`, posting directly to `POST /slips/{id}/legs` when `slip_id` is an eligible active slip and opening `#bet-add-dialog` otherwise), and cross-slip leg rows in `slip-panel` (`internal/web/slips.go`, `web/templates/partials.html`).
- Added `TestBetAddSheetListsEligibleSlipsAndEmptyState` and `TestAddButtonsOnResearchCardsAndRelatedBets` in `internal/web/research_test.go`, plus `TestSlipHubCrossSlipLegAddButtons` in `internal/web/slips_test.go`.

### Task 8: Docs: update `docs/concepts/slips.md` and `docs/reference/routes.md` for collapsed groups, popularity-first sorting, compact O/U controls and slip badges, visual-first bet research, and `GET /bets/add-sheet`, then run `good-vibes docs lint`.

- Updated `docs/concepts/slips.md` to document collapsible pick-board `<details>` sections (collapsed by default, auto-expanded on search, preserved via `app.js`), `SortPopular` (`popular`) as the default sort, compact paired O/U controls, Over/Under slip and tray badges, visual-first `/research/bet` layout, universal `Research ->` and `+ Add` actions, and `GET /bets/add-sheet`.
- Updated `docs/reference/routes.md` to document `sort=popular` as the default detail sort, collapsible sections and compact O/U controls on `GET /slips/{id}/pick/detail`, Over/Under leg badges and `Research ->` / `+ Add` links on slip legs, the visual-first `/research/bet` hierarchy, and the `GET /bets/add-sheet` route contract.
- Verified both docs stay within their line and code-span budgets and ran `good-vibes docs lint` (`0 error(s), 14 warning(s)`, none in the updated files).

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
