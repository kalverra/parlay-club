---
id: data-sources
title: Data sources
type: reference
summary: The free sources behind the slate, the ESPN scoreboard endpoint and its Sunday and Monday filter, the Open-Meteo forecast behind a game's weather, Parlay API game lines and props with The Odds API enrichment and fallback, ESPN rosters, game logs, and summary injuries, player matching, nfelo model projections, bookmakers, deep links, tiered credit floors, and the raw response archive every successful call leaves.
tags: [data, espn, scoreboard, games, sync, fixtures, weather, open-meteo, venues, odds, parlay-api, the-odds-api, props, lines, bookmakers, links, credits, budget, enrichment, fallback, nfelo, projections, spreads, quarterbacks, archive, raw-responses, sqlite, rosters, gamelogs, injuries, player-matching]
sources: [internal/ingest/espn/client.go, internal/ingest/espn/scoreboard.go, internal/ingest/espn/roster.go, internal/ingest/espn/summary.go, internal/ingest/espn/gamelog.go, internal/db/weeks.go, internal/db/games.go, internal/db/players.go, internal/ingest/weather/client.go, internal/ingest/weather/forecast.go, internal/ingest/weather/venues.go, internal/ingest/odds/client.go, internal/ingest/odds/events.go, internal/ingest/odds/lines.go, internal/ingest/odds/props.go, internal/ingest/odds/match.go, internal/ingest/nfelo/client.go, internal/ingest/nfelo/projections.go, internal/ingest/nfelo/teams.go, internal/db/nfelo.go, internal/refresh/nfelo.go, internal/refresh/research.go, internal/ingest/archive/transport.go, internal/ingest/archive/labels.go, internal/db/archive.go, internal/refresh/refresh.go, internal/refresh/sources.go, cmd/server/main.go]
last_verified: 2026-11-12
---

# Data sources

## Summary

Each source gets a client package under `internal/ingest/`. A client takes a base URL and an `*http.Client` in its
constructor, defaults to a 10 second timeout, sends `User-Agent: parlay-club`, returns plain structs, and never touches
the database. The caller decides when to fetch and writes results through the `Store`.

## Response archive

Every successful call to these sources is saved raw before the client parses it. The ingest `*http.Client` built in
`cmd/server` has `archive.Transport` as its transport, so the clients stay database-free and cannot forget a call.

- A 2xx response of at most 8 MiB is kept. The uncompressed bytes go in `api_bodies`, gzipped and keyed by their
  SHA-256, so an unchanged payload stores one body row. Each call adds one `api_responses` row with the source, key,
  method, redacted URL, status, time, duration, and body hash.
- Non-2xx responses, transport errors, timeouts, and bodies over the cap pass through unrecorded. A sink error is logged
  and dropped, so the client still gets its response.
- The stored URL keeps every query parameter except the odds APIs' `apiKey`, which becomes `REDACTED`. Odds responses
  also fill `credits_remaining` from `x-requests-remaining` and `credits_used` from `x-requests-last`.
- The `key` is the `sync_state` key that `refresh` puts in the request context. If a client forgets the label, the
  transport falls back to `host + path`. Parlay API calls use `odds:lines`; The Odds API calls use `odds:theoddsapi`.
  Both hosts map to `source = "odds"`.

The archive is research data. No page or route reads it. See [Response archive](../concepts/archive.md) for the tables,
the read API, and the size check.

## ESPN (free, no auth, unofficial)

### Scoreboard

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard`

With no params it returns the current week. For a specific week: `?seasontype={2|3}&week={n}&dates={season}`.

| Field                                                            | Use                                                                      |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `season.year`, `season.type`, `week.number`                      | Week row                                                                 |
| `events[].id`                                                    | `games.id`                                                               |
| `events[].date`                                                  | `kickoff_utc`                                                            |
| `events[].competitions[0].competitors[].homeAway`                | Which side the competitor is (`home` or `away`)                          |
| `events[].competitions[0].competitors[].team.abbreviation`        | `home_team` or `away_team`                                               |
| `events[].competitions[0].competitors[].team.displayName`        | `home_name` or `away_name`                                               |
| `events[].competitions[0].competitors[].team.id`                 | `home_espn_id` or `away_espn_id`                                         |
| `events[].competitions[0].competitors[].team.logo`               | `home_logo` or `away_logo` (`https://` URLs only; empty when missing)    |
| `events[].competitions[0].competitors[].score`                   | `home_score` or `away_score`, only for a live or final game              |
| `events[].competitions[0].venue`                                 | `venue_id`, `venue_name`, `is_indoor` from `id`, `fullName`, `indoor`    |
| `events[].status.type.state`                                     | `pre` → `SCHEDULED`, `in` → `LIVE`, `post` → `FINAL`                     |

