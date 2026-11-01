# Design: Slate, weather, and lines

## Context

Slips work with hand-entered legs and no games. This change brings in the week's games, weather, and game lines, and the
Games tab. Props wait for `props-research`. Read before starting:

- `plan/data-sources.md`: everything except Roster, Game log, Player matching, and the props endpoint.
- `plan/architecture.md`: Refresh.
- `plan/product.md`: Screens (Games tab, "lines as of").
- `docs/concepts/slips.md`, `docs/reference/odds-math.md`, `docs/concepts/live-updates.md`.

## Approach

- Each client in `internal/ingest/{espn,weather,odds}` takes a base URL and an `*http.Client`, returns plain structs,
  and never touches the DB. Fixtures are real responses saved while building.
- `internal/refresh` owns staleness, single-flight, backoff, credit floors, and writing results through the `Store`. It
  takes a `now` func and the three clients as interfaces. `Kick()` is called by middleware on full-page GETs.
- After a sync changes rows, refresh publishes `week:{id}` and `slip:{id}` for that week's slips.
- The SSE `LiveWindow` func from `slips-live` now checks game status and kickoff times.
- Quick-add pills call the existing add-leg path with fields prefilled; reference odds use `internal/parlay` with prices
  from the `odds` table for members' home books.
- Leg form gains a game picker limited to games not yet kicked off (past-week slips keep it optional).

## Risks

- ESPN field paths may differ from `plan/data-sources.md`. Confirm against fixtures first and fix the doc.
- Odds API team names may not equal ESPN `displayName` for every team. The full-slate match test catches it; add an
  alias map only if needed.
- Credits: tests never hit the real Odds API except the free `/events` live test.

## TODO

### Phase 1: ESPN scoreboard

- [ ] Save scoreboard fixtures (a full regular week, one with live games, one with an international game)
- [ ] Add internal/ingest/espn scoreboard client and parser with Sunday/Monday ET filter; test against fixtures
- [ ] Add Store upserts for weeks and games; test that rerunning a sync changes nothing
- [ ] Docs: add docs/reference/data-sources.md with the ESPN scoreboard section; delete it from plan/data-sources.md

### Phase 2: Weather

- [ ] Build the venue coordinates map from this season's scoreboards, international venues included
- [ ] Add internal/ingest/weather Open-Meteo client for the 3-hour kickoff window; test max wind, gust, precip
- [ ] Docs: add Open-Meteo and venues to docs/reference/data-sources.md; delete them from plan/data-sources.md

### Phase 3: Game lines

- [ ] Save /events and /odds fixtures; add internal/ingest/odds client with credit header parsing
- [ ] Add event matching to ESPN games; test zero misses across a full saved slate
- [ ] Map h2h, spreads, totals into odds rows per book; test against fixtures
- [ ] Add a TestLive per source that skips under -short
- [ ] Docs: add game lines, budget, and bookmakers to docs/reference/data-sources.md; delete them from plan/

### Phase 4: Refresh

- [ ] Add internal/refresh with staleness rules per source, single-flight, 5-minute backoff, and credit floors
- [ ] Wire Kick into full-page GET middleware and publish week and slip topics after changed syncs
- [ ] Fake-clock tests: refresh only when stale, never twice at once, never below a floor, backoff after errors
- [ ] Replace the SSE LiveWindow stub with the real game-window check; test it
- [ ] Docs: add docs/concepts/data-refresh.md; delete Refresh from plan/architecture.md

### Phase 5: Games tab

- [ ] Build game cards with local kickoff, lines (home book else median), weather badges, status, and lines-as-of note
- [ ] Add quick-add pills for both spreads, both moneylines, over and under, plus Custom leg with the game set
- [ ] Add the game picker and kickoff lock to the leg form, and reference odds on the slip
- [ ] Docs: update docs/reference/routes.md and docs/concepts/slips.md; delete Games tab from plan/product.md
