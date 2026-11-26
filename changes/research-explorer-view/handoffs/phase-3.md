# Phase 3: Phase 3: Dedicated per-bet research view and open-slip handoff

## Done

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

## Deviations

### Task 4

- `internal/web/slips.go` gained a `from=research` branch in `addLeg` so the existing add-leg route redirects
  back to the bet page instead of rendering the slip panel fragment. The task named three files; a working
  handoff needed this small route branch.
- Related bets exclude the other side of the same market. A hedge is not a useful related leg, so the list
  shows other markets, the same player's other props, and same-game legs instead.
- The explorer card keeps its existing player/game sheet links and gains a separate `Research bet →` link
  rather than turning the whole card into one tap target.

## Open issues

### Task 4

- `GET /research/bet` treats a kicked-off game as a missing selection. The analyzer and board already hide
  started games, so the page says the lines are closed rather than showing stale context.
- The player section still resolves its matchup context from `Store.LatestWeek`, the same as the player
  sheet. A bet on a game outside the latest cached week could show that week's matchup strip inside the
  player card. The bet header and team context always use the bet's own game.
- `docs/reference/routes.md` does not list `/research/bet` yet; the Phase 3 docs task should add it.

### Task 5

- `docs/concepts/data-refresh.md` still says loading game or player research calls `SyncGameResearch` or
  `SyncPlayerResearch`. Task 3 removed those calls from the research handlers, so that doc now contradicts
  the code. This task only owned `routes.md` and `slips.md`.
- `docs/reference/routes.md` is at its 600-line reference cap and `docs/concepts/slips.md` is at its
  575-line concept cap. The next edit to either must trim elsewhere or add a budget override with a reason.
- Task 4's open issues still stand: `/research/bet` treats a kicked-off game as missing, and the player
  section resolves matchup context from `Store.LatestWeek` rather than the bet's own game.

## Next-phase notes

### Task 4

- Build bet links with `researchBetHref` or the same `db.SameSelection` fields. Keep `line` and `odds` out
  of the URL so shared links survive line drift.
- Keep `/research/bet` read-only. The zero-refresher tests cover it with a recording refresher.
- The add-to-slip form uses one form with a `formaction` button per eligible slip, so it stays valid at
  375px and needs no JavaScript.
- The Phase 3 docs pass should also correct the four-destination nav prose noted in Task 3.

### Task 5

- Keep route contracts in `routes.md` and workflow description in `slips.md`; the two pages are at their
  line caps, so move new mechanics to a reference doc instead of growing them.
- The close-out pass should correct `docs/concepts/data-refresh.md` and check for any other prose that still
  says Research is not a bottom-bar tab or that the bar has four destinations.
- The explorer and per-bet pages are read-only against SQLite; the zero-refresher tests cover `/research`,
  `/research/explore`, and `/research/bet`, so future docs should keep the explicit props button framed as
  the only paid path.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/research_bet.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: test 7.5s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