- Keep only events whose kickoff in `America/New_York` falls on Sunday or Monday. A 9:30 a.m. ET London kickoff stays on
  the slate, and a Monday night game stored in UTC on Tuesday stays too.
- A scheduled game has no score, even when ESPN sends a placeholder zero.
- An unknown event state is an error, not a guess.
- Upsert the week and each game; never delete them, so a flexed game keeps its ID. Rerunning a sync with the same
  response leaves every row, including `updated_at`, unchanged.

### Summary (injuries)

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/summary?event={id}`

- `SummaryInjuries(ctx, eventID)` returns the raw `injuries` JSON array or `"[]"` when absent.
- `ParseInjuries(rawJSON)` parses per-team injury lists filtered to key positions (`QB`, `RB`, `WR`, `TE`, `LT`, `C`, `CB`, `S`, plus `OT`). Non-key positions are omitted.
- Each team group includes `LogoURL` (from `team.logo`). Injuries include `AthleteID`, `FullName`, `Position`,
  `Status`, `HeadshotURL` (from `athlete.headshot.href`), and a formatted `Detail` string.

### Roster

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/{espnTeamID}/roster`

- `Roster(ctx, espnTeamID)` flattens position groups into `[]RosterPlayer{ID, FullName, Position, HeadshotURL}` (with
  `HeadshotURL` from `headshot.href`), skipping entries without an athlete ID or full name.
- All ESPN logo and headshot URLs are trimmed and accepted only when they use `https://` with a non-empty host
  (returning `""` otherwise).

### Game log

`GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/{athleteID}/gamelog`

- `GameLog(ctx, athleteID)` fetches game log data through `webBaseURL` (defaulting to `https://site.web.api.espn.com/apis/common/v3/sports/football/nfl`).
- The parser matches header names dynamically (`names` and category-scoped `labels`, never positional indices).
- Walks regular-season and postseason event rows for the current season, most recent first.
- Counts `GamesPlayed` and populates `Markets map[string][]float64` for `PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD` (rushing + receiving + return TDs), and `KICKER_PTS` (`FG made * 3 + XP made`).

## Open-Meteo (free, no auth)

```text
GET https://api.open-meteo.com/v1/forecast?latitude={lat}&longitude={lon}
    &hourly=temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m
    &wind_speed_unit=mph&temperature_unit=fahrenheit&precipitation_unit=inch
    &timezone=UTC&start_hour={kickoff hour}&end_hour={kickoff hour + 2h}
```

- The kickoff window is the hour containing kickoff and the two after it, three hourly values. `start_hour` is kickoff
  in UTC rounded down to the hour, and `end_hour` is two hours later.
- `temp_f` is the temperature at the kickoff hour. `wind_mph` and `wind_gust_mph` are the window's maximum sustained
  wind and maximum gust. `precip_in` is the window's maximum hourly precipitation.
- Every hour in the window must be present and set. A missing hour, a null value, or a window the source does not cover
  is an error, not a zero.
- `Client.Forecast(ctx, venue, kickoff)` returns the four values above as a `weather.Forecast`.

### Venue coordinates

`internal/ingest/weather/venues.go` maps an ESPN venue ID to its name, latitude, and longitude. Coordinates are for the
stadium itself, not the city center. The map covers every venue on the 2026 regular-season scoreboards, weeks 1 through
18, including the international games, because the scoreboard response carries no coordinates.

`VenueByID` returns the venue and `true`. An unknown ID returns `false` and an empty venue, so the caller can log it and
leave that game's weather NULL. Add a venue when a new stadium appears.

## Parlay API (free tier: 1,000 credits/month)

Base: `https://parlay-api.com/v1/sports/americanfootball_nfl`. It is an Odds API-compatible feed. Every call carries
the key as the `apiKey` query param and sends `User-Agent: parlay-club`. `odds.NewClient(baseURL, apiKey, httpClient)`
defaults an empty base URL to the root above and a nil client to a 10 second timeout. An empty key is an error before
any request goes out.

