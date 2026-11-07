---
slug: slate-and-lines
run: 20261107-194704-slate-and-lines
outcome: done
generated: 2026-11-07T20:42:19Z
---

# Run summary: slate-and-lines

## Outcome

- Status: done
- Branch: slate-and-lines
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 15 of 15
- Phases: 5
- Checkpoints: 15
- Duration: 52m59s
- Cost: $2.29
- Input tokens: 51350078
- Output tokens: 458083
- Cache read tokens: 46492910

## Phases

### Phase 1: ESPN scoreboard

Tasks:

- [x] Save scoreboard fixtures (a full regular week, one with live games, one with an international game), add the internal/ingest/espn scoreboard client and parser with the Sunday/Monday ET filter, and test against the fixtures
- [x] Add Store upserts for weeks and games and test that rerunning a sync changes nothing
- [x] Docs: add docs/reference/data-sources.md with the ESPN scoreboard section; delete it from plan/data-sources.md

Phase totals: 3 checkpoints, 9m19s, $0.32, 6670295 input / 70119 output / 6008344 cache tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/ingest/espn`, a client with a base URL, shared `*http.Client`, 10 second default timeout,
  `User-Agent: parlay-club`, and JSON decoding. It returns plain structs and never touches the database.
- Added the scoreboard client and parser:
  - `ScoreboardParams` sends `dates={season}`, `seasontype`, and `week`; the zero value asks ESPN for the current week.
  - `parseScoreboard` keeps only events whose kickoff is Sunday or Monday in `America/New_York`, with `time/tzdata`
    embedded so the zone resolves in a container. Monday night games stored in UTC on Tuesday stay on the slate, and
    the 9:30 a.m. ET London kickoff stays too.
  - Kickoff parsing accepts RFC 3339 with seconds and ESPN's usual minute precision.
  - Event state maps `pre` → `SCHEDULED`, `in` → `LIVE`, `post` → `FINAL`; an unknown state is an error.
  - Competitors map to home and away by `homeAway`, with `team.id`, `abbreviation`, `displayName`, and score. A
    scheduled game has no score even when ESPN sends a placeholder zero. A missing home or away team is an error.
  - Venue maps `id`, `fullName`, and `indoor`.
- Saved three real scoreboard fixtures and a README under `internal/ingest/espn/testdata/`:
  - `scoreboard_week.json`: week 9, 15 events including the Thursday game and the Madrid game. The parser returns the
    14 Sunday and Monday games.
  - `scoreboard_live.json`: week 4 during Sunday afternoon games, with 8 live, 1 final, and 6 scheduled slate games.
  - `scoreboard_international.json`: week 5, trimmed to the London game, proving the 9:30 a.m. ET kickoff survives.
- Added the store side:
  - `db.Game`, `db.GameStatus`, and `db.ErrGameStatusInvalid`.
  - `Store.UpsertGame` inserts or updates only the slate-owned fields: week, teams and names, ESPN IDs, kickoff,
    status, venue, indoor flag, and scores. Odds, weather, and injuries are left alone.
  - The conflict update carries a `WHERE` guard over every owned column, so a rerun with the same data leaves the row
    and its `updated_at` untouched.
  - `Store.GameByID` returns a full row or `ErrGameNotFound`.
  - Tests cover the create path, an unchanged rerun after later syncs wrote odds/weather/injuries, a changed live game
    that updates only slate fields, an unknown status refusal, and a missing lookup.
  - `UpsertWeek` already existed from `slips-live` with its own create-and-reuse test, so this phase did not need to
    add it.
- Docs: added `docs/reference/data-sources.md` with the client contract, the scoreboard endpoint, the field table, the
  Sunday/Monday filter, and the upsert rules. Regenerated `docs/llms.txt`. Deleted the Scoreboard section from
  `plan/data-sources.md`, leaving the Summary section for later.
- Verification: `go test ./...` passes and `golangci-lint run ./...` reports 0 issues.

### Phase 2: Weather

Tasks:

- [x] Build the venue coordinates map from this season's scoreboards, international venues included
- [x] Add the Open-Meteo client for the 3-hour kickoff window and test max wind, gust, and precipitation
- [x] Docs: add Open-Meteo and venues to docs/reference/data-sources.md; delete them from plan/data-sources.md

Phase totals: 3 checkpoints, 7m27s, $0.25, 3825729 input / 49651 output / 3260140 cache tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/ingest/weather`, a client with a base URL, shared `*http.Client`, 10 second default timeout,
  `User-Agent: parlay-club`, and a small JSON `get` helper. It returns plain structs and never touches the database,
  matching the ESPN client from phase 1.
