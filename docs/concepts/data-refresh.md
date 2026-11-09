---
id: data-refresh
title: Data refresh
type: concept
summary: How page loads drive lazy syncs of the slate, weather, game lines, and nfelo projections, with per-source staleness windows, single-flight, failure backoff, tiered Parlay API and The Odds API credit floors, and an archive of every successful response.
tags: [refresh, sync, lazy, staleness, singleflight, backoff, credits, odds, parlay-api, the-odds-api, enrichment, fallback, props, weather, slate, live-window, nfelo, projections, archive, raw-responses]
sources: [internal/refresh/refresh.go, internal/refresh/rules.go, internal/refresh/sources.go, internal/refresh/research.go, internal/refresh/nfelo.go, internal/refresh/publish.go, internal/refresh/live.go, internal/db/sync.go, internal/db/nfelo.go, internal/ingest/nfelo/client.go, internal/ingest/archive/transport.go, cmd/server/main.go]
last_verified: 2026-11-09
---

# Data refresh

## Summary

Outside data comes in lazily. `internal/refresh` owns when. There is no cron, no Cloud Scheduler, and no background
ticker. A full-page `GET` calls `Kick()` from middleware; it returns at once and does the work in the background. Each
source has its own staleness window, its own single-flight, and its own failure backoff. Parlay API and The Odds API
are the metered sources, so each free event list runs before its balances gate the paid calls.

## What kicks a sync

`kickOnPage` wraps the app inside the sign-in and group middleware, with `CSRF` outermost. It calls `Kick()` only for a
full-page `GET`:

- not a non-`GET` request;
- not an HTMX partial (`HX-Request: true`);
- not an SSE stream (`Accept: text/event-stream`, or a path ending in `/events`).

Partials and streams skip it because the page load that mounted them already kicked, and an SSE reconnect must not spend
credits. Health, static files, and the sign-in screens sit outside the wrapper.

`Kick()` returns immediately. Its work detaches from the request with `context.WithoutCancel`, so a finished response
does not cancel a sync, and every source gets its own 30 second timeout. Tests wait for the background work through a
`WaitGroup`; the server never waits.

## One pass

1. Read the latest week and its games. Before the first slate sync there is no week, which is not an error.
2. Sync `espn:scoreboard` first, because it may add or change the week's games.
3. Read the week's games again, then start one weather source per game, one odds source for the week, and the nfelo
   projections source. Those run at the same time, each in its own goroutine.

Within a source, `singleflight` keyed by its `sync_state` source key lets concurrent page loads share one fetch. The
work inside the flight re-reads the staleness state before fetching, so a load that joined after the first run finished
does not fetch stale data twice.

## Archive

Each source labels its requests with `archive.WithKey(ctx, src.key)` in `refresh`, using the same string it writes to
`sync_state`. The shared ingest client in `cmd/server` wraps its transport with `archive.Transport`, so every successful
response is saved under that key before the client parses it. See [Response archive](archive.md).

The save sits beside the sync, not inside its transaction. A failed archive write is logged and dropped, so it never
fails a sync or changes the parsed rows. Parlay API's free `/events` list and paid `/odds` call both use `odds:lines`.
The Odds API's calls use `odds:theoddsapi`, so its rows archive under that key with `source = "odds"`.

## Staleness rules

Each source records its last successful sync in `sync_state.fetched_at`. A source that has never run is always stale.

| Source key         | Data                                    | Stale after                                                                                                             | Skipped when                                                       |
| ------------------ | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `espn:scoreboard`  | Latest week's slate, scores, and status | 10 min while any game is live; 30 min on Sunday or Monday; 6 h otherwise                                                 | Never                                                              |
| `weather:{gameID}` | Kickoff-window forecast for one game    | 1 h                                                                                                                      | Indoor, unknown venue, more than 48 h before kickoff, or kicked off |
| `odds:lines`       | The week's game lines                   | 24 h Monday–Friday; 12 h Saturday; 45 min Sunday before the last Sunday kickoff; 3 h after that and all day Monday      | No games, or every game has kicked off                             |
| `odds:props:{gameID}` | Game prop odds                       | 2 h                                                                                                                     | Once game has kicked off                                           |
| `espn:roster:{teamID}`| NFL team roster                      | 24 h                                                                                                                    | Never                                                              |
| `espn:summary:{gameID}`| Game summary and key injuries        | 2 h                                                                                                                     | Game status is FINAL                                               |
| `espn:gamelog:{athleteID}`| Athlete season game log and stats | 12 h                                                                                                                    | Never                                                              |
| `nfelo:projections`| The week's model spreads, win probabilities, and QB adjustments | 12 h                                                                                                     | Not a regular-season week, no games, or every game has kicked off  |

"Sunday" and "Monday" mean `America/New_York`. The zone database is embedded in the binary, so it resolves in the
container without a separate `tzdata` package. The scoreboard's live flag comes from the stored games: any game with
status `LIVE` shortens the window to 10 minutes.

`odds:theoddsapi` is not a scheduled source. The Odds API calls ride inside the `odds:lines` pass, and refresh records
their balance under that key so the next pass can enforce the fallback and enrichment floors.

Weather is per game, not per week. A game inside its window refreshes hourly until kickoff; an indoor game or one with
no coordinates in the venue map never gets a forecast. See [Data sources](../reference/data-sources.md) for the endpoint
and the venue map.

