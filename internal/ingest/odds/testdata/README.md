# Odds API fixtures

Real responses from The Odds API, saved while building the odds client. Only whole events were removed; no fields were
edited. The event list and the game lines were captured in the same minute, so their event IDs line up.

| File | Source |
| --- | --- |
| `events_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events`, fetched 2026-11-07. 27 events across weeks 9 and 10, including the Monday night game. Costs 0 credits; the response reported `x-requests-remaining: 500`. |
| `odds_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american`, fetched 2026-11-07. The same 27 events; 204 bookmaker blocks and 1,162 priced selections. Costs 3 credits; the response reported `x-requests-remaining: 497`. |
| `odds_game_links.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events/dc56d980f527ffeef89940133b211b13/odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american&includeLinks=true&includeSids=true`, fetched 2026-11-07. Bengals at Falcons. Costs 3 credits; the response reported `x-requests-last: 3` and `x-requests-remaining: 491`. Every DraftKings and FanDuel outcome carries `link` and `sid`; other books vary. |
| `odds_prop_links.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events/dc56d980f527ffeef89940133b211b13/odds?regions=us&markets=player_pass_yds&oddsFormat=american&includeLinks=true&includeSids=true`, fetched 2026-11-07. The same game. Costs 1 credit; the response reported `x-requests-last: 1` and `x-requests-remaining: 489`. DraftKings and FanDuel returned `link` and `sid` on all four outcomes each. |

Bookmakers in the saved response include `draftkings`, `fanduel`, `betmgm`, `betrivers`, `betus`, `betonlineag`,
`bovada`, `lowvig`, and `mybookieag`. Some books omit the moneyline on future games, which the client keeps as-is: a
market that is absent produces no rows. The two `*_links.json` files keep the full per-event response. Only whole events
were removed; no fields were edited. They show that the flags add no credits and that a missing `link` is normal for
some books.

The per-event odds endpoint returns one event object, while the all-events endpoint returns an array.
