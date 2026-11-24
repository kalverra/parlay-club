## Done

- Extended `internal/ingest/nflverse/player_stats.go` (`PlayerWeek`, `playerStatsColumns`, `parsePlayerStatsRow`, `normalizePosition`) to parse `opponent_team` (mapped through `mapTeam`), `player_display_name` (falling back to `player_name`), `position` (normalizing `FB`/`HB` to `RB`), `attempts`, `fg_made`, and `pat_made`.
- Updated `internal/db/nflverse.go` (`NFLversePlayerWeek`, `nflversePlayerWeekColumns`, `UpsertNFLversePlayerWeeks`, `scanNFLversePlayerWeeks`) to persist and scan the 12 new opponent, player name, position, and passing/rushing/receiving/kicking stat columns while keeping unchanged rows' `updated_at` intact.
- Added `Store.NFLversePlayerWeeksAgainstOpponent` and `Store.NFLversePlayerWeeksForPositionTypes` in `internal/db/nflverse.go` with position normalization and multi-season-type ordering.
- Updated `internal/refresh/nflverse.go` (`buildNFLversePlayerWeeks`) to populate the new `db.NFLversePlayerWeek` fields (including `KickerPts = 3*FGMade + PATMade`) during nflverse sync.
- Updated and added fixture, store round-trip, and refresh sync tests in `internal/ingest/nflverse/nflverse_test.go`, `internal/db/nflverse_test.go`, and `internal/refresh/nflverse_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `NFLversePlayerWeeksAgainstOpponent` and `NFLversePlayerWeeksForPositionTypes` normalize `FB`/`HB` to `RB` on both write and query, and `buildNFLversePlayerWeeks` computes `KickerPts` as `3*FGMade + PATMade`.
