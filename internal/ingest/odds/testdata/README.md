# Odds API fixtures

Responses saved while building the odds client. The event list, game lines, and per-event link fixtures come from The
Odds API; the props fixture captures Parlay API's flat `/props` shape. Only whole events were removed unless a note says
otherwise, and no fields were edited.

| File | Source |
| --- | --- |
| `events_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events`, fetched 2026-11-07. 27 events across weeks 9 and 10, including the Monday night game. Costs 0 credits; the response reported `x-requests-remaining: 500`. Parlay API returns the same array but may send `commence_time: null`, which the client skips. |
| `odds_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american`, fetched 2026-11-07. The same 27 events; 204 bookmaker blocks and 1,162 priced selections. Costs 3 credits; the response reported `x-requests-remaining: 497`. Parlay API returns the same shape but can leave a side's `price` or `point` null in `regions=us`. |
| `odds_game_links.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events/dc56d980f527ffeef89940133b211b13/odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american&includeLinks=true&includeSids=true`, fetched 2026-11-07. Bengals at Falcons. Costs 3 credits; the response reported `x-requests-last: 3` and `x-requests-remaining: 491`. Every DraftKings and FanDuel outcome carries `link` and `sid`; other books vary. |
| `odds_prop_links.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events/dc56d980f527ffeef89940133b211b13/odds?regions=us&markets=player_pass_yds&oddsFormat=american&includeLinks=true&includeSids=true`, fetched 2026-11-07. The same game. Costs 1 credit; the response reported `x-requests-last: 1` and `x-requests-remaining: 489`. DraftKings and FanDuel returned `link` and `sid` on all four outcomes each. |
| `props_sample.json` | A hand-built fixture shaped like `GET https://parlay-api.com/v1/sports/americanfootball_nfl/props?eventId=dc56d980f527ffeef89940133b211b13&markets=team_totals,player_pass_yds,...&oddsFormat=american&includeLinks=true&includeSids=true&limit=10000`. It is not a captured response. It covers FanDuel `over_link`/`under_link`/`over_sid`/`under_sid`, book-specific aliases (`player_passing_yards`, `player_passing_touchdowns`, `player_receiving_yds`, `player_rushing_attempts`, `player_anytime_touchdown_scorer`), a `period: "UNKNOWN"` row, a `period: "1H"` row, an unpriced DFS row, `player_anytime_td`, and `team_totals`. The flat array costs a flat 3 credits per call. |

Bookmakers in the saved response include `draftkings`, `fanduel`, `betmgm`, `betrivers`, `betus`, `betonlineag`,
`bovada`, `lowvig`, and `mybookieag`. Some books omit the moneyline on future games, which the client keeps as-is: a
market that is absent produces no rows. The two `*_links.json` files keep the full per-event response. Only whole events
were removed unless a note says otherwise. They show that the flags add no credits and that a missing `link` is normal
for some books. On Parlay API, only FanDuel currently returns a validated `link` and `sid`; DraftKings legs have none.

The per-event odds endpoint returns one event object, while the all-events endpoint returns an array. Parlay API's
`/props` endpoint returns a different, flat array of quote objects, one row per book, player, market, and period.
