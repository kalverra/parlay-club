---
id: sandbox-replay
title: Sandbox replay
type: reference
summary: The fail-closed HTTP transport and named week 9 response registry behind the local Sunday sandbox, the requests they answer, and which fixtures are real captures versus derived snapshots.
tags: [sandbox, replay, fixtures, testing, espn, weather, parlay-api, nfelo, provenance, offline]
sources: [internal/sandbox/registry.go, internal/sandbox/transport.go, internal/sandbox/sandbox_test.go, internal/ingest/espn/testdata/README.md, internal/ingest/odds/testdata/README.md, internal/ingest/weather/testdata/README.md, internal/ingest/nfelo/testdata/README.md]
last_verified: 2026-11-10
---

# Sandbox replay

`internal/sandbox` replays saved provider responses through the ordinary ESPN, Open-Meteo, Parlay API, and nfelo
clients. It has two parts: an `http.RoundTripper` that answers only registered requests, and a registry that preloads
the week 9 response files.

## Fail-closed transport

`NewTransport(registry, stage)` returns an `http.RoundTripper`. The ingest clients keep their normal constructors; only
their `*http.Client` changes.

- A request matches on method, host, path, stage, and normalized query. Query parameters are sorted, and credential
  parameters (`apiKey`, `token`, `access_token`, in any capitalization or `-`/`_` spelling) are ignored. The synthetic
  key passed to `odds.NewClient` therefore never has to match the key in a capture.
- The transport has no base transport and no network fallback. An unregistered request returns
  `sandbox.ErrNoResponse` with the method, credential-free URL, and stage. Missing files, duplicate requests, and
  incomplete fixtures fail `NewRegistry` before startup.
- Responses carry the saved status, headers, and body bytes. Each call gets a fresh body reader, so the transport is
  safe for the concurrent source goroutines.
- Nothing here touches the response archive. The sandbox client's transport is the replay transport, not an
  `archive.Transport` wrapper, so replayed calls never enter `api_responses`.

`SetStage` switches the snapshot set for later requests. Only the `morning` stage is registered today.

## Week 9 registry

`sandbox.Week9(fsys)` preloads every fixture from an `fs.FS` rooted at the repository, so the registry references the
saved source files instead of copying large bodies. The morning set is the full scheduled slate, game lines, nfelo
projections, every outdoor forecast, and the featured game's lazy prop board.

