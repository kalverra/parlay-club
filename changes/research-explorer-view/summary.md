---
slug: research-explorer-view
run: 20261126-044715-research-explorer-view
outcome: done
generated: 2026-11-26T05:49:07Z
---

# Run summary: research-explorer-view

## Outcome

- Status: done
- Branch: research-explorer-view
- Close-out: written
- Close: pending
- Final verdict: claude@claude-sonnet-5-5 approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 3
- Checkpoints: 6
- Duration: 41m6s
- Cost: $2.24
- Fresh input tokens: 1108618
- Cache read tokens: 63813622
- Cache write tokens: 269732
- Output tokens: 297708

| Model | Tokens | Cost |
| --- | ---: | ---: |
| claude-sonnet-5-5 | 4140540 | $1.21 |
| fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash-1017 | 61349140 | $1.03 |

## Phases

### Phase 1: Pure slate-wide candidate explorer in `internal/board`

Tasks:

- [x] Implement `ExploreQuery`, `ScoredRow`, `ExploreResult`, and `Explore` in `internal/board/explorer.go`, supporting targeted search (`Q`, `GameID`, `PlayerID`), open-exploration filters (`Category`, `Grades`, `Signal`, `Window`), facet counts, all `ExploreSort` orders, and `Limit`/`HasMore`, with comprehensive unit tests in `internal/board/explorer_test.go`.

Phase totals: 1 checkpoints, 6m0s, $0.15, 6923339 input / 51292 output / 6752092 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Implement `ExploreQuery`, `ScoredRow`, `ExploreResult`, and `Explore` in `internal/board/explorer.go`, supporting targeted search (`Q`, `GameID`, `PlayerID`), open-exploration filters (`Category`, `Grades`, `Signal`, `Window`), facet counts, all `ExploreSort` orders, and `Limit`/`HasMore`, with comprehensive unit tests in `internal/board/explorer_test.go`.

- `board.Explore` scores cached `Row`s with `analyzer.ScoreLeg` and returns `ScoredRow`s carrying the grade, a concise top-factor `Reason`, and `ImpliedPct`.
- Targeted filters (`Q`, `GameID`, `PlayerID`) and open-browse chips (`Category`, `Grades`, `Signal`, `Window`) work together, with facet counts tallied as if only that chip's own filter were cleared.
- All six `ExploreSort` orders are deterministic, with a `Limit` default of 50 and `HasMore` for the pre-slice total.
- Split `badgesFor` out of `ApplySignals` so the explorer can evaluate every signal badge for a row, not just the one it displays.
- `internal/board/explorer_test.go` covers every filter, every sort order, facet counts, paging, and the score fields.

### Phase 2: Primary Research nav tab and dual-mode explorer UI

Tasks:

- [x] Add `TabResearch` (`Research`, `/research`) to `internal/web/page.go` (using `grid-flow-col auto-cols-fr` in `web/templates/layout.html`) and set `page.Tab = TabResearch` in `internal/web/research.go`, updating nav/layout tests.
- [x] Redesign `GET /research` and add `GET /research/explore` (HTMX partial + `Hx-Push-Url`) in `internal/web/research.go`, `web/templates/research.html`, and `web/templates/partials.html` to support targeted game/player search and open-exploration filter/sort chips over cached data (plus an explicit `POST /research/games/{id}/props` button when a targeted game has no props yet), preserving existing `?game_id=`, `?player_id=`, and `?player=` views with integration and zero-refresher-call tests in `internal/web/research_test.go`.

Phase totals: 2 checkpoints, 12m14s, $0.33, 22403223 input / 87031 output / 22081524 cache read / 0 cache write tokens

Checkpoints: task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: Dedicated per-bet research view and open-slip handoff

Tasks:

- [x] Implement `GET /research/bet` (`BetResearchView`, `TeamBetContextView`, book pricing comparison, grade/warning breakdown, market-scoped `PlayerResearchView` for props, nfelo/EPA/weather/injury context for game markets, and optional `+ Add to slip` for eligible open slips) in `internal/web/research.go`, `web/templates/research_bet.html`, and `internal/web/render.go`, with tests covering player props, game markets, missing selections, and open-slip eligibility.
- [x] Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` to document the Research primary view, explorer filters/sorts, and per-bet research route.

Phase totals: 3 checkpoints, 21m18s, $1.72, 34837643 input / 148219 output / 34005446 cache read / 269732 cache write tokens

Checkpoints: task-4, task-5, final Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Implement `GET /research/bet` (`BetResearchView`, `TeamBetContextView`, book pricing comparison, grade/warning breakdown, market-scoped `PlayerResearchView` for props, nfelo/EPA/weather/injury context for game markets, and optional `+ Add to slip` for eligible open slips) in `internal/web/research.go`, `web/templates/research_bet.html`, and `internal/web/render.go`, with tests covering player props, game markets, missing selections, and open-slip eligibility.

- Added `GET /research/bet`, keyed on `db.SameSelection` fields (`game_id`, `market`, `side`, `team`,
  `player_id`, optional `player_name`), with a clean 404 page for selections that are not on the cached slate.
- Built `BetResearchView` with the bet summary, home-book / consensus / best cached price comparison, a
  de-vigged edge when both sides are cached, the full analyzer factor and warning breakdown, and the
  existing `PlayerResearchView` re-scoped to the bet's market and line for player props.
- Added `TeamBetContextView` for `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL`: nfelo model spread and win
  probabilities versus the market line, both teams' offensive and defensive EPA ranks, weather, venue, and
  key injuries.
- Added related bets in the same game, ahead of the same player's other markets, and linked each explorer
  card to its per-bet page while preserving the explorer URL in `from`.
- Added `+ Add to slip` for open slips that are still in the `pick` step and include the game. The form
  posts the existing `POST /slips/{id}/legs` route with `from=research` and returns to the bet page. An
  optional `slip_id` narrows the offer to that eligible slip.

### Task 5: Docs: update `docs/reference/routes.md` and `docs/concepts/slips.md` to document the Research primary view, explorer filters/sorts, and per-bet research route.

- `docs/reference/routes.md` lists the five-tab bottom bar, then documents `GET /research/explore` and
  `GET /research/bet` alongside the existing research routes, including the explorer's `q`, `game_id`,
  `player_id`, `cat`, `grade`, `signal`, `sort`, and `limit` contract and its zero-credit browse rule.
- The same doc records the explicit `POST /research/games/{id}/props` tap as the only paid research call,
  the `from=research` add-leg redirect, `HX-Push-Url`, and `/research/bet` in the 404 list.
- `docs/concepts/slips.md` now presents Research as a primary tab with three workflows: the cached
  explorer, the per-bet page, and the scoped game and player sheets. It covers the chip filters and sorts,
  the price and grade comparison, team context, related bets, and the open-slip `+ Add to slip` handoff.
- Corrected the stale route prose: research game and player sheets read cached SQLite and no longer sync
  ESPN data on demand, and the bottom bar has five destinations, not four.

## Final gate

- Rounds used: 2; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
