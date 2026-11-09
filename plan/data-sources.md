# Data sources

Five free sources. Each has a client package under `internal/ingest/` behind a small interface, so `internal/refresh`
and tests can swap in fakes. Clients take a base URL and an `*http.Client` (10 s timeout) in their constructor, and send
`User-Agent: parlay-club`. Parlay API is the primary odds feed; The Odds API enriches its game lines and serves as
fallback. nfelo adds model spreads, win probabilities, and quarterback adjustments.

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
| `odds:theoddsapi`     | The Odds API credit balance; enrichment and fallback lines | not scheduled; rides inside `odds:lines` |
| `odds:props:{game}`   | Props for one game, on demand          | 2 h; skip after kickoff                                         |
| `nfelo:projections`   | Model spreads, win probabilities, and QB adjustments for the week | 12 h; skip non-regular weeks and weeks where every game has kicked off |
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

## Parlay API (free tier: 1,000 credits/month)

- Base: `https://parlay-api.com/v1/sports/americanfootball_nfl`. Auth: `apiKey` query param.
- Game lines cost markets × regions: 3 credits for `regions=us` and `h2h,spreads,totals`. Bulk props cost a flat
  3 credits per call for all nine markets. Use `oddsFormat=american`.
- `includeLinks=true&includeSids=true` cost no extra credits on `/odds` and `/props`. Parlay API populates validated
  links and sids on eligible FanDuel outcomes only; DraftKings and other books omit them. The Odds API enrichment fills
  those gaps. The flags add nothing to the lines-and-props budget, so the credit floors stay as they are.
- After every call, store `x-requests-remaining` under that provider's `sync_state` key: `odds:lines` for Parlay API
  and `odds:theoddsapi` for The Odds API.

Game lines, event matching, bookmakers, and the credit budget are documented in
[docs/reference/data-sources.md](../docs/reference/data-sources.md). What remains here belongs to `props-research`.

### Endpoints

- Game lines for the week: `GET /odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american&includeLinks=true&includeSids=true`.
- Props for one game: `GET /props?eventId={eventId}&markets={prop keys}&oddsFormat=american&includeLinks=true&includeSids=true&limit=10000`.
  It costs a flat 3 credits and returns a flat array of quote rows, not one event object.
- Event list: `GET /events` (free) gives `id`, `home_team`, `away_team`, `commence_time` for matching. Parlay API may
  send `commence_time: null`, which the client skips.
- The per-event `/events/{id}/odds` endpoint is no longer used for Parlay API props. The Odds API uses it for its own
  per-event prop board (below).

### Deep-link flags

An earlier 2026-11-07 probe of The Odds API established the link shapes below and confirmed that the flags cost no
extra credits. Parlay API returns the same game-line shape and the same zero-cost flags.

- **Cost:** zero extra credits. Game lines stay at 3 credits with and without the flags; props stay at their flat
  3-credit call. Remaining falls only by the base cost each time.
- **Fields:** with the flags, each game-line outcome gains `link` and `sid`. On `/props`, each quote gains `over_link`,
  `under_link`, `over_sid`, and `under_sid`. Without the flags, the fields are absent.
- **FanDuel on Parlay API:** Parlay API returns a validated link and sid on eligible FanDuel outcomes when a matching
  native quote is open. DraftKings and other books omit them. Treat a missing link as normal and store NULL.
- **DraftKings on The Odds API:** The Odds API returns `link`, `sid`, and the bookmaker's `event_sid` on DraftKings and
  FanDuel game-line and prop outcomes. Refresh enriches Parlay API rows with them. If the linked line differs from an
  unlinked primary row, the row adopts the linked line and price so the link and line agree.
- **Link shape:** FanDuel is `https://sportsbook.fanduel.com/addToBetslip?marketId={marketId}&selectionId={sid}`,
  where the sid is the numeric selection ID. The FanDuel combiner reads `marketId` from the stored link and the
  selection ID from `sid`, so single and combined links work unchanged. DraftKings' link is
  `https://sportsbook.draftkings.com/?outcomes={sid}`, and its combined link is
  `https://sportsbook.draftkings.com/event/{eventSid}?outcomes={sid1},{sid2}`.
- **Manual fallback:** a leg whose book sends no link renders as a manual checklist item (`no link`). The exact export
  shapes are in `docs/concepts/export.md`.

Saved fixtures: `internal/ingest/odds/testdata/odds_game_links.json` and
`internal/ingest/odds/testdata/odds_prop_links.json` come from The Odds API probe. The Parlay API `/props` shape is in
`internal/ingest/odds/testdata/props_sample.json`.

## The Odds API (free tier: 500 credits/month)

- Base: `https://api.the-odds-api.com/v4/sports/americanfootball_nfl`. Auth: `apiKey` query param.
- Game lines cost 3 credits for `regions=us` and `h2h,spreads,totals`. The Odds API bills by `markets × regions`, so
  requesting every US book costs the same as filtering to DraftKings.
- Per-event props cost 1 credit per market returned, not the flat 3 of Parlay API's `/props`. A nine-market board costs
  9 credits.
- `includeLinks=true&includeSids=true` cost no extra credits. DraftKings and FanDuel outcomes carry `link` and `sid`,
  and each bookmaker block carries `event_sid`.
