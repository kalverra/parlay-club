# Weather fixtures

| File | Source |
| --- | --- |
| `venues_2026.json` | Every unique venue from the 2026 regular-season scoreboards, weeks 1 through 18 (`GET .../scoreboard?seasontype=2&week={n}&dates=2026`), fetched 2026-11-07. Only repeated events were removed; no fields were edited. |
| `forecast_lumen.json` | Open-Meteo forecast for Lumen Field (`latitude=47.59535&longitude=-122.33164`) across the week 9 kickoff window, `start_hour=2026-11-08T21:00&end_hour=2026-11-08T23:00`, fetched 2026-11-07. Unedited. |
| `forecast_highmark.json` | Open-Meteo forecast for Highmark Stadium (`latitude=42.77303&longitude=-78.79219`) across the week 9 Monday kickoff window, `start_hour=2026-11-10T01:00&end_hour=2026-11-10T03:00`, fetched 2026-11-07. Unedited. |
| `forecast_short_window.json` | Open-Meteo forecast for Lumen Field (`latitude=47.59535&longitude=-122.33164`) over two hours only, `start_hour=2026-11-08T22:00&end_hour=2026-11-08T23:00`, fetched 2026-11-07. Unedited; it proves a short window is an error. |
