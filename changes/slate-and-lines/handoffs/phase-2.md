# Phase 2: Weather

## Done

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

## Deviations

- The scoreboards do not carry latitude or longitude, and most venues have no usable address. The venue set comes from
  this season's scoreboards, but the coordinates were seeded manually and keyed by the ESPN venue IDs the scoreboard
  fixtures already surface. A fixture test enforces that the map covers exactly the season's venues and that names match
  ESPN's `fullName`.
- The venue fixture keeps ESPN's `indoor` flag, but the map does not store it. `games.is_indoor` already comes from the
  scoreboard, so the weather caller can use the game row instead of duplicating the flag in `Venue`.
- Moving the source section dropped two planning bullets: weather is only for outdoor venues, and Open-Meteo forecasts
  reach about 16 days so further-out games are skipped. Neither is enforced in the client; they are refresh-policy rules
  and belong in the phase 4 refresh docs and code.
- The forecast parser is strict in the same way as the scoreboard parser: missing hours, nulls, and misaligned series
  are errors, not defaults. That matches the docs' "no guesses" rule.

## Open issues

- There is no store write for weather yet. `UpsertGame` deliberately leaves `temp_f`, `wind_mph`, `wind_gust_mph`, and
  `precip_in` alone, so refresh needs a store method such as `UpsertGameWeather` before it can persist a forecast.
- Nothing writes `sync_state` for `weather:{game}` yet. Staleness, the 48-hour lookahead, the skip after kickoff, and
  the indoor skip are all phase 4 work.
- `Forecast` will happily fetch for an indoor venue or a game beyond Open-Meteo's horizon. The caller must skip both
  cases, and no code does that today.
- The venue map is a 2026 snapshot. A new stadium, a renamed stadium, or a future season needs a map edit; the fixture
  test compares the map to `venues_2026.json` exactly, so a new fixture also needs the map updated. An unknown ID is
  safe: it returns `false` and the game keeps NULL weather.
- Only the regular season is covered by the venue fixture. Preseason and postseason venues are not checked.

## Next-phase notes

- Phase 3 (game lines) does not depend on weather. Weather work resumes in phase 4 (refresh).
- In refresh, resolve a game's venue with `weather.VenueByID(game.VenueID)`, skip when `ok` is false or `game.IsIndoor`
  is true, skip kickoffs more than about 16 days out, and skip games that have already kicked off per the staleness
  rules.
- The fixture requests were captured with the exact query the client sends, including `start_hour` and `end_hour`. If the
  query changes, capture new fixtures instead of hand-editing the old ones.
- `weather.NewClient(baseURL, httpClient)` matches the ESPN client shape. Keep the base-URL plus `*http.Client`
  constructor when adding the odds client.
- A forecast is three hourly values, not the whole day. The temperature is the kickoff hour only; do not reuse the
  window max for it.

## Files touched

- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `internal/ingest/weather/client.go`
- `internal/ingest/weather/forecast.go`
- `internal/ingest/weather/forecast_test.go`
- `internal/ingest/weather/testdata/README.md`
- `internal/ingest/weather/testdata/forecast_highmark.json`
- `internal/ingest/weather/testdata/forecast_lumen.json`
- `internal/ingest/weather/testdata/forecast_short_window.json`
- `internal/ingest/weather/testdata/venues_2026.json`
- `internal/ingest/weather/venues.go`
- `internal/ingest/weather/venues_test.go`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
