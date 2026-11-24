# Task 3

## Done

- Updated `docs/reference/schema.md` for migration `0015_research_context.sql` (schema version 15), adding `player_gamelogs.games_json` and the 12 new `nflverse_player_weeks` columns plus `idx_nflverse_player_weeks_opp_pos` to the summary, migrations list, table overview, DDL, and conventions.
- Updated `docs/reference/data-sources.md` to document `espn.GameLog.Entries` (decoded from `events` and stored in `player_gamelogs.games_json`) and the expanded `nflverse.PlayerStats` columns (`player_display_name`, `position` normalization, `opponent_team`, `attempts`, `fg_made`, `pat_made`, and `kicker_pts`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Schema and data-source reference docs now reflect `0015_research_context.sql`, `player_gamelogs.games_json`, and the expanded `nflverse_player_weeks` columns; Phase 2 can proceed with analyzer defense ranks and `PlayerResearchSignals` updates.

## Commit

Document schema 0015 and research context sources

Updates the schema and data-sources reference documentation so developers and subsequent phases have an accurate contract for `player_gamelogs.games_json` and the expanded `nflverse_player_weeks` matchup columns.