nfelo is per week, not per game. One fetch of the whole CSV covers every NFL week since 2021, and refresh keeps only the
rows that match the current week's games. A game with no matching row keeps its last projection, or has none. A
non-regular-season week is skipped because the CSV's playoff week numbers are unverified, and a week whose games have all
kicked off is skipped because the projections are pre-game numbers that no longer change. The source spends no Odds API
credits, so it has no floor and does not wait on the odds sync.

## Odds credits

Parlay API is primary and The Odds API enriches it or serves as fallback. The budgets are in
[Data sources](../reference/data-sources.md). Refresh enforces them in this order:

1. Call both free `/events` lists and match each provider to the week's games on its own. The matches are stored even
   when no lines will be bought.
2. Store the primary balance under `odds:lines` and The Odds API balance under `odds:theoddsapi`.
3. Below 50 Parlay API credits, skip its paid `/odds` call and log a warning. Cached rows keep serving. This is not a
   failure, so it does not trigger backoff.
4. The Odds API buys lines when Parlay API bought them and The Odds API has at least 100 credits. It also buys them
   when Parlay API is unconfigured, failed, or below its floor, provided The Odds API has at least 25. Otherwise it
   stops after the free event list.
5. Merge the rows. A primary row with no link adopts The Odds API's link, sid, and event sid; when the linked line
   differs, it also adopts the linked line and price. A linked primary row keeps its line, price, and link. Secondary
   rows that fill a missing book or market are appended.
6. Record the primary fetch and write the merged rows in one transaction. A fallback event ID fills
   `games.odds_event_id` only when the game has no stored ID, so a transient primary failure does not churn it.

The paid responses carry every upcoming event, not just the synced week. Refresh writes only lines for matched games
and drops the rest, so the `odds` table never takes a row for a game that does not exist locally. Team names arrive as
provider display names and are converted to the game's ESPN abbreviation before the write.

A missing or unreadable remaining count is a sync failure, not a guessed balance. `odds.ErrNoAPIKey` means a provider is
disabled, not failed. If one provider fails while the other succeeds, refresh logs a warning and commits the surviving
rows, so `odds:lines` does not enter backoff. If every configured provider fails, the sync returns the joined error and
the source backs off with cached rows still served.

Props and research data run on demand through `SyncGameProps`, `SyncGameResearch`, and `SyncPlayerResearch`.
Opening a game card or research partial triggers an on-demand sync with a short 10-second timeout:

- `odds:props:{gameID}`: Stale after 2 hours, skipped once kickoff has passed. Before matching players, it syncs
  rosters (`espn:roster:{teamID}`) for both teams. It enforces the Parlay API 150-credit floor (`OddsPropsFloor`). When
  The Odds API is configured and Parlay API bought props, it calls `EventProps` at or above 100 credits
  (`TheOddsAPIEnrichFloor`) and merges via `odds.MergeProps` to populate DraftKings prop deep links. If Parlay API is
  unconfigured, errors, returns no props, or sits below 150 credits, it falls back to The Odds API when at least 25
  credits remain (`TheOddsAPIFallbackFloor`). Prop players are matched against team rosters via `ResolvePlayerMatch`.
- `espn:roster:{teamID}`: Stale after 24 hours. Upserts team roster players into `roster_players` with normalized names.
- `espn:summary:{gameID}`: Stale after 2 hours, skipped when game status is `FINAL`. Stores raw `injuries_json` on
  `games`.
- `espn:gamelog:{athleteID}`: Stale after 12 hours. Upserts `player_gamelogs` with market stats. In addition to on-demand
  research lookups, `Kick()` scans slip legs in the current week and syncs stale game logs in the background.

## Failure and backoff

A failed fetch or write logs the error, keeps the old rows, leaves `fetched_at` unchanged, and pauses that source for 5
minutes. The pause lives in memory, keyed by source key, so one bad forecast does not pause the slate or the lines. A
successful run clears it. The server never records a failure in the database.

A source's row writes run inside one transaction, so a failure part way through leaves no partial rows. A sync that
succeeds with no row changes still records `fetched_at`, so a repeated response is not fetched again until the next
window.

## Publishing changed weeks

After a transaction commits and only when it changed rows, refresh publishes live topics:

- `week:{weekID}` for the game board;
- `slip:{id}` for every slip in that week.

The scoreboard, weather, odds, and nfelo syncs all use the same helper. Publishing after commit means a listener never
refetches a write that might roll back. A no-op rerun publishes nothing. See [Live updates](live-updates.md) for the hub
and the client.

## Live window

`Service.LiveWindow(weekID)` is the rule behind the SSE stream's longer deadline. A week is live when any game is
`LIVE`, or any `SCHEDULED` game kicks off within the next 30 minutes. A `FINAL` game is never live, even when its
kickoff was recent, and a scheduled game whose kickoff has passed is not either.

The check reads the week's games with a 5 second timeout. A read error returns false, so the stream falls back to the
idle deadline rather than staying open on a guess. The stream itself uses a 55 minute deadline inside the live window;
see [Live updates](live-updates.md).

## Guardrails

- Page loads are the only trigger. Adding a cron or scheduled job needs a plan change, because it spends free-tier
  credits and keeps the instance warm.
- Keep the work detached from the request and bounded by its own timeout.
- Check each provider's credit floor after its free event list and before its paid call. Never buy lines on a stale or
  missing balance.
- Publish once per committed change, never inside the transaction.
- Keep one single-flight per source. A fake clock moves the windows in tests; the real clock is only in the server.