- After every call, store `x-requests-remaining` under `odds:theoddsapi`.

### Enrichment and fallback

`refresh` matches each provider's free `/events` list to the week's games independently, maps rows to ESPN game IDs and
team abbreviations, then merges. The floors live in `internal/refresh/refresh.go`:

- **Enrichment:** when Parlay API bought the week's lines and The Odds API has at least 100 credits
  (`TheOddsAPIEnrichFloor`), fetch its lines too. Missing links, sids, event sids, books, and markets fill in from The
  Odds API. A linked primary row keeps its line, price, and link; an unlinked row that gains a link adopts the linked
  line and price when they differ.
- **Fallback:** when Parlay API is unconfigured, errors, or has fewer than 50 credits, The Odds API serves the week if
  it has at least 25 (`TheOddsAPIFallbackFloor`). A fallback event ID writes `games.odds_event_id` only when the game
  has no stored ID.
- **Failures:** `ErrNoAPIKey` means a provider is disabled. One provider failing while the other succeeds logs a warning
  and commits the survivor, without backoff. Every configured provider failing returns the joined error and backs off
  for 5 minutes.

### Per-event props

`GET /events/{eventId}/odds?regions=us&markets={prop keys}&oddsFormat=american&includeLinks=true&includeSids=true`

The response is one event object with `bookmakers[].markets[].outcomes[]`. `Client.EventProps` maps it into the same
`Prop` rows, with `bookmakers[].sid` in `Prop.EventSid`. `odds.MergeProps` merges a Parlay API board with one of these
boards using the same precedence as game lines. Refresh does not call it yet, and a caller must merge one event at a
time because the event ID backfills by book.

### Market keys

| Parlay API key            | Aliases                                                     | Market       |
| ------------------------- | ----------------------------------------------------------- | ------------ |
| `team_totals`             |                                                             | `TEAM_TOTAL` |
| `player_pass_yds`         | `player_passing_yards`, `player_pass_yards`                 | `PASS_YDS`   |
| `player_pass_tds`         | `player_passing_tds`, `player_passing_touchdowns`           | `PASS_TDS`   |
| `player_rush_yds`         | `player_rushing_yards`, `player_rush_yards`                 | `RUSH_YDS`   |
| `player_rush_attempts`    | `player_rushing_attempts`                                   | `RUSH_ATT`   |
| `player_reception_yds`    | `player_receiving_yds`, `player_receiving_yards`             | `REC_YDS`    |
| `player_receptions`       |                                                             | `RECEPTIONS` |
| `player_anytime_td`       | `player_anytime_touchdown_scorer`, `anytime_td`, `anytime_touchdown_scorer` | `ANYTIME_TD` |
| `player_kicking_points`   |                                                             | `KICKER_PTS` |

`team_totals` and the player markets come from `/props`, so they load with props. Keep only rows whose `period` is empty
or `FULL`; drop `1H`, `Q1`, and `UNKNOWN`. For player markets, `player` (or the legacy `player_name`) is the subject,
`over_price` and `under_price` are the sides, and `line` is the line. An anytime touchdown is a single `YES` row with no
line. `team_totals` puts the full team name in `player`.

## nfelo (free, no auth)

- Base: `https://raw.githubusercontent.com/greerreNFL/nfelo/main/output_data`. A public GitHub repo, no key.
- One file: `GET /historic_projected_spreads.csv`, about 1,550 rows from 2021 through the current week.
- Read columns by header name. The spread is `home_line_pre_regression`, from the home side; the win probability is
  `home_probability_nfelo`; the QB names and adjustments are `home_538_qb`, `away_538_qb`, `home_538_qb_adj`, and
  `away_538_qb_adj`. `home_closing_line_rounded_nfelo` echoes the market on most current rows, so it is not the model
  number.
- Map `OAK` to `LV` and `WAS` to `WSH`; every other code already matches ESPN.
- Refresh keeps the current regular-season week's rows in `nfelo_games`. The endpoint, field table, and storage are in
  [docs/reference/data-sources.md](../docs/reference/data-sources.md) and
  [docs/reference/schema.md](../docs/reference/schema.md).

## Player matching (`props-research`)

- Normalize a name: lowercase; drop `.`, `'`, `-`, and commas; drop a trailing `jr`, `sr`, `ii`, `iii`, `iv`, `v`;
  collapse spaces.
- For a prop in a game, look up `player_matches(odds_name, team)` for each of the game's two teams. On a miss, compare
  against both teams' ESPN rosters by normalized name. Exactly one hit → store it (manual = 0) and use it. Zero or
  several → leave `player_id` NULL.
- Rows with `manual = 1` are never overwritten. A small form on the research view lets a user fix a match.
- The Parlay API doesn't say which team a player is on; the roster match supplies `team`.

## Fixtures and live tests

- Save real responses as `internal/ingest/{source}/testdata/*.json`. Trim them only by removing whole events.
- Unit tests serve fixtures with `httptest.NewServer` and point the client's base URL at it.
- One `TestLive*` per source hits the real endpoint, checks that parsing returns sane data, and calls `t.Skip` under
  `testing.Short()`. The Parlay API live test uses only the free `/events` endpoint and skips when `ODDS_API_KEY` is
  unset.
