# Phase 1: ESPN scoreboard

## Done

- Added `internal/ingest/espn`, a client with a base URL, shared `*http.Client`, 10 second default timeout,
  `User-Agent: parlay-club`, and JSON decoding. It returns plain structs and never touches the database.
- Added the scoreboard client and parser:
  - `ScoreboardParams` sends `dates={season}`, `seasontype`, and `week`; the zero value asks ESPN for the current week.
  - `parseScoreboard` keeps only events whose kickoff is Sunday or Monday in `America/New_York`, with `time/tzdata`
    embedded so the zone resolves in a container. Monday night games stored in UTC on Tuesday stay on the slate, and
    the 9:30 a.m. ET London kickoff stays too.
  - Kickoff parsing accepts RFC 3339 with seconds and ESPN's usual minute precision.
  - Event state maps `pre` → `SCHEDULED`, `in` → `LIVE`, `post` → `FINAL`; an unknown state is an error.
  - Competitors map to home and away by `homeAway`, with `team.id`, `abbreviation`, `displayName`, and score. A
    scheduled game has no score even when ESPN sends a placeholder zero. A missing home or away team is an error.
  - Venue maps `id`, `fullName`, and `indoor`.
- Saved three real scoreboard fixtures and a README under `internal/ingest/espn/testdata/`:
  - `scoreboard_week.json`: week 9, 15 events including the Thursday game and the Madrid game. The parser returns the
    14 Sunday and Monday games.
  - `scoreboard_live.json`: week 4 during Sunday afternoon games, with 8 live, 1 final, and 6 scheduled slate games.
  - `scoreboard_international.json`: week 5, trimmed to the London game, proving the 9:30 a.m. ET kickoff survives.
- Added the store side:
  - `db.Game`, `db.GameStatus`, and `db.ErrGameStatusInvalid`.
  - `Store.UpsertGame` inserts or updates only the slate-owned fields: week, teams and names, ESPN IDs, kickoff,
    status, venue, indoor flag, and scores. Odds, weather, and injuries are left alone.
  - The conflict update carries a `WHERE` guard over every owned column, so a rerun with the same data leaves the row
    and its `updated_at` untouched.
  - `Store.GameByID` returns a full row or `ErrGameNotFound`.
  - Tests cover the create path, an unchanged rerun after later syncs wrote odds/weather/injuries, a changed live game
    that updates only slate fields, an unknown status refusal, and a missing lookup.
  - `UpsertWeek` already existed from `slips-live` with its own create-and-reuse test, so this phase did not need to
    add it.
- Docs: added `docs/reference/data-sources.md` with the client contract, the scoreboard endpoint, the field table, the
  Sunday/Monday filter, and the upsert rules. Regenerated `docs/llms.txt`. Deleted the Scoreboard section from
  `plan/data-sources.md`, leaving the Summary section for later.
- Verification: `go test ./...` passes and `golangci-lint run ./...` reports 0 issues.

## Deviations

- The task said to add week and game upserts. `UpsertWeek` already landed in `slips-live`, so this phase only added
  `UpsertGame` and its tests. The week idempotence behavior was already covered.
- The live fixture is a web archive snapshot of week 4, not a response captured live while building, because that
  slate had already passed. It is still an unedited real response; only whole events were removed from the files.
- The international fixture is trimmed to the London event so the test reads clearly. The full week 9 fixture also
  contains the Madrid game, so international venues are covered in a complete slate as well.
- The parser treats an unknown status, a missing competition, a missing home or away team, or an unparsable score on a
  live or final game as errors rather than filling defaults. That matches the docs' "no guesses" rule but is stricter
  than the design spelled out.

## Open issues

- `UpsertGame` returns the stored row but does not say whether it changed anything. Refresh needs a changed signal so
  it can publish `week:{id}` only when a sync actually moved rows. Either compare `updated_at` before and after, or
  make the store return a `changed` bool.
- `UpsertGame` requires the week row first because of the foreign key. There is no sync orchestration yet; callers
  must call `UpsertWeek` and then upsert each game.
- The scoreboard response has no latitude or longitude, and only some venues include a city/state/country address.
  Phase 2 cannot derive coordinates from the scoreboards alone; the venue map needs a manual seed or another geocoding
  source, keyed by the ESPN venue IDs the fixtures already surface.
- There is no week-slate list query yet (for example `GamesByWeek`). The Games tab and refresh will need one; today
  only `GameByID` and `GameKickoff` exist.
- The current-week concept is still undefined, as in `slips-live`. The home page and new-slip form still fall back to
  `LatestWeek`.
- Only regular-season scoreboards are fixture-tested. `seasontype=3` shares the same `WeekID` formula and slate filter
  but has no saved postseason response.

## Next-phase notes

- ESPN field paths matched `plan/data-sources.md` for the scoreboard; no alias or path fixes were needed. Keep the
  fixture-first check for the Summary endpoint later.
- Reuse `espn.NewClient`, the `httptest` fixture pattern, and the `testdata/README.md` source table for any further ESPN
  endpoints. The scoreboard parser is `parseScoreboard` over the unexported response structs, so sibling parsers can
  stay in the same package and share the team and venue shapes.
- For the weather phase, extend `espn.Venue` only if the new source returns more venue fields; the scoreboard fields we
  need now are already mapped. Weather writes need their own store method because `UpsertGame` deliberately leaves
  `temp_f`, `wind_mph`, `wind_gust_mph`, and `precip_in` untouched.
- The `testdata` files are large single-line JSON responses. Keep them as saved, and filter in tests instead of
  hand-editing fields.
- The Games tab should render from a new week-slate query, not from repeated `GameByID` calls.

## Files touched

- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `internal/db/games.go`
- `internal/db/games_test.go`
- `internal/ingest/espn/client.go`
- `internal/ingest/espn/scoreboard.go`
- `internal/ingest/espn/scoreboard_test.go`
- `internal/ingest/espn/testdata/README.md`
- `internal/ingest/espn/testdata/scoreboard_international.json`
- `internal/ingest/espn/testdata/scoreboard_live.json`
- `internal/ingest/espn/testdata/scoreboard_week.json`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
