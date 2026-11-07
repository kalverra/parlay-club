---
id: data-sources
title: Data sources
type: reference
summary: The free sources behind the slate, the ESPN scoreboard endpoint and its Sunday and Monday filter, and the Open-Meteo forecast behind a game's weather.
tags: [data, espn, scoreboard, games, sync, fixtures, weather, open-meteo, venues]
sources: [internal/ingest/espn/client.go, internal/ingest/espn/scoreboard.go, internal/db/weeks.go, internal/db/games.go, internal/ingest/weather/client.go, internal/ingest/weather/forecast.go, internal/ingest/weather/venues.go]
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