| Fixture | Request | File | Provenance |
| --- | --- | --- | --- |
| `espn-week-9-scoreboard` | `GET site.api.espn.com/.../scoreboard` | `internal/ingest/espn/testdata/scoreboard_week.json` | Captured 2026-11-07. |
| `parlay-week-9-events` | `GET parlay-api.com/.../events` | `internal/ingest/odds/testdata/events_nfl.json` | Captured from The Odds API 2026-11-07; Parlay API serves the same shape. |
| `parlay-week-9-lines` | `GET parlay-api.com/.../odds?regions=us&markets=h2h,spreads,totals...` | `internal/ingest/odds/testdata/odds_nfl.json` | Captured from The Odds API 2026-11-07; same shape. |
| `parlay-week-9-featured-props` | `GET parlay-api.com/.../props?eventId=dc56d980...&markets=...` | `internal/ingest/odds/testdata/props_sample.json` | **Derived.** Hand-built Parlay API props shape, not a live capture. |
| `nfelo-week-9-projections` | `GET raw.githubusercontent.com/.../historic_projected_spreads.csv` | `internal/ingest/nfelo/testdata/historic_projected_spreads.csv` | Captured 2026-11-09, unedited. |
| `weather-lumen` | Open-Meteo Lumen Field, `2026-11-08T21:00`–`23:00` | `internal/ingest/weather/testdata/forecast_lumen.json` | Captured 2026-11-07. |
| `weather-arrowhead` | Open-Meteo Arrowhead, `2026-11-08T18:00`–`20:00` | `internal/ingest/weather/testdata/forecast_arrowhead.json` | **Derived.** |
| `weather-hard-rock` | Open-Meteo Hard Rock, `2026-11-08T18:00`–`20:00` | `internal/ingest/weather/testdata/forecast_hard_rock.json` | **Derived.** |
| `weather-lincoln-financial` | Open-Meteo Lincoln Financial, `2026-11-08T18:00`–`20:00` | `internal/ingest/weather/testdata/forecast_lincoln_financial.json` | **Derived.** |
| `weather-northwest` | Open-Meteo Northwest, `2026-11-08T18:00`–`20:00` | `internal/ingest/weather/testdata/forecast_northwest.json` | **Derived.** |
| `weather-bank-of-america` | Open-Meteo Bank of America, `2026-11-08T18:00`–`20:00` | `internal/ingest/weather/testdata/forecast_bank_of_america.json` | **Derived.** |
| `weather-sofi` | Open-Meteo SoFi, `2026-11-08T21:00`–`23:00` | `internal/ingest/weather/testdata/forecast_sofi.json` | **Derived.** |
| `weather-gillette` | Open-Meteo Gillette, `2026-11-08T21:00`–`23:00` | `internal/ingest/weather/testdata/forecast_gillette.json` | **Derived.** |
| `weather-levis` | Open-Meteo Levi's, `2026-11-08T21:00`–`23:00` | `internal/ingest/weather/testdata/forecast_levis.json` | **Derived.** |
| `weather-soldier` | Open-Meteo Soldier Field, `2026-11-09T01:00`–`03:00` | `internal/ingest/weather/testdata/forecast_soldier.json` | **Derived.** |

The four indoor week 9 venues need no forecast. `forecast_highmark.json` remains a weather-client fixture, but the week 9
sandbox does not register it because Buffalo plays at U.S. Bank Stadium, an indoor venue.

### Provenance

Every fixture declares `Captured` or `Derived`. Captured bytes are real provider responses trimmed only by removing
whole events or rows. Derived bytes are edited or synthesized snapshots, and the registry rejects a derived fixture
without a note. The distinction is deliberate: the scoreboard, event list, line board, and projections exercise the real
parsers on real provider shapes, while the hand-built props board and missing forecasts fill gaps without pretending a
provider sent them.

### Credit headers

The three Parlay API fixtures carry `X-Requests-Remaining`, `X-Requests-Used`, and `X-Requests-Last`. Remaining starts at
620 after the free event list, falls to 617 when the 3-credit line call is bought, and falls to 614 when the featured
props call is bought. Those balances sit above `refresh.OddsLinesFloor` (50) and `refresh.OddsPropsFloor` (150), so the
normal event, line, and lazy prop paths enforce their floors instead of silently skipping work.

## Adding a response

1. Save the real body under the matching `internal/ingest/{source}/testdata/` directory. Trim it only by removing whole
   events or rows, and update that directory's README.
2. Add a `sandbox.Fixture` to `week9Fixtures` or `week9Weather` with the exact method, URL, query, stage, and
   provenance. A derived file must explain what it was built from in `Note`.
3. Run `go test ./internal/sandbox`. The tests drive the real clients, so a changed query, week, team name, or kickoff
   fails visibly instead of falling through to a provider.

## Tests

`internal/sandbox/sandbox_test.go` covers the morning replay end to end:

- the ESPN scoreboard parses 14 Sunday and Monday games for week 9;
- Parlay API's event list matches all 14 games and the 3-credit line board parses every market with its saved credits;
- every outdoor kickoff resolves a three-hour forecast, including the captured Lumen window and a derived one;
- nfelo returns a week 9 projection for every slate game;
- the featured prop board parses through Parlay API's flat `/props` shape;
- an unknown week, an unregistered The Odds API request, an unknown stage, and a missing fixture file all fail closed.