- Added the venue map in `venues.go`:
  - `Venue` carries ESPN's venue ID and name plus the stadium's latitude and longitude.
  - `venues` holds 38 entries, every unique venue on the 2026 regular-season scoreboards for weeks 1 through 18,
    including the eight international venues: Estadio Banorte (Mexico City), FC Bayern Munich Stadium, Maracanã,
    Melbourne Cricket Ground, Santiago Bernabéu, Stade de France, Tottenham Hotspur Stadium, and Wembley.
  - Coordinates are for the stadium, not the city center.
  - `VenueByID` returns the venue and `true`, or an empty venue and `false` for an ID the map does not know.
- Added the Open-Meteo forecast client in `forecast.go`:
  - `Forecast(ctx, venue, kickoff)` requests `temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m` with mph,
    Fahrenheit, inches, and UTC.
  - The window is the hour containing kickoff plus the two after it. `start_hour` is kickoff in UTC truncated to the
    hour; `end_hour` is two hours later. Kickoff minutes do not move the window.
  - `temp_f` is the temperature at the kickoff hour. `wind_mph`, `wind_gust_mph`, and `precip_in` are the window's
    maxima.
  - Parsing requires every hourly series to line up, every hour of the window to be present and in order, and every
    value in the window to be non-null. A missing hour, a null, or a short window is an error rather than a zero.
- Saved four fixtures and a source table under `internal/ingest/weather/testdata/`:
  - `venues_2026.json`: every unique venue from the 2026 scoreboards.
  - `forecast_lumen.json`: Lumen Field across the week 9 Sunday kickoff window.
  - `forecast_highmark.json`: Highmark Stadium across the week 9 Monday kickoff window.
  - `forecast_short_window.json`: a two-hour response that proves a short window is an error.
- Tests cover map coverage and exact names against the season fixture, international venues, the unknown-ID path, the
  full query the client puts on the wire, the kickoff-window maxima, kickoff-hour temperature versus the window maximum,
  the short-window error, and an HTTP 503.
- Docs: added the Open-Meteo endpoint, the kickoff-window rules, and the venue-coordinate rules to
  `docs/reference/data-sources.md`; regenerated `docs/llms.txt`; deleted the Open-Meteo and venue sections from
  `plan/data-sources.md`.
- Verification: `go test ./...` and `go test -short ./...` pass, `golangci-lint run ./internal/ingest/weather/...`
  reports 0 issues, and `good-vibes docs lint` is ok.

### Phase 3: Game lines

Tasks:

- [x] Save /events and /odds fixtures, add the odds client with credit header parsing, and map h2h, spreads, and totals into odds rows per book with fixture tests
- [x] Add event matching to ESPN games, test zero misses across a full saved slate, and add a TestLive per source that skips under -short
- [x] Docs: add game lines, budget, and bookmakers to docs/reference/data-sources.md; delete them from plan/

Phase totals: 3 checkpoints, 8m45s, $0.36, 6682835 input / 72933 output / 5895736 cache tokens