### Credits

Every response, the free event list included, carries `x-requests-remaining`, `x-requests-used`, and `x-requests-last`.
The client returns them as `Credits{Remaining, Used, Last}`. A missing or unreadable remaining count is an error,
because the caller cannot enforce a credit floor without it. The used and last counts are informational and default to
zero.

### Game lines

`GET /odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american&includeLinks=true&includeSids=true` costs 3 credits
and returns every book's three game markets. `Client.Lines(ctx)` maps them into one `Line` per book and selection,
shaped like an `odds` row:

| `Line` field | Source                                                           |
| ------------ | ---------------------------------------------------------------- |
| `EventID`    | `events[].id`                                                    |
| `Book`       | `bookmakers[].key`                                               |
| `MarketType` | `h2h` → `ML`, `spreads` → `SPREAD`, `totals` → `TOTAL`            |
| `Team`       | The outcome's name for a moneyline or spread; empty for a total  |
| `Side`       | `TEAM` for a moneyline or spread; `OVER` or `UNDER` for a total  |
| `Line`       | NULL for a moneyline; the point for a spread or total            |
| `Price`      | `price`, American                                                |
| `Link`       | The outcome's `link`, the book's deep link; empty when absent    |
| `Sid`        | The outcome's `sid`, the book's selection ID; empty when absent  |
| `EventSid`   | `bookmakers[].sid`, the book's own event ID; empty when absent   |

- The two deep-link flags cost no extra credits. A 2026-11-07 probe of The Odds API measured `x-requests-last` at 3 with
  and without them for the three game markets, and at 1 with and without them for one prop market. The outcome fields
  land on each outcome, not on the market or book; `event_sid` comes from the bookmaker block. A book may omit either,
  so a missing link is normal and the row stores NULL.
