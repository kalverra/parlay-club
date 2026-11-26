# Task 5 handoff

## Done

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

None.

## Open issues

- `docs/concepts/data-refresh.md` still says loading game or player research calls `SyncGameResearch` or
  `SyncPlayerResearch`. Task 3 removed those calls from the research handlers, so that doc now contradicts
  the code. This task only owned `routes.md` and `slips.md`.
- `docs/reference/routes.md` is at its 600-line reference cap and `docs/concepts/slips.md` is at its
  575-line concept cap. The next edit to either must trim elsewhere or add a budget override with a reason.
- Task 4's open issues still stand: `/research/bet` treats a kicked-off game as missing, and the player
  section resolves matchup context from `Store.LatestWeek` rather than the bet's own game.

## Next-phase notes

- Keep route contracts in `routes.md` and workflow description in `slips.md`; the two pages are at their
  line caps, so move new mechanics to a reference doc instead of growing them.
- The close-out pass should correct `docs/concepts/data-refresh.md` and check for any other prose that still
  says Research is not a bottom-bar tab or that the bar has four destinations.
- The explorer and per-bet pages are read-only against SQLite; the zero-refresher tests cover `/research`,
  `/research/explore`, and `/research/bet`, so future docs should keep the explicit props button framed as
  the only paid path.

## Commit

Document the Research explorer and per-bet pages

Research now has a primary tab, a cached-only explorer, and a dedicated bet page, so the route reference and
slip concept doc described a screen that no longer existed. The docs now match the shipped filters, sorts,
prices, team context, and open-slip handoff.
