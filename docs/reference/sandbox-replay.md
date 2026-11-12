---
id: sandbox-replay
title: Sandbox replay
type: reference
summary: The fail-closed HTTP transport and named week 9 response registry behind the local Sunday sandbox, the requests they answer or leave as labeled empty fallbacks, which fixtures are real captures versus derived snapshots, and how startup, reset, stages, seeded people, and scripted grades replay them.
tags: [sandbox, replay, fixtures, fallbacks, testing, espn, weather, parlay-api, nfelo, provenance, offline, startup, clock, reset, seeding, personas, stages, grading]
sources: [internal/sandbox/registry.go, internal/sandbox/transport.go, internal/sandbox/scenario.go, internal/sandbox/state.go, internal/sandbox/reset.go, internal/sandbox/seed.go, internal/sandbox/plan.go, internal/sandbox/sandbox_test.go, internal/sandbox/state_test.go, cmd/server/sandbox.go, cmd/server/main.go, internal/config/config.go, internal/ingest/espn/testdata/README.md, internal/ingest/odds/testdata/README.md, internal/ingest/weather/testdata/README.md, internal/ingest/nfelo/testdata/README.md]
last_verified: 2026-11-11
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
- The transport has no base transport and no network fallback. An unregistered request returns `sandbox.ErrNoResponse`
  with the method, credential-free URL, and stage. Missing files, duplicate requests, and incomplete fixtures fail
  `NewRegistry` before startup. The registry may also hold fallbacks for optional research surfaces; a fallback answers
  only when no exact fixture matches, and its response carries `X-Sandbox-Replay: empty`.
- Responses carry the saved status, headers, and body bytes. Each call gets a fresh body reader, so the transport is
  safe for the concurrent source goroutines.
- Nothing here touches the response archive. The sandbox client's transport is the replay transport, not an
  `archive.Transport` wrapper, so replayed calls never enter `api_responses`.

`SetStage` switches the snapshot set for later requests. Four stages are registered: `morning`, `early-loss`,
`all-clear`, and `sunday-complete`. Each stage has a complete response set, including its own scoreboard snapshot and
the supporting lines, projections, props, and forecasts that are due at its clock.

## Server startup

`SUNDAY_SANDBOX=1` makes `cmd/server` build `sandbox.Week9` from the checkout root before it mounts any route. One
`http.Client` with the replay transport becomes the transport for the ordinary ESPN, weather, Parlay API, and nfelo
clients. The Parlay API client gets a synthetic key and no fallback provider, so a real key from `.env` is never passed.
Sandbox mode also skips Google provider construction and forces the dev sign-in form.

A fresh `./data/sunday-sandbox.db` runs `refresh.Service.RunOnce` before the server listens: the scoreboard first, then
the weather, lines, and projections sources, all synchronously. Startup then writes a `morning` row with the scenario
clock to the sandbox-only `sandbox_state` table. A later boot reads that row, restores the stage and clock, and does not
rebuild. The schema migrations stay untouched and ordinary dev keeps `./data/parlay.db`; sandbox mode ignores `DB_PATH`.
See [Persistence](../concepts/persistence.md).

The sandbox scenario clock is `2026-11-08T13:00:00Z` on a first boot: the week 9 Sunday morning. Refresh, slips, and the
web handlers share it; auth cookies and SSE timers stay on real time. See [Data refresh](../concepts/data-refresh.md). A
later stage moves the same clock to its fixed instant.

## Stages

The controls page and `POST /dev/sandbox/stage` select one of four situations. Every stage rebuilds the morning baseline
first, so each control works from any previous state. It then sets the scenario clock and replay stage, saves that
state, runs the real refresh synchronously, locks every shared member, and records the manifest's scripted manual grades
through `slips.Service.SetLegResult`. It never infers a grade from a score or writes a result column directly.

| Stage | Clock | Visible state |
| --- | --- | --- |
| `morning` | `2026-11-08T13:00:00Z` | Full scheduled week, open slip, six pending picks. |
| `early-loss` | `2026-11-08T15:30:00Z` | Madrid game live at Atlanta 17, Cincinnati 10; other games scheduled; slip locked; Ada's Bengals ML scripted `LOST`; other five picks pending; slip `LOST`. |
| `all-clear` | `2026-11-09T00:30:00Z` | Every Sunday game final with a scripted score; Monday night upcoming; slip locked; all six picks scripted `WON`; slip `WON`. |
| `sunday-complete` | `2026-11-10T04:30:00Z` | Every Sunday and Monday game final with scores; slip locked; all six picks scripted `WON`. |

