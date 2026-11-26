## Done

- Added `TabResearch Tab = "research"` and a `Research` → `/research` entry to `navTabs` in `internal/web/page.go`, between This week and Record Book.
- `researchHandlers.page` now sets `Tab: TabResearch`, so `/research` renders the bottom bar with its own tab lit.
- `web/templates/layout.html` already used `grid-flow-col auto-cols-fr`; no edit was needed there.
- Updated the nav and layout tests for five tabs and added `TestLayoutMarksTheResearchTab` plus a tab-current assertion in `TestResearchPage`.

## Deviations

- The design's nav placement was unspecified. Research sits third, after This week and before Record Book, because bet discovery is a primary task rather than a reference screen.
- The design asked to "ensure" the layout uses `grid-flow-col auto-cols-fr`. That shipped with the FAQ docs view change, so this task only asserts it in `TestLayoutRendersBottomTabs`.

## Open issues

- Five tabs at 375px leave about 70px per label. "Record Book" may wrap to two lines. The decision doc's follow-up to re-check the bar at 375px is still open; shorten labels or padding if it looks crowded.
- `docs/reference/routes.md` (line 97) and `docs/concepts/slips.md` (line 47) still say the bar has four destinations. The Phase 3 docs task owns that rewrite.

## Next-phase notes

- Any new full-page research route (`GET /research/explore` without `Hx-Request`, `GET /research/bet`) must build its `Page` through `researchHandlers.page` so the tab stays current. HTMX partials bypass the layout, so they need no tab work.
- `page.Tab` is what shows or hides the whole bar; the pick board still passes an empty tab on purpose, and `TestPickBoardShowsSearchChipsSortsAndDock` asserts its page has no `aria-current`.

## Commit

Add Research to the primary bottom nav

Research was reachable only from a slip or pick row, so people could not go straight to bet research. The bar already sizes any number of tabs evenly, so the new destination needed no layout change.
