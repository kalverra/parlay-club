# Task 4 handoff

## Done

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

## Deviations

- `internal/web/slips.go` gained a `from=research` branch in `addLeg` so the existing add-leg route redirects
  back to the bet page instead of rendering the slip panel fragment. The task named three files; a working
  handoff needed this small route branch.
- Related bets exclude the other side of the same market. A hedge is not a useful related leg, so the list
  shows other markets, the same player's other props, and same-game legs instead.
- The explorer card keeps its existing player/game sheet links and gains a separate `Research bet →` link
  rather than turning the whole card into one tap target.

## Open issues

- `GET /research/bet` treats a kicked-off game as a missing selection. The analyzer and board already hide
  started games, so the page says the lines are closed rather than showing stale context.
- The player section still resolves its matchup context from `Store.LatestWeek`, the same as the player
  sheet. A bet on a game outside the latest cached week could show that week's matchup strip inside the
  player card. The bet header and team context always use the bet's own game.
- `docs/reference/routes.md` does not list `/research/bet` yet; the Phase 3 docs task should add it.

## Next-phase notes

- Build bet links with `researchBetHref` or the same `db.SameSelection` fields. Keep `line` and `odds` out
  of the URL so shared links survive line drift.
- Keep `/research/bet` read-only. The zero-refresher tests cover it with a recording refresher.
- The add-to-slip form uses one form with a `formaction` button per eligible slip, so it stays valid at
  375px and needs no JavaScript.
- The Phase 3 docs pass should also correct the four-destination nav prose noted in Task 3.

## Commit

Add dedicated per-bet research view

Bet research previously stopped at a game or player sheet, so a candidate card did not carry its own
prices, grade, and market context into one place. The dedicated page keeps the explorer's cached-only
promise while making the handoff to an open slip one tap.
