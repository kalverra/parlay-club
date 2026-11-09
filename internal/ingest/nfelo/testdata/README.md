# nfelo fixtures

| File | Source |
| --- | --- |
| `historic_projected_spreads.csv` | `GET https://raw.githubusercontent.com/greerreNFL/nfelo/main/output_data/historic_projected_spreads.csv` fetched 2026-11-09. The real file, unedited: 1,551 data rows from 2021 through 2026 week 9. |

The first column is pandas's nameless row index. The client ignores it and reads
the rest by header name.