The scripted scores keep the declared outcomes coherent: Cincinnati wins 31-24 for a 55-point total, Kansas City wins by
13 against a 9.5 spread, Philadelphia wins, and the Rams win by 11 in a 51-point game. Those are manual scripted grades
for the sandbox, not auto-grading behavior.

A stage change removes and recreates the seeded people, so the handler reissues the same browser's real session and
group cookies. The route answers `HX-Refresh`, and the controller publishes the normal week and slip signals after the
writes commit. A page-load refresh and a stage change share one mutex, so a pass that began earlier finishes before the
chosen stage and cannot put morning data back.

## Reset and seeded people

`POST /dev/sandbox/reset` is mounted only while `SUNDAY_SANDBOX=1` and is the same operation as the `morning` stage. It
sits inside the normal `RequireUser`, group scope, and HTMX CSRF checks, so it is an ordinary signed-in route. The
response asks HTMX to refresh the page.

One reset:

1. sets the scenario to the `morning` stage and saves that stage and clock;
2. clears every disposable table (`api_responses`, `api_bodies`, activity, wagers, legs, slip members, slips, player
   research, player matches, nfelo games, odds, games, weeks, group members, groups, users, and `sync_state`) in one
   transaction;
3. clears the refresh service's in-memory failure backoff and runs the real morning ingest synchronously;
4. signs in three fixed `.test` personas through `auth.SignIn`, creates one shared group, and adds the other two through
   the normal invite join;
5. leaves a fourth persona in its own group as the outsider;
6. creates the shared slip with `Store.CreateSlip` and adds six named picks through `slips.Service.AddLeg`;
7. syncs the featured game's prop board and publishes the final `week:{id}` and `slip:{id}` signals after every write
   commits.

The schema, the migration version, and the sandbox-only `sandbox_state` row survive. The seeded people are Ada
(`ada@sandbox.test`), Ben (`ben@sandbox.test`), and Cleo (`cleo@sandbox.test`) in the shared `Sunday Sandbox` group,
plus Dex (`dex@sandbox.test`) outside it. The `Week 9 Sandbox` slip stays `OPEN` with six `PENDING` legs, two per shared
person. Stages refer to people and legs by symbolic IDs, not the numeric row IDs a reset creates.

A reset removes and recreates the signed-in persona, so the handler reissues that browser's real session and group
cookies for the same persona. Any other signed-in identity is cleared and sent back to sign-in. A page-load refresh
starts at most one background pass and waits behind a reset, so a pass that began earlier cannot write after the fresh
morning. A failed rebuild answers 500 and leaves reset available to retry. Startup writes the stage row only after the
first full build succeeds, so a failed first boot retries instead of restoring a half-built sandbox.

## Week 9 registry

`sandbox.Week9(fsys)` preloads every fixture from an `fs.FS` rooted at the repository, so the registry references the
saved source files instead of copying large bodies. The morning set is the full scheduled slate, game lines, nfelo
projections, every outdoor forecast, the featured game's lazy prop board, and the research answers that board needs:
both team rosters, the game summary, and the game logs behind its players. Each later stage reuses those supporting
files with its stage name and swaps in a same-week scoreboard snapshot, so lines, projections, and due forecasts still
replay at that stage's clock.

