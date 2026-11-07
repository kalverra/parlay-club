# Data sources

Three free sources. Each has a client package under `internal/ingest/` behind a small interface, so `internal/refresh`
and tests can swap in fakes. Clients take a base URL and an `*http.Client` (10 s timeout) in their constructor, and send
`User-Agent: parlay-club`.

The field paths and market keys below come from memory of these APIs. Before writing each parser, save a real response
into `testdata/` and confirm the paths against it; fix this doc where they differ.

## Sources and staleness

`internal/refresh` reads `sync_state.fetched_at` per source key. "Game day" means Sunday or Monday in
`America/New_York`.

| Source key            | What                                   | Stale after                                                     |
| --------------------- | -------------------------------------- | --------------------------------------------------------------- |
| `espn:scoreboard`     | Current week's games, scores, status   | 10 min while a game is live; 30 min on game days; 6 h otherwise |
| `espn:summary:{game}` | Injuries for one game                  | 2 h; only for games not yet final                               |
| `weather:{game}`      | Kickoff-window weather for one game    | 1 h within 48 h of kickoff; skip earlier and after kickoff      |
| `odds:lines`          | Game lines for the week                | see the budget in [docs/reference/data-sources.md](../docs/reference/data-sources.md) |
| `odds:props:{game}`   | Props for one game, on demand          | 2 h; skip after kickoff                                         |
| `espn:roster:{team}`  | Team roster (`props-research`)                  | 24 h                                                            |
| `espn:gamelog:{id}`   | Player game log (`props-research`)              | 12 h                                                            |
| `grade`               | Auto-grading (`grading-record-book`)                 | runs whenever a FINAL game has PENDING legs                     |

## ESPN (free, no auth, unofficial)

### Summary (injuries, box score)

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/summary?event={id}`

- `injuries[]`: per team, a list of athletes with position and status. Store the block raw in `games.injuries_json`; the
  research view filters it.
- `boxscore.players[]`: per team, `statistics[]` groups (`passing`, `rushing`, `receiving`, `kicking`, and return
  groups), each with `keys[]` and `athletes[].stats[]` aligned to the keys. Parse by key name, never by index.

### Roster (`props-research`)

`GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/{espnTeamID}/roster`

Athletes with `id`, `fullName`, and `position.abbreviation`, grouped by side of the ball.

### Game log (`props-research`)

`GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/{athleteID}/gamelog`

Per-game stat rows for the season, keyed by `labels`/`names`. Extract, most recent first, the values each prop market
uses (product.md "Grading" table names the stat).

## The Odds API (free tier: 500 credits/month)

- Base: `https://api.the-odds-api.com/v4/sports/americanfootball_nfl`. Auth: `apiKey` query param.
- Cost per call = markets returned × regions. Use `regions=us` and `oddsFormat=american`.
- After every call, store `x-requests-remaining` in `sync_state.odds_credits_remaining`.

Game lines, event matching, bookmakers, and the credit budget are documented in
[docs/reference/data-sources.md](../docs/reference/data-sources.md). What remains here belongs to `props-research`.

### Endpoints

- Props for one game: `GET /events/{eventId}/odds?regions=us&markets={prop keys}&oddsFormat=american`.
- Event list: `GET /events` (free) gives `id`, `home_team`, `away_team`, `commence_time` for matching.

### Market keys

| Odds API key            | Market       |
| ----------------------- | ------------ |
| `team_totals`           | `TEAM_TOTAL` |
| `player_pass_yds`       | `PASS_YDS`   |
| `player_pass_tds`       | `PASS_TDS`   |
| `player_rush_yds`       | `RUSH_YDS`   |
| `player_rush_attempts`  | `RUSH_ATT`   |
| `player_reception_yds`  | `REC_YDS`    |
| `player_receptions`     | `RECEPTIONS` |
| `player_anytime_td`     | `ANYTIME_TD` |
| `player_kicking_points` | `KICKER_PTS` |

`team_totals` and the player markets are only on the per-event endpoint, so they load with props. For player markets,
outcome `description` is the player, `name` is Over/Under/Yes, and `point` is the line.

## Player matching (`props-research`)

- Normalize a name: lowercase; drop `.`, `'`, `-`, and commas; drop a trailing `jr`, `sr`, `ii`, `iii`, `iv`, `v`;
  collapse spaces.
- For a prop in a game, look up `player_matches(odds_name, team)` for each of the game's two teams. On a miss, compare
  against both teams' ESPN rosters by normalized name. Exactly one hit → store it (manual = 0) and use it. Zero or
  several → leave `player_id` NULL.
- Rows with `manual = 1` are never overwritten. A small form on the research view lets a user fix a match.
- The Odds API doesn't say which team a player is on; the roster match supplies `team`.

## Fixtures and live tests

- Save real responses as `internal/ingest/{source}/testdata/*.json`. Trim them only by removing whole events.
- Unit tests serve fixtures with `httptest.NewServer` and point the client's base URL at it.
- One `TestLive*` per source hits the real endpoint, checks that parsing returns sane data, and calls `t.Skip` under
  `testing.Short()`. The Odds API live test uses only the free `/events` endpoint and skips when `ODDS_API_KEY` is
  unset.
