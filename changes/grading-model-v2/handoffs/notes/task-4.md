# Task 4

## Done

- Added `Client.FFOpportunity` and `PlayerOpportunity` in `internal/ingest/nflverse/opportunity.go` (plus
  `DefaultFFOpportunityBaseURL`, optional-column CSV support in `client.go`, optional `passing_cpoe` parsing in
  `player_stats.go`, week-9 `ep_weekly_2026.csv` fixture, and parser tests).
- Added migration `internal/db/migrations/0019_v2_opportunity_signals.sql` adding `exp_pass_yds`, `exp_rush_yds`,
  `exp_rec_yds`, `exp_receptions`, `exp_pass_tds`, `exp_rush_tds`, `exp_rec_tds`, `carry_share`, and `cpoe` to
  `nflverse_player_weeks`, and updated `migrate_test.go` and `migrate_internal_test.go`.
- Extended `db.NFLversePlayerWeek`, `UpsertNFLversePlayerWeeks`, and `scanNFLversePlayerWeeks` in
  `internal/db/nflverse.go` with round-trip and idempotency tests in `internal/db/nflverse_test.go`.
- Added `FFOpportunity` to `refresh.NFLverseClient` and wired fail-soft `FFOpportunity` ingestion, `carry_share`
  (`carries / team_carries`), and `cpoe` (`passing_cpoe`) through `internal/refresh/nflverse.go` with unit and fail-soft
  tests in `internal/refresh/nflverse_test.go`.
- Updated `docs/reference/schema.md`, `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, and
  `changes/grading-model-v2/design.md` to reflect migration 19 and the upstream `ffverse/ffopportunity` endpoint.

## Deviations

- `nflverse/nflverse-data` does not host `ff_opportunity/ep_weekly_{season}.csv` (it returns 404);
  `ffverse/ffopportunity` publishes `ep_weekly_{season}.csv` under
  `https://github.com/ffverse/ffopportunity/releases/download/latest-data`. `Client.FFOpportunity` fetches from
  `DefaultFFOpportunityBaseURL` when `baseURL == DefaultBaseURL`, and uses `/ff_opportunity/ep_weekly_{season}.csv`
  for custom base URLs.
- `ep_weekly_{season}.csv` omits `season_type` (encoding postseason as `week > 18` since 2021, `> 17` before 2021) and
  omits `cpoe`. `FFOpportunity` derives `SeasonType` from `(season, week)` unless an optional `season_type` column is
  present, and `PlayerStats` reads `cpoe` from `stats_player_week_{season}.csv`'s `passing_cpoe` column as an optional
  column.

## Open issues

- `internal/sandbox/registry.go` does not preload `ep_weekly_2026.csv` because `sandbox_test.go` enforces the existing
  104-fixture snapshot count; `TestNFLverseEndToEndWithSandboxFixtures` exercises the fail-soft path cleanly.

## Next-phase notes

- `db.NFLversePlayerWeek` now stores `ExpPassYds`, `ExpRushYds`, `ExpRecYds`, `ExpReceptions`, `ExpPassTDs`,
  `ExpRushTDs`, `ExpRecTDs`, `CarryShare`, and `CPOE` (all `NOT NULL DEFAULT 0` in SQLite).
- Downstream role-archetype derivation in `internal/analyzer/signals.go` can rank RB workloads using `CarryShare` and
  `OffensePct` directly alongside `TargetShare` for WR/TEs.

## Commit

Ingest nflverse expected opportunity and carry share

The v2 model needs expected yards, receptions, touchdowns, carry share, and CPOE on weekly player rows to separate role volume from noisy box-score outcomes. Pulling expected opportunity from ffverse/ffopportunity fail-soft keeps the core nflverse sync resilient when upstream opportunity files lag or fail.