| Fixture | Request | File | Provenance |
| --- | --- | --- | --- |
| `espn-week-9-scoreboard` | `GET site.api.espn.com/.../scoreboard` | `internal/ingest/espn/testdata/scoreboard_week.json` | Captured 2026-11-07. |
| `espn-week-9-scoreboard-early-loss` | `GET site.api.espn.com/.../scoreboard` | `internal/ingest/espn/testdata/scoreboard_week9_live.json` | **Derived.** Madrid game live at ATL 17, CIN 10; other slate games scheduled. |
| `espn-week-9-scoreboard-all-clear` | `GET site.api.espn.com/.../scoreboard` | `internal/ingest/espn/testdata/scoreboard_week9_final_sunday.json` | **Derived.** All Sunday games final with scripted scores; Monday night scheduled. |
| `espn-week-9-scoreboard-sunday-complete` | `GET site.api.espn.com/.../scoreboard` | `internal/ingest/espn/testdata/scoreboard_week9_complete.json` | **Derived.** All Sunday and Monday games final with scripted scores. |
| `parlay-week-9-events` | `GET parlay-api.com/.../events` | `internal/ingest/odds/testdata/events_nfl.json` | Captured from The Odds API 2026-11-07; Parlay API serves the same shape. |
| `parlay-week-9-lines` | `GET parlay-api.com/.../odds?regions=us&markets=h2h,spreads,totals...` | `internal/ingest/odds/testdata/odds_nfl.json` | Captured from The Odds API 2026-11-07; same shape. |
| `parlay-week-9-featured-props` | `GET parlay-api.com/.../props?eventId=dc56d980...&markets=...` | `internal/ingest/odds/testdata/props_sample.json` | **Derived.** Hand-built Parlay API props shape, not a live capture. |
| `nfelo-week-9-projections` | `GET raw.githubusercontent.com/.../historic_projected_spreads.csv` | `internal/ingest/nfelo/testdata/historic_projected_spreads.csv` | Captured 2026-11-09, unedited. |
| `espn-featured-home-roster` | `GET site.api.espn.com/.../teams/1/roster` | `internal/ingest/espn/testdata/roster_falcons.json` | Captured 2026-11-11. Read before the props sync matches player names. |
| `espn-featured-away-roster` | `GET site.api.espn.com/.../teams/4/roster` | `internal/ingest/espn/testdata/roster_bengals.json` | Captured 2026-11-11. Resolves the featured prop board's players. |
| `espn-week-9-featured-summary` | `GET site.api.espn.com/.../summary?event=401873038` | `internal/ingest/espn/testdata/summary_week9_featured.json` | Captured 2026-11-11. The research page's injury report. |
| `espn-gamelog-burrow` | `GET site.web.api.espn.com/.../athletes/3915511/gamelog` | `internal/ingest/espn/testdata/gamelog_burrow.json` | Captured 2026-11-11. Player research last-5 games. |
| `espn-gamelog-chase` | `GET site.web.api.espn.com/.../athletes/4362628/gamelog` | `internal/ingest/espn/testdata/gamelog_chase.json` | Captured 2026-11-11. Player research last-5 games. |
| `espn-gamelog-higgins` | `GET site.web.api.espn.com/.../athletes/4239993/gamelog` | `internal/ingest/espn/testdata/gamelog_higgins.json` | Captured 2026-11-11. Player research last-5 games. |
| `espn-gamelog-mcpherson` | `GET site.web.api.espn.com/.../athletes/4360234/gamelog` | `internal/ingest/espn/testdata/gamelog_mcpherson.json` | Captured 2026-11-11. Player research last-5 games. |
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

The four indoor week 9 venues need no forecast. `forecast_highmark.json` remains a weather-client fixture, but the week
9 sandbox does not register it because Buffalo plays at U.S. Bank Stadium, an indoor venue. The registry holds 88
fixtures: the 22 morning responses plus a 22-response set for each of the three later stages.

The featured game is the one game whose whole card and research surface is captured. Opening its props syncs both
rosters, so its prop board resolves real players instead of showing an empty state, and the research page and player
pages have an injury report and last-5 game logs. Joe Mixon has prop lines but is not on the captured Bengals roster, so
his lines stay unmatched and exercise the manual-match flow.

### Empty fallbacks

Every other slate game answers its optional research and props requests with an empty fallback instead of failing the
sync. A fallback is a request pattern, not a capture. It applies only when no exact fixture matches, and the transport
marks its response with `X-Sandbox-Replay: empty`, so a debugging session can tell the two apart.

| Fallback | Request pattern | Body | Why |
| --- | --- | --- | --- |
| `props-empty` | `GET parlay-api.com/.../props` | `[]` | Only the featured game has a captured prop board. The empty array still carries credit headers above the props floor. |
| `summary-empty` | `GET site.api.espn.com/.../summary` | `{}` | A game outside the featured one has no captured injury report. |
| `roster-empty` | `GET site.api.espn.com/.../teams/*/roster` | `{"athletes":[]}` | Only the featured game's two teams have captured rosters. |
| `gamelog-empty` | `GET site.web.api.espn.com/.../athletes/*/gamelog` | `{}` | Only the featured prop board's players have captured game logs. |