Checkpoints: task-7, task-8, task-9 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/ingest/odds`, a client for The Odds API with a base URL, an API key, a shared `*http.Client`, a 10
  second default timeout, and `User-Agent: parlay-club`. It returns plain structs and never touches the database.
  `NewClient("", "")` fails with `ErrNoAPIKey` before any request goes out.
- Added credit parsing. Every response, the free event list included, must carry a readable `X-Requests-Remaining` or the
  call is an error. `X-Requests-Used` and `X-Requests-Last` are informational and default to zero. `Credits` is returned
  beside the data.
- Added `Client.Events`, the free `/events` list. It parses `commence_time` as RFC 3339, stores it in UTC, and returns
  `ID`, `CommenceTime`, `HomeTeam`, and `AwayTeam`.
- Added `Client.Lines`, the `/odds` call with `regions=us`, `markets=h2h,spreads,totals`, and `oddsFormat=american`. It
  maps one `Line` per book and selection: `h2h` → `ML`, `spreads` → `SPREAD`, `totals` → `TOTAL`, with `Side` set to
  `TEAM`, `OVER`, or `UNDER`. `Line` is NULL for a moneyline and set for a spread or total.
- Markets the client does not read produce no rows, so the same client can serve props later. A book that omits a market
  produces no rows for it. A missing price, a spread or total with no point, an empty team name, or a total outcome that
  is not Over or Under is an error.
- Added `MatchEvents`, which pairs an Odds API event with an ESPN game when both team names equal the game's display
  names and `commence_time` is within `MatchWindow` (12 hours) of kickoff, in either direction. It returns the
  game-ID-to-event-ID map plus unmatched events in input order. Each game and event is used at most once.
- Saved two fixtures and a source table under `internal/ingest/odds/testdata/`:
  - `events_nfl.json`: 27 events across weeks 9 and 10, including the Monday night game. Free.
  - `odds_nfl.json`: the same 27 events, 204 bookmaker blocks, and 1,162 priced selections. Costs 3 credits.
- Tests cover the event list, the full on-the-wire query, credit headers, HTTP failures on both endpoints, the required
  API key, the full line mapping (1,162 rows), exact DraftKings rows for the Madrid game, unknown markets, malformed
  markets, and matching. The slate test matches all 14 saved week 9 games with zero misses, including the Madrid game
  and the Monday night game, and returns the 13 next-week events as unmatched. The matching test covers the 12 hour
  boundary on both sides, different names, and swapped sides.
- Added `TestLiveEvents`, which hits only the free `/events` endpoint, skips under `-short`, and skips when
  `ODDS_API_KEY` is unset. Also added the missing `TestLiveScoreboard` and `TestLiveForecast`, so every source now has a
  live test that skips under `-short`. `TestLiveForecast` also skips when the request cannot reach Open-Meteo at all;
  HTTP and parse errors still fail it.
- Docs: moved game lines, credits, bookmakers, event matching, and the credit budget into
  `docs/reference/data-sources.md`; deleted the game-line, budget, and bookmaker detail from `plan/data-sources.md`;
  regenerated `docs/llms.txt`.
- Verification: `go test ./...`, `go test -short ./...`, `go vet`, and `golangci-lint run ./internal/ingest/odds/...`
  pass. This sandbox cannot reach Open-Meteo (TLS handshake failure, and curl gets 403), so `TestLiveForecast` skips
  here; the odds and saved-fixture tests do not use the network.

### Phase 4: Refresh

Tasks:

- [x] Add internal/refresh with staleness rules per source, single-flight, 5-minute backoff, and credit floors, with fake-clock tests for each rule
- [x] Wire Kick into full-page GET middleware, publish week and slip topics after changed syncs, and replace the SSE LiveWindow stub with the real game-window check and its test
- [x] Docs: add docs/concepts/data-refresh.md; delete Refresh from plan/architecture.md

Phase totals: 3 checkpoints, 15m8s, $0.70, 19290830 input / 151215 output / 17905240 cache tokens

Checkpoints: task-10, task-11, task-12 Review fixes: 0 Integration checks: 0

#### Done

- Added `internal/refresh`, the package that owns lazy sync. `Kick`/`KickContext` detach from the request, run in the
  background, and return at once. Each source gets its own 30 second timeout. Tests wait through an internal
  `WaitGroup`; the server never waits.
- One pass reads the latest week, syncs `espn:scoreboard` first, re-reads the week's games, then starts one weather
  source per game and one `odds:lines` source for the week in parallel.
- Added the staleness rules in `rules.go`:
  - Scoreboard: 10 minutes while any game is `LIVE`, 30 minutes on Sunday or Monday in `America/New_York`, 6 hours
    otherwise. A source that has never run is stale.
  - Weather: hourly inside the 48 hour pre-kickoff window. Indoor games, unknown venues, games more than 48 hours out,
    and games at or past kickoff never fetch.
  - Odds: 24 hours Monday through Friday, 12 hours Saturday, 45 minutes Sunday before the last Sunday kickoff, 3 hours
    Sunday after that and all day Monday. A week with no games, or with every game kicked off, never fetches.
- Added single-flight keyed by source key. The work inside the flight re-reads `sync_state` and skips the fetch when a
  caller that joined late finds the source fresh. A failed fetch or write pauses that source for 5 minutes in memory;
  a later success clears the pause. Failures log, keep old rows, and leave `fetched_at` alone. `time/tzdata` is
  embedded so the Eastern rules work in the container.
- Added the odds pass: call the free `/events` list, match it to the week's games, persist the fresh balance and the
  matches even when no lines are bought, skip the paid `/odds` call below 50 remaining credits, and otherwise buy one
  response for the whole week. Only lines for matched games are written; other upcoming events are dropped. Team
  names are converted from The Odds API display name to the game's ESPN abbreviation. Each response's remaining
  credits are stored in `sync_state.odds_credits_remaining`.
- Added the store surface refresh needs: `GamesForWeek`, `GameByID`, `SetGameWeather`, `SetGameOddsEvent`,
  `UpsertOdds`, `OddsForGame`, `SlipIDsForWeek`, `SyncState`, `RecordSync`, and `SetSyncCredits`. Weather and odds
  writes only move `updated_at` or report a change when values really changed. `UpsertOdds` is keyed on
  game/book/market/team/player/side.
- Added `publishWeek`, which publishes `week:{id}` and `slip:{id}` for every slip in the week. The scoreboard, weather,
  and odds syncs all call it after their transaction commits and only when a row changed.
- Added the real live-window rule in `internal/refresh/live.go`: a week is live when any game is `LIVE`, or any
  `SCHEDULED` game kicks off within 30 minutes. A `FINAL` game never counts, and a scheduled game at or past kickoff
  does not either. `Service.LiveWindow(weekID)` reads the week with a 5 second timeout and fails closed to the idle
  deadline.
- Wired everything in `cmd/server/main.go`: real ESPN, Open-Meteo, and Odds clients; `LiveWindow` passed to
  `live.NewStream`; and `kickOnPage` inside the sign-in and group middleware, outside `CSRF`. Only full-page `GET`s
  kick: no HTMX partials, no SSE (`Accept: text/event-stream` or a path ending in `/events`), no non-`GET`s.
- Tests:
  - `rules_test.go` covers every scoreboard, odds, and weather window, including the exact boundaries, game days,
    the 48 hour lookahead, indoor/unknown/no venue games, and the last-Sunday-kickoff switch.
  - `run_test.go` proves only due sources run, 12 concurrent callers share one run, a failure backs off 5 minutes and
    retries after, and success clears the backoff.
  - `sources_test.go` proves the slate writes and a rerun changes nothing, weather writes and a repeated forecast
    changes nothing, odds write only matched games across `ML`/`SPREAD`/`TOTAL` and record credits, the paid call is
    skipped below the floor while free matches are still stored, a stored low balance does not wedge the source, and
    a kick uses the fake clock's staleness.
  - `publish_test.go` proves each changed sync publishes the week and its slips once and a no-op sync publishes
    nothing.
  - `live_test.go` covers the rule table and the store-backed service check.
  - `cmd/server/main_test.go` covers the full-page, HTMX, SSE, SSE-path, and form-post cases, and a signed-in page
    load kicking.
  - DB tests cover sync state and credits, game ordering and weather/event changes, odds upserts, and slip IDs across
    groups.
- Docs: added `docs/concepts/data-refresh.md`, updated `docs/concepts/live-updates.md` to point at
  `refresh.Service.LiveWindow`, deleted the Refresh section from `plan/architecture.md`, updated the `plan/README.md`
  row, and regenerated `docs/llms.txt`.
- Verification: `go test ./...`, `go test -short ./...`, `go vet ./...`, and `golangci-lint run ./...` pass with 0
  issues. `good-vibes docs lint` is ok. Live tests skip under `-short`; the phase adds no metered live test.

### Phase 5: Games tab

Tasks:

- [x] Build game cards with local kickoff, lines (home book else median), weather badges, status, and the lines-as-of note, plus quick-add pills for both spreads, both moneylines, over, under, and Custom leg
- [x] Add the game picker and kickoff lock to the leg form, and reference odds on the slip
- [x] Docs: update docs/reference/routes.md and docs/concepts/slips.md; delete Games tab from plan/product.md

Phase totals: 3 checkpoints, 12m20s, $0.65, 14880389 input / 114165 output / 13423450 cache tokens

Checkpoints: task-13, task-14, task-15 Review fixes: 0 Integration checks: 0

#### Done

- `/games` is now its own handler instead of a shell placeholder. It reads `LatestWeek`, `GamesForWeek`, and each game's
  `OddsForGame` rows, then renders one card per game in kickoff order.
- Each card shows the matchup (`MIA @ BUF`, plus full names), the kickoff as a `<time datetime="{UTC ISO}">` that
  `app.js` formats in the browser's zone, a status chip (Upcoming, Live, Final), the score for a live or final game, and
  the two spreads and the over and under.
- Prices and lines come from the viewer's `HomeBook` when that book prices the selection, and from the median across
  every book otherwise. Medians use `parlay.ReferencePrice` for prices and a sorted median for lines.
- Weather badges: 🏠 Indoor for a dome, 💨 wind mph (amber from 15, red from 20), and 🌧 at 0.1 in/hr of precipitation or
  more. An indoor game or a game with no forecast shows no weather.
- Odds rows from an older sync are dropped per game, so a card never mixes a stale price with the current board. This
  resolves the stale-odds open issue from phase 4; no prune was added.
- The "lines as of" note uses the newest current odds row on the board, or the `odds:lines` `sync_state.fetched_at` when
  that is newer. A board with neither says "No lines yet this week."
- Quick-add pills for both spreads, both moneylines, Over, Under, and Custom leg. A pill posts the normal
  `POST /slips/{id}/legs` route with `quick=1` and the game, market, team, side, line, and price prefilled, swaps an
  "Added {selection}" confirmation into that card, and keeps `Hx-Trigger: leg-saved`. Custom leg opens the bottom sheet
  with the game preselected and targets the same card.
- Quick add targets the newest slip of the week and shows the pills only while it is `OPEN`. With no slip the card links
  to `/slips/new`; with a locked slip it says picks cannot be added.
- The leg form gained a game picker. A current week lists only games that have not kicked off and requires a game; a past
  week lists every game and allows "No game". `?game_id=` preselects a game, and `?quick=1` makes the form target the
  game card. A kicked-off pick still answers 422 from the existing kickoff lock.
- The slip payout now shows **Reference** odds next to Combined. It is the product of each counting leg's reference
  price: one price per slip member's home book for the same game, market, player, side, and line, reduced with
  `parlay.ReferencePrice`. A leg no member book prices keeps its own price.
- Docs: `docs/reference/routes.md` documents the Games route, the stale-row filter, the note, quick add, and the new leg
  form parameters; `docs/concepts/slips.md` covers quick add, the game picker, reference odds, and the Games tab;
  `plan/product.md` no longer carries the Games tab or "lines as of" bullets; `docs/guides/local-dev.md` and
  `docs/llms.txt` follow.
- Tests: `internal/web/games_test.go` covers a full card, live and final cards, stale-row filtering, the home-book
  choice, all six quick-add markets, the custom-leg target, and a moneyline-only board. `internal/web/slips_test.go`
  covers the picker (upcoming games only, past-week history), the kickoff 422, and reference odds across two member
  books. `internal/web/layout_test.go` renders the Games page through the shell.
- Verification: `go test ./...`, `go test -count=1 ./...`, `go test -short -race ./internal/web/ ./internal/slips/`,
  `go vet ./...`, and `golangci-lint run ./...` pass. `good-vibes docs lint` and `good-vibes check` pass.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
