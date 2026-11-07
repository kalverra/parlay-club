---
id: data-sources
title: Data sources
type: reference
summary: The free sources behind the slate, the ESPN scoreboard endpoint and its Sunday and Monday filter, the Open-Meteo forecast behind a game's weather, and The Odds API game lines, bookmakers, and credit budget.
tags: [data, espn, scoreboard, games, sync, fixtures, weather, open-meteo, venues, odds, lines, bookmakers, credits, budget]
sources: [internal/ingest/espn/client.go, internal/ingest/espn/scoreboard.go, internal/db/weeks.go, internal/db/games.go, internal/ingest/weather/client.go, internal/ingest/weather/forecast.go, internal/ingest/weather/venues.go, internal/ingest/odds/client.go, internal/ingest/odds/events.go, internal/ingest/odds/lines.go, internal/ingest/odds/match.go]
last_verified: 2026-11-07
---

# Data sources

## Summary

Each source gets a client package under `internal/ingest/`. A client takes a base URL and an `*http.Client` in its
constructor, defaults to a 10 second timeout, sends `User-Agent: parlay-club`, returns plain structs, and never touches
the database. The caller decides when to fetch and writes results through the `Store`.

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
| `events[].competitions[0].competitors[].score`                   | `home_score` or `away_score`, only for a live or final game              |
| `events[].competitions[0].venue`                                 | `venue_id`, `venue_name`, `is_indoor` from `id`, `fullName`, `indoor`    |
| `events[].status.type.state`                                     | `pre` → `SCHEDULED`, `in` → `LIVE`, `post` → `FINAL`                     |

- Keep only events whose kickoff in `America/New_York` falls on Sunday or Monday. A 9:30 a.m. ET London kickoff stays
  on the slate, and a Monday night game stored in UTC on Tuesday stays too.
- A scheduled game has no score, even when ESPN sends a placeholder zero.
- An unknown event state is an error, not a guess.
- Upsert the week and each game; never delete them, so a flexed game keeps its ID. Rerunning a sync with the same
  response leaves every row, including `updated_at`, unchanged.

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

## The Odds API (free tier: 500 credits/month)

Base: `https://api.the-odds-api.com/v4/sports/americanfootball_nfl`. Every call carries the key as the `apiKey` query
param and sends `User-Agent: parlay-club`. `odds.NewClient(baseURL, apiKey, httpClient)` defaults an empty base URL to the
root above and a nil client to a 10 second timeout. An empty key is an error before any request goes out.

### Credits

Every response, the free event list included, carries `x-requests-remaining`, `x-requests-used`, and `x-requests-last`.
The client returns them as `Credits{Remaining, Used, Last}`. A missing or unreadable remaining count is an error, because
the caller cannot enforce a credit floor without it. The used and last counts are informational and default to zero.

### Game lines

`GET /odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american` costs about 3 credits and returns every book's
three game markets. `Client.Lines(ctx)` maps them into one `Line` per book and selection, shaped like an `odds` row:

| `Line` field | Source                                                           |
| ------------ | ---------------------------------------------------------------- |
| `EventID`    | `events[].id`                                                    |
| `Book`       | `bookmakers[].key`                                               |
| `MarketType` | `h2h` → `ML`, `spreads` → `SPREAD`, `totals` → `TOTAL`            |
| `Team`       | The outcome's name for a moneyline or spread; empty for a total  |
| `Side`       | `TEAM` for a moneyline or spread; `OVER` or `UNDER` for a total  |
| `Line`       | NULL for a moneyline; the point for a spread or total            |
| `Price`      | `price`, American                                                |

- A market key the client does not read produces no rows, so the same client can serve later prop requests.
- A book that omits a market produces no rows for it. Some books omit the moneyline on future games.
- A missing price, a spread or total with no point, an empty team name, or a non-Over/Under total outcome is an error,
  not a guess.
- `Team` stays The Odds API's display name; the caller maps it to an ESPN abbreviation.

### Event list and matching

`GET /events` is free and returns `id`, `home_team`, `away_team`, and `commence_time`. `Client.Events` parses
`commence_time` as RFC 3339 and stores it in UTC.

`MatchEvents` pairs each event with one game when `home_team` and `away_team` equal the game's ESPN `displayName`s and
`commence_time` is within 12 hours of `kickoff_utc`, in either direction. The result maps game ID to event ID. Each event
and each game is used at most once, so input order decides a rare tie. Unmatched events come back in input order for
logging.

### Bookmakers

`Book` keeps the API's bookmaker key. The saved 2026-11-07 `us` response carried `draftkings`, `fanduel`, `betmgm`,
`betrivers`, `betus`, `betonlineag`, `bovada`, `lowvig`, and `mybookieag`. Keep only keys a live response returns. A
member's home book with no row for a selection falls back to the median in [odds math](odds-math.md).

### Budget

Target at most 400 credits a month, leaving room for testing.

- `odds:lines` is stale after 24 h Monday through Friday, 12 h on Saturday, and 45 min on Sunday until the last Sunday
  kickoff. After that it runs only before the Monday game, every 3 h. Skip the fetch once every game of the week has
  kicked off.
- `odds:props:{game}` runs only when someone opens that game's quick-add, and only while at least 150 credits remain.
- `odds:lines` runs only while at least 50 credits remain.
- Below a floor, serve cached rows and show "lines paused until the monthly reset".
- The rough split: lines are about 20 calls × 3 = 60 credits a week, props about 6 games × 9 markets = 54 a week, so
  about 450 a month at the high end. Props stop first, at 150, which protects lines.