The paths use `path.Match` patterns, so `*` matches one path segment and query parameters are ignored. Each stage
registers the same four fallbacks. The game card then renders its normal "No prop lines available yet." message and the
research page shows no injuries instead of logging a replay miss. A request outside these four surfaces still fails
closed, and the sandbox tests keep the featured game's captures non-empty so a client change or fixture drift cannot
hide behind a fallback.

### Provenance

Every fixture declares `Captured` or `Derived`. Captured bytes are real provider responses trimmed only by removing
whole events or rows. Derived bytes are edited or synthesized snapshots, and the registry rejects a derived fixture
without a note. The distinction is deliberate: the captured event list, line board, projections, and morning scoreboard
exercise the real parsers on real provider shapes, while the scripted live and final scoreboards, the hand-built props
board, and the missing forecasts fill gaps without pretending a provider sent them.

### Credit headers

The three Parlay API fixtures carry `X-Requests-Remaining`, `X-Requests-Used`, and `X-Requests-Last`. Remaining starts
at 620 after the free event list, falls to 617 when the 3-credit line call is bought, and falls to 614 when the featured
props call is bought. Those balances sit above `refresh.OddsLinesFloor` (50) and `refresh.OddsPropsFloor` (150), so the
normal event, line, and lazy prop paths enforce their floors instead of silently skipping work.

## Adding a response

1. Save the real body under the matching `internal/ingest/{source}/testdata/` directory. Trim it only by removing whole
   events or rows, and update that directory's README.
2. Add a `sandbox.Fixture` to `morningFixtures`, `week9Weather`, or `week9StageScoreboards` with the exact method, URL,
   query, stage, and provenance. A derived file must explain what it was built from in `Note`. A stage scoreboard is
   picked up by `stageFixtures`, which reuses the morning supporting responses under that stage.
3. If the request is an optional surface that most games do not capture, add a `sandbox.Fallback` to `week9Fallbacks`
   instead, with a provider-shaped empty body and a note saying why the scenario leaves it empty.
4. Run `go test ./internal/sandbox`. The tests drive the real clients, so a changed query, week, team name, or kickoff
   fails visibly instead of falling through to a provider.

## Tests

`internal/sandbox/sandbox_test.go` covers the replay end to end:

- the ESPN scoreboard parses 14 Sunday and Monday games for week 9;
- each later stage parses its own coherent slate: one live game at early loss, 13 finals with the Monday game still
  scheduled at all-clear, and 14 finals at Sunday completion;
- Parlay API's event list matches all 14 games and the 3-credit line board parses every market with its saved credits;
- every outdoor kickoff resolves a three-hour forecast, including the captured Lumen window and a derived one;
- nfelo returns a week 9 projection for every slate game;
- the featured prop board parses through Parlay API's flat `/props` shape;
- the featured game's rosters parse, its summary yields key-position injuries, and its four prop-board players have game
  logs with games played;
- another game's props, summary, rosters, and game logs return labeled empty bodies instead of replay misses, while an
  unknown week, an unregistered The Odds API request, an unknown stage, and a missing fixture file all fail closed.

`internal/sandbox/state_test.go` covers the one-row stage record, rejects a stage the registry does not know, and checks
the scenario clock and every stage name and time. `cmd/server/sandbox_test.go` boots a fresh sandbox through the real
clients and refresh, asserts the whole morning landed offline with no archive rows, and reconstructs the runtime to show
the stage and clock survive a restart without re-fetching. It also resets twice and compares the people, memberships,
slip, legs, results, activity, games, and sync state; proves the seeded people and slip are real service rows; checks
that manual edits and a stale failure backoff disappear; shows a sentinel normal dev file is untouched; observes the
post-commit week and slip signals; and proves the signed-in persona keeps a working session and shared group after
reset. One server test opens the featured game the way a browser does: it syncs props, resolves Joe Burrow against the
replayed roster, reads the injury report, and fetches his game log, so a missing research fixture fails the suite
instead of only showing up in the logs. Another opens a non-featured game and checks that its props and research land as
empty rather than failing the sync.

For stages, the server tests select each target directly and in sequence, reset back to morning, and check that early
loss settles the slip `LOST` while five legs stay pending, all-clear wins every leg with every Sunday game final, and
Sunday completion makes all 14 games final. They exercise the HTTP stage route with real cookies and render the
controls, pick board, and hub pages, reject unknown stages without changing rows, clock, or cookies, race a page-load
kick against a stage change, and prove the stage route is absent outside sandbox mode.
