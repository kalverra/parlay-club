# Odds API fixtures

Real responses from The Odds API, saved while building the odds client. Only whole events were removed; no fields were
edited. The event list and the game lines were captured in the same minute, so their event IDs line up.

| File | Source |
| --- | --- |
| `events_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/events`, fetched 2026-11-07. 27 events across weeks 9 and 10, including the Monday night game. Costs 0 credits; the response reported `x-requests-remaining: 500`. |
| `odds_nfl.json` | `GET https://api.the-odds-api.com/v4/sports/americanfootball_nfl/odds?regions=us&markets=h2h,spreads,totals&oddsFormat=american`, fetched 2026-11-07. The same 27 events; 204 bookmaker blocks and 1,162 priced selections. Costs 3 credits; the response reported `x-requests-remaining: 497`. |

Bookmakers in the saved response include `draftkings`, `fanduel`, `betmgm`, `betrivers`, `betus`, `betonlineag`,
`bovada`, `lowvig`, and `mybookieag`. Some books omit the moneyline on future games, which the client keeps as-is: a
market that is absent produces no rows.
