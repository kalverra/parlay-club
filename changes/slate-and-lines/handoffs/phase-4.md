# Phase 4: Refresh

## Done

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

## Deviations

- `Kick` is split into exported `Kick`/`KickContext` plus an unexported `kick` and `wait`. The design only named
  `Kick`; the extra shape is what lets tests wait for background work and drive the fake clock.
- The odds balance is written twice by design: `SetSyncCredits` stores the fresh balance immediately after the free
  `/events` call, and `RecordSync` stores it again with `fetched_at` when the pass finishes. This keeps the balance
  current when the free call succeeds but the paid call fails, without pretending the source is fresh.
- The credit floor is checked against the balance the free call just reported, not the stored balance. That is why
  the free call still runs when the stored balance is low; `oddsStale` controls frequency, and a stale low balance
  cannot pause the source forever.
- A below-floor pass still records `fetched_at` (with the balance and matches). The next balance check waits for the
  next staleness window, up to 24 hours, instead of rechecking on every page load. The design did not spell this out;
  it follows the credit guardrail.
- Odds rows carry `fetched_at`, and `UpsertOdds` treats a new fetch time as a change. A real odds sync therefore
  publishes even when every price is unchanged. Tests pass a fixed clock to exercise the true no-op path. This keeps
  the "lines as of" note moving, which is the intended UI behavior.
- `isFullPageGet` also drops paths ending in `/events` when the SSE `Accept` header is missing. The design named the
  header; the path check is an extra guard for clients and proxies that strip it.
- `LiveWindow` fails closed on a store read error after 5 seconds. The design only said it checks status and kickoff
  times; the failure policy comes from the live-updates guardrail that a stream must never stay open on a guess.

## Open issues

- No odds rows are pruned. `UpsertOdds` only inserts or updates its keyed rows, so a book that stops carrying a
  market, or a game whose event match changes, leaves the old row with an old `fetched_at` behind. Phase 5 should
  either filter by the latest sync time or add a prune.
- `games.odds_event_id` is `UNIQUE` and is never cleared. If a match must move to another game, the old value has to
  be cleared first or the write will collide. Matching assigns each event to at most one game, so today this only
  matters after a rename or realignment.
- `syncScoreboard` upserts but never deletes. A game ESPN drops from a week, or moves to another week, stays under its
  old week until it reappears and `week_id` is updated.
- The verification bullet "opening the same game twice inside the cache window spends credits once" is covered by the
  staleness table and the single-flight test, not by an end-to-end test that counts credits across two `Kick`s. That
  test would be cheap to add if phase 5 changes the trigger.
- `TestRunSourcesNeverRunsOneSourceTwiceAtOnce` uses a 250 ms sleep to let callers join the flight. It asserts the
  property that matters and is stable here, but it is timing-based.
- Weather skips a venue ID missing from the venue map silently, with no log. A new stadium or a changed ESPN venue ID
  means no forecast until the map is updated; the map covers the 2026 season.
- The live window stops at kickoff for a `SCHEDULED` game. If ESPN has not flipped the status to `LIVE`, a stream
  opened right at kickoff gets the 10 minute idle deadline until the next scoreboard sync. This is the documented
  behavior.
- Fixtures and docs were captured on 2026-11-07, leaving 497 Odds API credits after phase 3. Phase 4 tests make no
  live game-lines calls.

## Next-phase notes

- Phase 5 reads cards from `store.GamesForWeek(weekID)`. Games already carry kickoff, status, scores, venue, the
  weather fields (`TempF`, `WindMPH`, `WindGustMPH`, `PrecipIN`), and `OddsEventID`. Indoor games have nil weather.
- `store.OddsForGame(gameID)` returns every book and selection: market types `ML`, `SPREAD`, and `TOTAL`; team is the
  ESPN abbreviation (`""` for a total); side is `TEAM`, `OVER`, or `UNDER`; `Line` is nil for a moneyline. Use the
  member's `User.HomeBook` first, else pass the candidates' decimal prices to `parlay.ReferencePrice` with the own
  price as the fallback. Use the existing American/decimal helpers.
- The "lines as of" note should come from the newest `FetchedAt` on the game's odds rows, or `sync_state.fetched_at`
  for `odds:lines`. A game with no odds rows has no line.
- Because rows are not pruned, filter to the book you want, and ignore a row whose `fetched_at` predates the latest
  odds sync if the board must not mix old prices with new ones.
- Quick-add can build a `db.Leg` with `GameID`, `MarketType`, `Team` or `Side`, `Line`, and `Odds` from the chosen
  odds row; `slips.Service.AddLeg` already takes all of those. The existing slip rules still apply: membership, the
  per-member limit, and the lock.
- `refresh.LiveWindow` is already wired into `live.NewStream`, so the Games tab needs no SSE work for the live
  deadline.
- `Kick` is already wrapped around the app inside the auth and group middleware. The Games tab handler needs no extra
  refresh call; a page load is enough.
- Sync keys are `espn:scoreboard`, `weather:{gameID}`, and `odds:lines`. Only `odds:lines` stores
  `odds_credits_remaining`.
- Do not add a live game-lines test; it spends credits. `TestLiveEvents` is free, and the saved odds fixtures cover
  parsing.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/data-refresh.md`
- `docs/concepts/live-updates.md`
- `docs/llms.txt`
- `go.mod`
- `internal/db/games.go`
- `internal/db/odds.go`
- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/db/sync.go`
- `internal/db/sync_test.go`
- `internal/refresh/live.go`
- `internal/refresh/live_test.go`
- `internal/refresh/publish.go`
- `internal/refresh/publish_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/refresh/run_test.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`
- `plan/README.md`
- `plan/architecture.md`

## Run log

- Mode: 
- Checkpoints: task-10, task-11, task-12
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
