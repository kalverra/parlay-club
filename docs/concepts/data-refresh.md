---
id: data-refresh
title: Data refresh
type: concept
summary: How page loads drive lazy syncs of the slate, weather, and game lines, with per-source staleness windows, single-flight, failure backoff, Odds API credit floors, and an archive of every successful response.
tags: [refresh, sync, lazy, staleness, singleflight, backoff, credits, odds, weather, slate, live-window, archive, raw-responses]
sources: [internal/refresh/refresh.go, internal/refresh/rules.go, internal/refresh/sources.go, internal/refresh/publish.go, internal/refresh/live.go, internal/db/sync.go, internal/ingest/archive/transport.go, cmd/server/main.go]
last_verified: 2026-11-07
---

# Data refresh

## Summary

Outside data comes in lazily. `internal/refresh` owns when. There is no cron, no Cloud Scheduler, and no background
ticker. A full-page `GET` calls `Kick()` from middleware; it returns at once and does the work in the background. Each
source has its own staleness window, its own single-flight, and its own failure backoff. The Odds API is the only
metered source, so a free balance check guards the paid call.

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
3. Read the week's games again, then start one weather source per game and one odds source for the week. Those run at
   the same time, each in its own goroutine.

Within a source, `singleflight` keyed by its `sync_state` source key lets concurrent page loads share one fetch. The
work inside the flight re-reads the staleness state before fetching, so a load that joined after the first run finished
does not fetch stale data twice.

## Archive

Each source labels its requests with `archive.WithKey(ctx, src.key)` in `refresh`, using the same string it writes to
`sync_state`. The shared ingest client in `cmd/server` wraps its transport with `archive.Transport`, so every successful
response is saved under that key before the client parses it. See [Response archive](archive.md).

The save sits beside the sync, not inside its transaction. A failed archive write is logged and dropped, so it never
fails a sync or changes the parsed rows. Both Odds calls in one pass, the free `/events` list and the paid `/odds` call,
use `odds:lines` today, so both rows carry that key.

## Staleness rules

Each source records its last successful sync in `sync_state.fetched_at`. A source that has never run is always stale.

| Source key         | Data                                    | Stale after                                                                                                             | Skipped when                                                       |
| ------------------ | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `espn:scoreboard`  | Latest week's slate, scores, and status | 10 min while any game is live; 30 min on Sunday or Monday; 6 h otherwise                                                 | Never                                                              |
| `weather:{gameID}` | Kickoff-window forecast for one game    | 1 h                                                                                                                      | Indoor, unknown venue, more than 48 h before kickoff, or kicked off |
| `odds:lines`       | The week's game lines                   | 24 h Monday–Friday; 12 h Saturday; 45 min Sunday before the last Sunday kickoff; 3 h after that and all day Monday      | No games, or every game has kicked off                             |

"Sunday" and "Monday" mean `America/New_York`. The zone database is embedded in the binary, so it resolves in the
container without a separate `tzdata` package. The scoreboard's live flag comes from the stored games: any game with
status `LIVE` shortens the window to 10 minutes.

Weather is per game, not per week. A game inside its window refreshes hourly until kickoff; an indoor game or one with
no coordinates in the venue map never gets a forecast. See [Data sources](../reference/data-sources.md) for the endpoint
and the venue map.

## Odds credits

The Odds API budget is in [Data sources](../reference/data-sources.md). Refresh enforces it in this order:

1. Call the free `/events` list and match it to the week's games.
2. Store the reported balance in `sync_state.odds_credits_remaining`, and store the matches even when no lines will be
   bought.
3. Below 50 credits, skip the paid `/odds` call and log a warning. Cached rows keep serving. This is not a failure, so
   it does not trigger backoff.
4. At or above 50, call `/odds` once for the whole week and store the new balance.

The paid response carries every upcoming event, not just the synced week. Refresh writes only lines for matched games
and drops the rest, so the `odds` table never takes a row for a game that does not exist locally. Team names arrive as
The Odds API display names and are converted to the game's ESPN abbreviation before the write.

A missing or unreadable remaining count is a sync failure, not a guessed balance. Props get their own source and a 150
credit floor later; only game lines exist today.

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

The scoreboard, weather, and odds syncs all use the same helper. Publishing after commit means a listener never
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
- Check the credit floor after the free event list and before the paid call. Never buy lines on a stale or missing
  balance.
- Publish once per committed change, never inside the transaction.
- Keep one single-flight per source. A fake clock moves the windows in tests; the real clock is only in the server.