- On Parlay API, only FanDuel outcomes currently carry a validated `link` and `sid`, and only while a matching native
  quote is open. DraftKings and other books omit them. The Odds API enrichment fills the DraftKings links and any
  missing FanDuel link in the same sync; see [Enrichment and fallback](#enrichment-and-fallback). A week whose
  enrichment has paused keeps the primary rows, and an unlinked leg renders as a manual `no link` entry in export.
- A market key the client does not read produces no rows, so the same client can serve prop requests.
- A book that omits a market produces no rows for it. Some books omit the moneyline on future games.
- A `regions=us` response includes exchanges and DFS books that may quote only one side of a two-way market. An outcome
  with a null or missing `price`, or a spread or total with no `point`, is skipped rather than failing the whole sync. A
  moneyline outcome whose name is neither team, such as a three-way feed's `Draw`, is skipped too.
- `Team` stays the provider's display name; the caller maps it to an ESPN abbreviation.

### Event list and matching

`GET /events` is free and returns `id`, `home_team`, `away_team`, and `commence_time`. Parlay API can send
`commence_time: null` on a fixture whose kickoff is not yet reported; `Client.Events` skips that event instead of
failing the list. It parses a valid `commence_time` as RFC 3339 and stores it in UTC.

`MatchEvents` pairs each event with one game when `home_team` and `away_team` equal the game's ESPN `displayName`s and
`commence_time` is within 12 hours of `kickoff_utc`, in either direction. The result maps game ID to event ID. Each
event and each game is used at most once, so input order decides a rare tie. Unmatched events come back in input order
for logging.

### Bookmakers

`Book` keeps the API's bookmaker key. The saved 2026-11-07 `us` response carried `draftkings`, `fanduel`, `betmgm`,
`betrivers`, `betus`, `betonlineag`, `bovada`, `lowvig`, and `mybookieag`. Parlay API's `regions=us` also includes
exchanges and DFS books such as `prizepicks`, `pick6`, `kalshi`, and `polymarket`; their unpriced or one-sided quotes
are skipped. Keep only keys a live response returns. A member's home book with no row for a selection falls back to the
median in [odds math](odds-math.md).

### Bulk props

```text
GET /props?eventId={id}
    &markets=team_totals,player_pass_yds,player_pass_tds,player_rush_yds,player_rush_attempts,
        player_reception_yds,player_receptions,player_anytime_td,player_kicking_points
    &oddsFormat=american&includeLinks=true&includeSids=true&limit=10000
```

`/props` costs a flat 3 credits per call, whatever the number of markets or bookmakers returned. The default page is
500 rows; `limit=10000` fits a whole game's board in one call. Unlike `/odds`, the response is a flat JSON array of
quote objects, one row per book, player, market, and period:

| Field                                                    | Use                                                            |
| -------------------------------------------------------- | -------------------------------------------------------------- |
| `event_id`                                               | Event ID; a row for another event is skipped                   |
| `bookmaker`                                              | `Prop.Book`                                                    |
| `player`, `player_name`                                  | The subject, with `player_name` as a legacy alias              |
| `market_key`                                             | Canonical or alias key, mapped to a market type (table below)  |
| `period`                                                 | Keep empty or `FULL`; skip `1H`, `Q1`, and `UNKNOWN`           |
| `line`                                                   | `Prop.Line`; NULL for an anytime touchdown                     |
| `over_price`, `under_price`                              | `Prop.Price` for the `OVER` and `UNDER` rows                   |
| `over_link`, `under_link`                                | `Prop.Link` for each side; empty when absent                   |
| `over_sid`, `under_sid`                                  | `Prop.Sid` for each side; empty when absent                    |

`Client.Props(ctx, eventID)` returns one `Prop` per priced side. A priced `over_price` or `under_price` becomes an
`OVER` or `UNDER` row. An anytime touchdown becomes one `YES` row with a NULL line, even when the wire sends `0.5`.
`team_totals` sets `Team` to the full team name; every other market sets `PlayerName`. A row with no line, no price, or
no subject or book is skipped, so unpriced DFS rows never reach `refresh`.

| Parlay API key                   | Aliases                                                          | Market       |
| -------------------------------- | ---------------------------------------------------------------- | ------------ |
| `team_totals`                    |                                                                  | `TEAM_TOTAL` |
| `player_pass_yds`                | `player_passing_yards`, `player_pass_yards`                      | `PASS_YDS`   |
| `player_pass_tds`                | `player_passing_tds`, `player_passing_touchdowns`                | `PASS_TDS`   |
| `player_rush_yds`                | `player_rushing_yards`, `player_rush_yards`                      | `RUSH_YDS`   |
| `player_rush_attempts`           | `player_rushing_attempts`                                        | `RUSH_ATT`   |
| `player_reception_yds`           | `player_receiving_yds`, `player_receiving_yards`                  | `REC_YDS`    |
| `player_receptions`              |                                                                  | `RECEPTIONS` |
| `player_anytime_td`              | `player_anytime_touchdown_scorer`, `anytime_td`, `anytime_touchdown_scorer` | `ANYTIME_TD` |
| `player_kicking_points`          |                                                                  | `KICKER_PTS` |

Parlay API rows carry validated prop links and sids on FanDuel only today. `odds:props:{game}` runs on demand
when someone opens a game card or research view, stale after 2 hours and skipped after kickoff. Before matching
players, it syncs rosters for both teams (`espn:roster:{team}`). It checks the Parlay API 150-credit floor
(`OddsPropsFloor`), calls `Client.Props`, maps `TEAM_TOTAL` to the game's team abbreviation, and resolves player
names through `ResolvePlayerMatch`.

## The Odds API (free tier: 500 credits/month)

Base: `https://api.the-odds-api.com/v4/sports/americanfootball_nfl`. It speaks the same Odds API wire format as Parlay
API. `odds.NewClient(odds.TheOddsAPIBaseURL, cfg.TheOddsAPIKey, httpClient)` builds it, and `refresh` wires it as
`FallbackLines`. An empty `THE_ODDS_API_KEY` disables the provider.

### Credits and sync state

Every response carries the same `x-requests-remaining`, `x-requests-used`, and `x-requests-last` headers. Refresh stores
the remaining balance in `sync_state` under `odds:theoddsapi` (`refresh.TheOddsAPISource`). The `odds:` prefix makes the
archive record these responses as `source = "odds"` and fill their credit columns. The primary balance stays under
`odds:lines`.

### Game lines

```text
GET /odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american&includeLinks=true&includeSids=true
```

The call costs 3 credits. The Odds API bills by `markets × regions`, not by bookmaker count, so asking for all of
`regions=us` costs the same 3 credits as filtering to one book. Its rows map through the same `Line` table above.
DraftKings and FanDuel outcomes carry `link` and `sid`, and each bookmaker block carries its own event `sid`, so a
DraftKings combined export link has both IDs it needs.

### Enrichment and fallback

Refresh matches each provider's free `/events` list to the week's games on its own, maps its rows to ESPN game IDs and
team abbreviations, then merges the rows. Provider event IDs never need to agree.

- **Enrichment.** When Parlay API buys the week's lines and The Odds API has at least 100 credits, the sync also buys
  The Odds API lines. A primary row with no link adopts The Odds API's `link`, `sid`, and `event_sid`. If the linked
  line differs, the row also adopts The Odds API's line and price, so the stored line matches the deep link. A primary
  row that already has a link keeps its line, price, and link; a missing sid backfills only when the lines match, and a
  missing event sid always backfills. Selections only The Odds API quotes are appended, filling missing books and
  markets.
- **Fallback.** When Parlay API is unconfigured, errors, or sits below its 50-credit floor, The Odds API serves the
  whole week if it has at least 25 credits. Its event IDs fill `games.odds_event_id` only for games that have no stored
  ID, so a transient primary failure does not churn an existing ID.
- **Failures.** `odds.ErrNoAPIKey` means the provider is disabled, not failed. If one provider fails while the other
  succeeds, refresh logs a warning and commits the surviving rows, so the source does not back off. If every
  configured provider fails, the sync returns the joined error and refresh backs off for 5 minutes with cached rows
  still served.

### Per-event props

```text
GET /events/{id}/odds?regions=us&markets={prop markets}
    &oddsFormat=american&includeLinks=true&includeSids=true
```

The call costs 1 credit per market returned, unlike Parlay API's flat 3-credit `/props`. It returns one event object
whose `bookmakers[].markets[].outcomes[]` carry the prices. `Client.EventProps` maps them into the same `Prop` rows and
copies `bookmakers[].sid` into `Prop.EventSid`.

In `odds:props:{game}`, `odds.MergeProps` merges a Parlay API board with a per-event board from The Odds API:

- **Enrichment.** When Parlay API bought props and The Odds API has at least 100 credits (`TheOddsAPIEnrichFloor`),
  refresh fetches The Odds API props too. Unlinked primary rows gain DraftKings `link`, `sid`, and `event_sid` (and adopt
  the linked line and price if they differ). Secondary-only rows append missing books and markets.
- **Fallback.** When Parlay API is unconfigured, errors, returns no props, or sits below 150 credits, The Odds API
  serves the game's props if it has at least 25 credits (`TheOddsAPIFallbackFloor`).

## nfelo (free, no auth)

Base: `https://raw.githubusercontent.com/greerreNFL/nfelo/main/output_data`. The data comes from a public GitHub repo and
needs no key or account. `nfelo.NewClient(baseURL, httpClient)` defaults an empty base URL to the root above and a nil
client to a 10 second timeout. `Client.Projections(ctx)` fetches the one file the app uses:

```text
GET /historic_projected_spreads.csv
```

It holds about 1,550 rows from 2021 through the current week. The client reads every column by header name, never by
position, and ignores the nameless pandas index in the first column. A missing required header is an error, a row with an
empty required cell is dropped, and a non-numeric number is an error.

| Field                          | Column                                          | Use                                                                |
| ------------------------------ | ----------------------------------------------- | ------------------------------------------------------------------ |
| `Season`, `Week`               | `season`, `week`                                | Match to a `weeks` row                                             |
| `Home`, `Away`                 | `home_team`, `away_team`                        | Match to `games.home_team` and `games.away_team`, then map to ESPN |
| `HomeSpread`                   | `home_line_pre_regression`                      | Model spread from the home side; negative favors the home team     |
| `HomeWinProb`                  | `home_probability_nfelo`                        | Home win probability, 0 to 1                                       |
| `HomeQB`, `AwayQB`             | `home_538_qb`, `away_538_qb`                    | The starting quarterback the model used                            |
| `HomeQBAdj`, `AwayQBAdj`       | `home_538_qb_adj`, `away_538_qb_adj`            | Quarterback adjustment in Elo points                               |

- The spread is from the home side: a negative number means the home team is favored. `home_line_close` is the market
  line, and `home_closing_line_rounded_nfelo` echoes it on most current rows, so neither is the model number.
  `home_line_pre_regression` is the pure pre-regression rating.
- Team codes that differ from ESPN's are mapped: `OAK` → `LV` and `WAS` → `WSH`. Every other code already matches. An
  unmapped code passes through unchanged, so the game gets no row and refresh logs the unmatched CSV row at debug.
- The file has no played flag. A played game keeps its last pre-game projection.

Refresh writes the current regular-season week's matching rows into `nfelo_games`; see
[Data refresh](../concepts/data-refresh.md). The CSV is archived under the `nfelo:projections` key with
`source = "nfelo"`, so a format change leaves the raw bytes in the archive. The real file saved on 2026-11-09 is
`internal/ingest/nfelo/testdata/historic_projected_spreads.csv`.

## Player matching

Sportsbooks format player names differently from ESPN and from each other. Parlay API also omits the player's team on
prop quotes. Player matching normalizes names and matches each prop to an ESPN athlete ID and team:

- **Normalization (`NormalizePlayerName`):** lowercases, strips `.`, `'`, `-`, and `,`, collapses spaces, and drops a
  trailing generational suffix (`jr`, `sr`, `ii`, `iii`, `iv`, `v`) when preceded by a space.
- **Resolution (`ResolvePlayerMatch`):**
  1. Normalizes the sportsbook name. If empty, returns unmatched.
  2. Checks `player_matches` for `(norm, homeTeam)` and `(norm, awayTeam)`. If a `manual = 1` row exists, it wins
     immediately. If exactly one unambiguous match exists, returns `(espn_athlete_id, team)`. If conflicting matches
     exist for both teams, returns unmatched.
  3. On a miss, queries `roster_players` for `team IN (homeTeam, awayTeam) AND norm_name = norm`.
  4. If exactly one athlete matches across both teams, upserts `player_matches(odds_name, team, espn_athlete_id, manual=0)`
     and returns `(athlete_id, team)`. If zero or multiple athletes match, leaves `player_id` and `team` empty.
- **Manual overrides (`SaveManualPlayerMatch`):** writes `manual = 1` in `player_matches`. Auto-matching never
  overwrites a manual match. A manual save clears conflicting auto-matches for the opponent in that matchup, and
  backfills `team` on matching `odds` rows and `team` and `player_id` on matching `legs` rows.

## Budget

Two free tiers fund the odds data. Refresh checks each balance after that provider's free event list and before its paid
call. Both balances live in `sync_state`: `odds:lines` for Parlay API and `odds:theoddsapi` for The Odds API.

### Parlay API

The free tier is 1,000 credits a month. Target at most 800, leaving room for testing and playoff weeks.

- `odds:lines` is stale after 24 h Monday through Friday, 12 h on Saturday, and 45 min on Sunday until the last Sunday
  kickoff. After that it runs only before the Monday game, every 3 h. Skip the fetch once every game of the week has
  kicked off.
- `odds:props:{game}` runs on demand when someone opens that game's card or research view, and only while at least 150
  credits remain (`refresh.OddsPropsFloor`). Each game is one flat 3-credit `/props` call for all nine markets, not one
  credit per market.
- `odds:lines` runs only while at least 50 credits remain (`refresh.OddsLinesFloor`).
- Below a floor, serve cached rows and show "lines paused until the monthly reset".
- The rough split: lines are about 20 calls × 3 = 60 credits a week, and props about 6 games × 3 = 18 a week, or about
  72 a month. That is about 312 credits a month at the high end. Props stop first, at 150, which protects lines.

### The Odds API

The free tier is 500 credits a month. Its `/odds` call costs 3 credits, the same as Parlay API's.

- Enrichment runs only while at least 100 credits remain (`refresh.TheOddsAPIEnrichFloor`) and Parlay API bought the
  week's lines. Below that, the free `/events` call still refreshes the balance and matches, but no paid lines are
  bought.
- Fallback runs only while at least 25 credits remain (`refresh.TheOddsAPIFallbackFloor`) when Parlay API is
  unconfigured, errors, or sits below its floor.
- Routine enrichment is about 20 line calls a week × 3 = 60 credits, or about 240 a month. Enrichment pauses at 100,
  so it never spends the reserve kept for fallback.
- Per-event props cost 1 credit per market returned, so a nine-market call costs 9. In `odds:props:{game}`, enrichment
  runs at or above 100 credits (`refresh.TheOddsAPIEnrichFloor`) and fallback runs at or above 25 credits
  (`refresh.TheOddsAPIFallbackFloor`).
