# Weather fixtures

| File | Source |
| --- | --- |
| `venues_2026.json` | Every unique venue from the 2026 regular-season scoreboards, weeks 1 through 18 (`GET .../scoreboard?seasontype=2&week={n}&dates=2026`), fetched 2026-11-07. Only repeated events were removed; no fields were edited. |
| `forecast_lumen.json` | Open-Meteo forecast for Lumen Field (`latitude=47.59535&longitude=-122.33164`) across the week 9 kickoff window, `start_hour=2026-11-08T21:00&end_hour=2026-11-08T23:00`, fetched 2026-11-07. Unedited. |
| `forecast_highmark.json` | Open-Meteo forecast for Highmark Stadium (`latitude=42.77303&longitude=-78.79219`) across the week 9 Monday kickoff window, `start_hour=2026-11-10T01:00&end_hour=2026-11-10T03:00`, fetched 2026-11-07. Unedited. |
| `forecast_short_window.json` | Open-Meteo forecast for Lumen Field (`latitude=47.59535&longitude=-122.33164`) over two hours only, `start_hour=2026-11-08T22:00&end_hour=2026-11-08T23:00`, fetched 2026-11-07. Unedited; it proves a short window is an error. |
| `forecast_arrowhead.json` | **Derived.** Week 9 kickoff window for Arrowhead Stadium, `2026-11-08T18:00` through `20:00`. Synthesized for the local sandbox because the source no longer serves this forecast. |
| `forecast_hard_rock.json` | **Derived.** Week 9 kickoff window for Hard Rock Stadium, `2026-11-08T18:00` through `20:00`. Synthesized; it carries the only nonzero precipitation in the set. |
| `forecast_lincoln_financial.json` | **Derived.** Week 9 kickoff window for Lincoln Financial Field, `2026-11-08T18:00` through `20:00`. Synthesized. |
| `forecast_northwest.json` | **Derived.** Week 9 kickoff window for Northwest Stadium, `2026-11-08T18:00` through `20:00`. Synthesized. |
| `forecast_bank_of_america.json` | **Derived.** Week 9 kickoff window for Bank of America Stadium, `2026-11-08T18:00` through `20:00`. Synthesized. |
| `forecast_sofi.json` | **Derived.** Week 9 kickoff window for SoFi Stadium, `2026-11-08T21:00` through `23:00`. Synthesized. |
| `forecast_gillette.json` | **Derived.** Week 9 kickoff window for Gillette Stadium, `2026-11-08T21:00` through `23:00`. Synthesized. |
| `forecast_levis.json` | **Derived.** Week 9 kickoff window for Levi's Stadium, `2026-11-08T21:00` through `23:00`. Synthesized. |
| `forecast_soldier.json` | **Derived.** Week 9 kickoff window for Soldier Field, `2026-11-09T01:00` through `03:00`. Synthesized. |

The derived forecasts fill every outdoor game in the week 9 Sunday morning refresh. They use Open-Meteo's response shape and
three-hour kickoff window, but they are not captured provider data. The sandbox registry in `internal/sandbox` is the only
consumer; the weather client tests still use the captured Lumen and Highmark fixtures.
