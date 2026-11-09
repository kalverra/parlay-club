# Data sources

Five free sources. Each has a client package under `internal/ingest/` behind a small interface, so `internal/refresh`
and tests can swap in fakes. Clients take a base URL and an `*http.Client` (10 s timeout) in their constructor, and send
`User-Agent: parlay-club`. Parlay API is the primary odds feed; The Odds API enriches its game lines and props, and serves
as fallback. nfelo adds model spreads, win probabilities, and quarterback adjustments.

Scoreboard slates, weather forecasts, game lines, props, team rosters, player game logs, summary injuries, player
matching, and model projections are documented in [docs/reference/data-sources.md](../docs/reference/data-sources.md).
What remains here belongs to `grading-record-book`.

The field paths below come from memory of these APIs. Before writing each parser, save a real response into `testdata/`
and confirm the paths against it; fix this doc where they differ.

## Sources and staleness

`internal/refresh` reads `sync_state.fetched_at` per source key.

| Source key | What                                 | Stale after                                 |
| ---------- | ------------------------------------ | ------------------------------------------- |
| `grade`    | Auto-grading (`grading-record-book`) | runs whenever a FINAL game has PENDING legs |

## ESPN (free, no auth, unofficial)

### Box score (`grading-record-book`)

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/summary?event={id}`

- `boxscore.players[]`: per team, `statistics[]` groups (`passing`, `rushing`, `receiving`, `kicking`, and return
  groups), each with `keys[]` and `athletes[].stats[]` aligned to the keys. Parse by key name, never by index.

## Fixtures and live tests

- Save real responses as `internal/ingest/{source}/testdata/*.json`. Trim them only by removing whole events.
- Unit tests serve fixtures with `httptest.NewServer` and point the client's base URL at it.
- One `TestLive*` per source hits the real endpoint, checks that parsing returns sane data, and calls `t.Skip` under
  `testing.Short()`. The Parlay API live test uses only the free `/events` endpoint and skips when `ODDS_API_KEY` is
  unset.
