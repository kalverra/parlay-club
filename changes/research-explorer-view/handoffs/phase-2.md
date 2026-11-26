# Phase 2: Phase 2: Primary Research nav tab and dual-mode explorer UI

## Done

### Task 2: Add `TabResearch` (`Research`, `/research`) to `internal/web/page.go` (using `grid-flow-col auto-cols-fr` in `web/templates/layout.html`) and set `page.Tab = TabResearch` in `internal/web/research.go`, updating nav/layout tests.

- Added `TabResearch Tab = "research"` and a `Research` → `/research` entry to `navTabs` in `internal/web/page.go`, between This week and Record Book.
- `researchHandlers.page` now sets `Tab: TabResearch`, so `/research` renders the bottom bar with its own tab lit.
- `web/templates/layout.html` already used `grid-flow-col auto-cols-fr`; no edit was needed there.
- Updated the nav and layout tests for five tabs and added `TestLayoutMarksTheResearchTab` plus a tab-current assertion in `TestResearchPage`.

### Task 3: Redesign `GET /research` and add `GET /research/explore` (HTMX partial + `Hx-Push-Url`) in `internal/web/research.go`, `web/templates/research.html`, and `web/templates/partials.html` to support targeted game/player search and open-exploration filter/sort chips over cached data (plus an explicit `POST /research/games/{id}/props` button when a targeted game has no props yet), preserving existing `?game_id=`, `?player_id=`, and `?player=` views with integration and zero-refresher-call tests in `internal/web/research_test.go`.

- Redesigned `GET /research` into a dual-mode explorer: one debounced search box, game/player target
  pickers, and phone-first chip rows for bet type, grade, signals, and sort.
- Added `GET /research/explore` as the HTMX partial and set `Hx-Push-Url` to the matching `/research?...`
  URL, so filters and targets survive refreshes and shared links.
- Rendered candidate bet cards from `board.Explore` over cached week odds, with headshot/team media,
  selection text, grade, odds plus implied %, the top reason, signal pills, matchup, kickoff, and links
  into the player or game sheet.
- Added the explicit `POST /research/games/{id}/props` button, shown only when a targeted game has cached
  lines but no props, and made it the only research path that calls `SyncGameProps`.
- Added integration tests for cached browsing, chip filtering and odds sorting, `Hx-Push-Url`, the props
  button, preserved `?game_id=`/`?player_id=`/`?player=` views, and zero refresher calls on every research
  GET.

## Deviations

### Task 2

- The design's nav placement was unspecified. Research sits third, after This week and before Record Book, because bet discovery is a primary task rather than a reference screen.
- The design asked to "ensure" the layout uses `grid-flow-col auto-cols-fr`. That shipped with the FAQ docs view change, so this task only asserts it in `TestLayoutRendersBottomTabs`.

### Task 3

- The HTMX swap target is `#research-explore` (search form, chip bars, and cards together) rather than the
  design's `#research-results`. One response keeps facet counts, current-chip states, and the card list in
  sync. The design prose was corrected.
- Research game and player sheets no longer call `SyncGameResearch`/`SyncPlayerResearch`. The design's
  zero-credit guardrail and the task's zero-refresher requirement win over the old auto-sync, so the sheets
  read cached SQLite only. `SyncGameProps` is now the sole refresher call in the research handlers.
- Grade `C+` was undefined in the design. It filters to A, B, and C rows ("C or better").
- The candidate cards link to the existing player/game sheets for now. Their tap target becomes
  `/research/bet` when Phase 3 adds that route.
- Registered the new `research-explore` partial in `internal/web/render.go`; the task did not name that file
  but `RenderPartial` only serves registered names.

## Open issues

### Task 2

- Five tabs at 375px leave about 70px per label. "Record Book" may wrap to two lines. The decision doc's follow-up to re-check the bar at 375px is still open; shorten labels or padding if it looks crowded.
- `docs/reference/routes.md` (line 97) and `docs/concepts/slips.md` (line 47) still say the bar has four destinations. The Phase 3 docs task owns that rewrite.

### Task 3

- After an HTMX search or chip change, an already-open sheet below the explorer is not replaced, so it can
  describe the previously targeted game or player. Clicking the new "Open game sheet" or "Open player
  sheet" link reloads the full page correctly. Phase 3 may want an out-of-band sheet reset.
- The player picker still renders every rostered athlete for the week. `hx-preserve` avoids re-rendering
  the control, but each partial response still carries the full option list.
- `board.Explore`'s text search matches team abbreviations, not full team names. The game picker covers
  that case.
- The targeted game sheet still writes auto-resolved player matches through `UpsertPlayerMatches`. That is
  an existing SQLite cache write, not an external API call.
- The Task 2 note about five bottom tabs crowding at 375px still applies.

## Next-phase notes

### Task 2

- Any new full-page research route (`GET /research/explore` without `Hx-Request`, `GET /research/bet`) must build its `Page` through `researchHandlers.page` so the tab stays current. HTMX partials bypass the layout, so they need no tab work.
- `page.Tab` is what shows or hides the whole bar; the pick board still passes an empty tab on purpose, and `TestPickBoardShowsSearchChipsSortsAndDock` asserts its page has no `aria-current`.

### Task 3

- `ResearchCardView` now carries `MarketType`, `Side`, `Team`, `Line`, and `OddsValue` alongside the display
  fields, so `/research/bet` links can be built from a card without re-reading `board.Row`.
- Build per-bet links with `db.SameSelection` identity and keep the explorer's `q`, `cat`, `grade`,
  `signal`, and `sort` in a `from` URL so the back link restores the list.
- Keep `GET /research/bet` read-only: the zero-refresher tests use a recording refresher and will catch any
  accidental `Sync*` call.
- The explicit props button's form posts the current explorer filters, so a refreshed partial comes back on
  the same view.

## Files touched

- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/page_test.go`
- `internal/web/render.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/research.html`

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: generate reused, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
