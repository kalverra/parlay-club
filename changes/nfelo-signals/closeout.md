# Close-out

## Consequences

The analyzer now has independent model context—projected spreads, win probabilities, and quarterback ratings—without
spending Odds API credits. Friends see friendlier, sharper advice on slips: `hook` explains whether the model agrees
with key numbers, `chalk` gives win probability on heavy moneyline favorites, and `qb_drag` highlights when a backup
quarterback downgrades an offense. Slip views show model projections on spread and moneyline legs.

When nfelo sync fails, is missing, or is older than 72 hours, all model signals silently deactivate without erroring or
blocking.

Maintenance requires watching team code mappings between nfelo and ESPN (`OAK`→`LV`, `WAS`→`WSH`). Because stale or
unmatched data turns signals off silently, maintainers must rely on sync logs and archive history rather than UI
warnings to detect upstream feed issues.

We would revisit batching `NfeloForGame` lookups if per-game queries slow slip rendering.

## Alternatives considered

- **Team-stat sources (e.g. nflverse EPA):** Rejected for now. EPA measures opponent strength for props rather than
  spread and win-probability sanity checks, and requires heavier ingest pipelines.
- **RBSDM / CrazyNinjaOdds:** Rejected because RBSDM provides no machine-readable export, and CrazyNinjaOdds lacks an
  NFL data export. nfelo offers clean, public CSV files.
- **Custom ratings and Elo modeling:** Rejected because building and calibrating custom models takes substantial effort
  for weaker signals than established models provide.
- **Scraping nfeloapp.com:** Rejected in favor of the public GitHub CSV repository to avoid brittle HTML scraping.
- **Reading `output_data/projected_spreads.csv` or `nfelo_games.csv`:** Rejected because `projected_spreads.csv` is
  absent on `main`, and `nfelo_games.csv` omits team abbreviations and quarterback names.
- **Passing nfelo via the `stats` map:** Rejected because `stats` is keyed per-player while nfelo projections apply
  per-game.
- **Dynamic CSV fetch on render:** Rejected to keep slip rendering pure, offline-capable, and fast without external
  network dependencies.
- **New analyzer rules for every signal:** Rejected in favor of augmenting `hook` and `chalk` to keep the warning banner
  concise.

## Implementation notes

The design expected team-code checks against the weather venue map, but that map contains coordinates and venue IDs
instead of team codes. The test validates against the 32 ESPN abbreviations directly.

The CSV fixture contains 1,551 rows through 2026 week 9 (1,449 with complete quarterback cells). Blank quarterback cells
drop that row as designed. The sample week 9 game in the design (BUF vs MIA) was illustrative; actual 2026 week 9 data
featured MIN vs BUF (Carson Wentz vs Josh Allen), providing a real test case for `qb_drag`.

Maintainer limits:

- `qb_drag` uses static pre-game quarterback adjustments (threshold −30 Elo, ~1.2 points). It cannot detect game-day
  scratch announcements unless upstream files update.
- Freshness checks read `sync_state` for `nfelo:projections`, not `nfelo_games.updated_at` (which only bumps on value
  changes). Sync errors or data over 72 hours old silently silence all model output.
- nfelo's repository lacks an open source license ("All rights reserved"). A person should ask the author for permission
  before shipping.
- The 0.6 MB CSV fetches every 12 hours. Archive storage should be verified against runbooks after the first week.

## Follow-ups

- **Batch nfelo lookups in slip views:** Replace individual `db.NfeloForGame` calls per game with a single batch query
  if slip rendering profiles show database contention.
- **Seek author permission for nfelo CSV usage:** Contact the nfelo maintainer for explicit confirmation of automated
  CSV ingest for private club use.
- **Archive size check:** Monitor SQLite raw response archive growth after seven days of automated 12-hour syncs.
