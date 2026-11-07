# ESPN fixtures

Real scoreboard responses from ESPN's public site API, saved while building the client. Only whole events were removed;
no fields were edited.

| File | Source |
| --- | --- |
| `scoreboard_week.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard` on 2026-11-07. Week 9, 15 events including the Thursday game and the Madrid game. |
| `scoreboard_live.json` | The same endpoint, captured from a web archive snapshot at 2026-10-04 15:18 ET during the week 4 Sunday afternoon games: 8 live, 1 final, 6 scheduled, plus the Thursday game. |
| `scoreboard_international.json` | `GET .../scoreboard?seasontype=2&week=5&dates=2026`, trimmed to the London game. |
