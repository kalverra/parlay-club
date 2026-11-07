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
- Groups later (`groups`): games, odds, weather, and `sync_state` stay global and shared, so a second group adds no
  credit use. The topics published after a sync go through one helper, so `groups` can fan out to each group's streams
  in one place.

## Verification

- `go test ./...` and `golangci-lint run ./...` pass; each `TestLive*` skips under `-short`.
- Saved fixtures parse the full slate, a live week, and an international game; rerunning a sync changes no rows.
- Event matching finds every game in a full saved slate, and weather parsing returns the kickoff-window max wind, gust,
  and precipitation.
- Fake-clock tests prove refresh runs only when stale, never twice at once, never below a credit floor, and backs off
  for 5 minutes after an error.
- Opening the same game twice inside the cache window spends credits once.
- Quick-add creates the selected spread, moneyline, or total with the game and price set; reference odds use the
  member's home book and fall back to the median.
- At phone width the Games tab shows kickoff, lines, weather badges, status, and the "lines as of" note.

## Risks

- ESPN field paths may differ from `plan/data-sources.md`. Confirm against fixtures first and fix the doc.
- Odds API team names may not equal ESPN `displayName` for every team. The full-slate match test catches it; add an
  alias map only if needed.
- Credits: tests never hit the real Odds API except the free `/events` live test.

## TODO

### Phase 1: ESPN scoreboard

- [x] Save scoreboard fixtures (a full regular week, one with live games, one with an international game), add the
  internal/ingest/espn scoreboard client and parser with the Sunday/Monday ET filter, and test against the fixtures
- [x] Add Store upserts for weeks and games and test that rerunning a sync changes nothing
- [ ] Docs: add docs/reference/data-sources.md with the ESPN scoreboard section; delete it from plan/data-sources.md

### Phase 2: Weather

- [ ] Build the venue coordinates map from this season's scoreboards, international venues included
- [ ] Add the Open-Meteo client for the 3-hour kickoff window and test max wind, gust, and precipitation
- [ ] Docs: add Open-Meteo and venues to docs/reference/data-sources.md; delete them from plan/data-sources.md

### Phase 3: Game lines

- [ ] Save /events and /odds fixtures, add the odds client with credit header parsing, and map h2h, spreads, and totals
  into odds rows per book with fixture tests
- [ ] Add event matching to ESPN games, test zero misses across a full saved slate, and add a TestLive per source that
  skips under -short
- [ ] Docs: add game lines, budget, and bookmakers to docs/reference/data-sources.md; delete them from plan/

### Phase 4: Refresh

- [ ] Add internal/refresh with staleness rules per source, single-flight, 5-minute backoff, and credit floors, with
  fake-clock tests for each rule
- [ ] Wire Kick into full-page GET middleware, publish week and slip topics after changed syncs, and replace the SSE
  LiveWindow stub with the real game-window check and its test
- [ ] Docs: add docs/concepts/data-refresh.md; delete Refresh from plan/architecture.md

### Phase 5: Games tab

- [ ] Build game cards with local kickoff, lines (home book else median), weather badges, status, and the lines-as-of
  note, plus quick-add pills for both spreads, both moneylines, over, under, and Custom leg
- [ ] Add the game picker and kickoff lock to the leg form, and reference odds on the slip
- [ ] Docs: update docs/reference/routes.md and docs/concepts/slips.md; delete Games tab from plan/product.md
