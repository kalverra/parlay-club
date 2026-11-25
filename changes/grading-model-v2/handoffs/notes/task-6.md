# Task 6

## Done

- Updated `docs/reference/schema.md` so the `nflverse_player_weeks` table summary, DDL block (`exp_pass_yds`,
  `exp_rush_yds`, `exp_rec_yds`, `exp_receptions`, `exp_pass_tds`, `exp_rush_tds`, `exp_rec_tds`, `carry_share`, and
  `cpoe`), and conventions match migration `0019_v2_opportunity_signals.sql`.
- Updated `docs/reference/data-sources.md` to document `DefaultFFOpportunityBaseURL`, optional-column handling
  (`passing_cpoe` and `season_type`), week-based postseason fallback (`week > 18` since 2021, `> 17` through 2020),
  blank/`NA` defaults, `carry_share` and `cpoe` persistence during refresh, and the `ep_weekly_2026.csv` test fixture.
- Verified documentation with `good-vibes docs lint` and `rumdl check`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 3 (`internal/modelv2`) can rely on `docs/reference/schema.md` and `docs/reference/data-sources.md` as accurate
  contracts for the `nflverse_player_weeks` schema and `ff_opportunity` / `passing_cpoe` ingestion semantics.

## Commit

Document ff_opportunity and migration 0019 schema

Keeps the data-source and SQLite schema reference docs aligned with the new expected-opportunity ingestion path, carry_share calculation, and passing_cpoe column added in Phase 2 so downstream model work has an accurate source of truth.
