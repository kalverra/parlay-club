# ESPN fixtures

Real scoreboard, roster, gamelog, and summary responses from ESPN's public site API, saved while building the client.

| File | Source |
| --- | --- |
| `scoreboard_week.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard` on 2026-11-07. Week 9, 15 events including the Thursday game and the Madrid game. |
| `scoreboard_week9_live.json` | **Derived** from `scoreboard_week.json` for the sandbox's early-loss stage: the Madrid game is in the third quarter at ATL 17, CIN 10, and every other slate game is still scheduled. |
| `scoreboard_week9_final_sunday.json` | **Derived** from `scoreboard_week.json` for the sandbox's all-clear stage: every Sunday game is final with a scripted score, while the Monday night game is still scheduled. |
| `scoreboard_week9_complete.json` | **Derived** from `scoreboard_week.json` for the sandbox's Sunday-complete stage: every Sunday and Monday game is final with a scripted score. |
| `scoreboard_live.json` | The same endpoint, captured from a web archive snapshot at 2026-10-04 15:18 ET during the week 4 Sunday afternoon games: 8 live, 1 final, 6 scheduled, plus the Thursday game. |
| `scoreboard_international.json` | `GET .../scoreboard?seasontype=2&week=5&dates=2026`, trimmed to the London game. |
| `roster.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/2/roster` on 2026-11-09. Buffalo Bills roster. |
| `roster_falcons.json` | The same endpoint for team `1` on 2026-11-11. Atlanta Falcons roster, the sandbox featured game's home side. |
| `roster_bengals.json` | The same endpoint for team `4` on 2026-11-11. Cincinnati Bengals roster, the sandbox featured game's away side and the team its prop board resolves against. |
| `gamelog.json` | `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/3918298/gamelog` on 2026-11-09. Josh Allen regular season game log. |
| `gamelog_kicker.json` | `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/3917232/gamelog` on 2026-11-09. Tyler Bass game log for kicking stats. |
| `gamelog_burrow.json` | The same endpoint for athlete `3915511` on 2026-11-11. Joe Burrow game log, behind the sandbox featured prop board. |
| `gamelog_chase.json` | The same endpoint for athlete `4362628` on 2026-11-11. Ja'Marr Chase game log, behind the sandbox featured prop board. |
| `gamelog_higgins.json` | The same endpoint for athlete `4239993` on 2026-11-11. Tee Higgins game log, behind the sandbox featured prop board. |
| `gamelog_mcpherson.json` | The same endpoint for athlete `4360234` on 2026-11-11. Evan McPherson game log, behind the sandbox featured prop board. |
| `summary.json` | `GET https://site.api.espn.com/apis/site/v2/sports/football/nfl/summary?event=401873051` on 2026-11-09. Buffalo Bills at Minnesota Vikings summary with injury reports. |
| `summary_week9_featured.json` | The same endpoint for `event=401873038` on 2026-11-11. Cincinnati Bengals at Atlanta Falcons summary, the sandbox featured game's injury report. |
| `roster_empty.json` | **Derived** empty roster (`{"athletes":[]}`) the sandbox replays for teams outside the featured game. |
| `summary_empty.json` | **Derived** empty summary (`{}`) the sandbox replays for games outside the featured game. |
| `gamelog_empty.json` | **Derived** empty game log (`{}`) the sandbox replays for athletes outside the featured prop board. |
