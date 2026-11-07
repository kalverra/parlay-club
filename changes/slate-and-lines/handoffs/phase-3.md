# Phase 3: Game lines

## Done

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

## Deviations

- The odds client constructor takes an API key as its second argument: `odds.NewClient(baseURL, apiKey, httpClient)`.
  The design said every client takes just a base URL and an `*http.Client`; the key is required for auth. The design's
  Approach is updated.
- `/odds` returns every upcoming event, not just the synced week: the saved response has weeks 9 and 10. Matching keeps
  only games on the saved slate, so refresh must write lines only for matched games and treat the other events as
  unmatched. The docs say this and the slate test asserts the 13 extra events.
- No alias map was needed. Exact `displayName` equality matched all 14 games on the saved week 9 slate, including the
  international game. The risk stays for later weeks and renamed teams.
- The plan keeps its free `/events` line under props. Game lines, the budget, and bookmakers were deleted, but event
  matching is shared with props-research, so the endpoint reference stayed in the props section.
- Team names on a `Line` stay The Odds API's display names. The `odds` table expects an ESPN abbreviation, so the caller
  maps them; doing it here would couple the client to ESPN.
- `TestLiveForecast` skips when the transport cannot reach Open-Meteo. This sandbox blocks that host, and a blocked
  network is not a parser regression. The test still fails on an HTTP status or a parse failure, so it keeps checking
  the parser wherever the host is reachable. The ESPN and Odds API live tests stay strict because both hosts answer
  here.

## Open issues

- Nothing writes odds yet. `games.odds_event_id` is set only on insert, and `UpsertGame` deliberately leaves it alone.
  There is no way to set the match and no odds upsert or delete, so phase 4 needs store methods such as
  `SetGameOddsEvent` and `UpsertOdds` (with `fetched_at`) before it can persist lines.
- `MatchEvents` returns only game ID to event ID. Lines arrive keyed by event ID, so refresh must invert the map or match
  each line's `EventID` back to a game.
- One `/odds` call covers many events and weeks. Lines for unmatched events must be dropped; otherwise the `odds` table
  would take rows for games that do not exist locally and violate the foreign key.
- Matching is exact on `displayName` with a 12 hour window and is tested only against the saved week 9 slate. A renamed
  or relocated team may need an alias map in a later week.
- The stale-after rules and credit floors in `docs/reference/data-sources.md` are policy only. No code enforces them
  until phase 4.
- The docs and fixtures were captured on 2026-11-07 and spent 3 credits, leaving 497. Re-capture both fixtures together
  if the query changes, so their event IDs still line up.

## Next-phase notes

- In refresh, call `Events` first (free), match against the week's games, then call `Lines` once for the week. `Lines`
  costs about 3 credits; skip it below the 50 credit floor and when every game has kicked off.
- Persist `Credits.Remaining` into `sync_state.odds_credits_remaining` on every odds call. A missing remaining header is
  an error by design, so treat it as a sync failure rather than guessing a balance.
- Write odds rows only for games in `MatchEvents`' result. Set `games.odds_event_id` from that map, and set `fetched_at`
  to the sync time. A book that omits a market is normal, not an error.
- Convert `Line.Team` from the Odds API display name to the game's ESPN abbreviation before writing. The game's
  `home_name` and `away_name` already give the mapping for the two teams.
- The odds fixtures are pinned to the exact query in `linesQuery()`. If the query changes, capture new fixtures instead
  of hand-editing these.
- `TestLiveEvents` spends zero credits and needs only `ODDS_API_KEY`. Do not add a live game-lines test; it would spend
  credits.

## Files touched

- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `internal/ingest/espn/live_test.go`
- `internal/ingest/odds/client.go`
- `internal/ingest/odds/client_test.go`
- `internal/ingest/odds/events.go`
- `internal/ingest/odds/events_test.go`
- `internal/ingest/odds/lines.go`
- `internal/ingest/odds/lines_internal_test.go`
- `internal/ingest/odds/lines_test.go`
- `internal/ingest/odds/live_test.go`
- `internal/ingest/odds/match.go`
- `internal/ingest/odds/match_test.go`
- `internal/ingest/odds/testdata/README.md`
- `internal/ingest/odds/testdata/events_nfl.json`
- `internal/ingest/odds/testdata/odds_nfl.json`
- `internal/ingest/weather/live_test.go`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
