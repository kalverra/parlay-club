# Phase 4: Phase 4: Universal Research + Add actions and eligible-slip chooser

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 6

`betResearchHref` is ready for the remaining Phase 4 tasks (`GET /bets/add-sheet` and `#bet-add-dialog`).

### Task 7

`GET /bets/add-sheet`, `#bet-add-dialog`, and universal `+ Add` actions are in place across Research Explorer, Related Bets, and cross-slip leg rows.

### Task 8

All Phase 4 tasks are complete and living documentation matches the implemented bet-browsing, O/U clarity, visual-first research, and eligible-slip chooser behavior.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/pick.go`
- `internal/web/render.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/layout.html`
- `web/templates/partials.html`
- `web/templates/research_bet.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7, task-8
- Verify: verify passed: test 10.2s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
