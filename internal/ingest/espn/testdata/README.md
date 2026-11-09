# ESPN fixtures

Real scoreboard, roster, gamelog, and summary responses from ESPN's public site API, saved while building the client.

| File | Source |
| --- | --- |
| `scoreboard_week.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard` on 2026-11-07. Week 9, 15 events including the Thursday game and the Madrid game. |
| `scoreboard_live.json` | The same endpoint, captured from a web archive snapshot at 2026-10-04 15:18 ET during the week 4 Sunday afternoon games: 8 live, 1 final, 6 scheduled, plus the Thursday game. |
| `scoreboard_international.json` | `GET .../scoreboard?seasontype=2&week=5&dates=2026`, trimmed to the London game. |
| `roster.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/2/roster` on 2026-11-09. Buffalo Bills roster. |
| `gamelog.json` | `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/3918298/gamelog` on 2026-11-09. Josh Allen regular season game log. |
| `gamelog_kicker.json` | `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/3917232/gamelog` on 2026-11-09. Tyler Bass game log for kicking stats. |
| `summary.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/summary?event=401873051` on 2026-11-09. Buffalo Bills at Minnesota Vikings summary with injury reports. |

