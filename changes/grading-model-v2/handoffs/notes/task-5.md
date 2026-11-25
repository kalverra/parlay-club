# Task 5

## Done

- Added `RoleArchetype` (`QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2`), `PlayerEWMAUsage`, `PlayerRoleProfile`,
  `RoleDefenseProfile`, `Game.OpponentRoleDefense`, and `PlayerStats.Role` / `PlayerStats.EWMA` in
  `internal/analyzer/analyzer.go`.
- Implemented `ComputeEWMA`, `BuildPlayerEWMAUsage` / `BuildPlayerEWMAUsageWithHalfLife` (normalized `adjust=True`
  with 3-week half-life), `BuildPlayerRoleProfiles`, and `BuildOppRoleDefenseProfiles` in
  `internal/analyzer/signals.go`, and wired them into `LoadSlipSignals` / `LoadBoardSignals` via a single
  `Store.NFLversePlayerWeeksForSeasonTypes` query in `internal/db/nflverse.go` when player legs are present.
- Wired strict pre-kickoff replay of role archetypes, EWMA usage/opportunity features, and role-specific opponent
  defense profiles into `internal/eval/replay.go` (`loadPriorNFLverse`, `populatePriorNFLversePlayers`,
  `buildBasePreKickoffGame`, `loadArchivedPlayerStatsForAthletes`), strictly excluding same-week and future rows.
- Added unit and integration tests in `internal/db/nflverse_test.go`, `internal/analyzer/signals_test.go`, and
  `internal/eval/replay_test.go` covering sparse 1-week/0-week data, tie-breaking, permutation determinism, and
  replay leakage guards.

## Deviations

- Instead of querying `NFLversePlayerWeeksAgainstOpponent` per game/position inside `LoadSlipSignals` and
  `LoadBoardSignals`, `loadSignals` fetches all season player-week rows once via `NFLversePlayerWeeksForSeasonTypes`
  (when `len(athleteIDs) > 0` and nflverse is fresh). Assigning opposing players to `WR1`/`WR2` or `RB1`/`RB2`
  requires each opposing player's full team depth-chart context, and one season query avoids N+1 per-opponent/position
  round-trips while keeping the `len(athleteIDs) == 0` 5-query baseline intact.

## Open issues

- `DataCoverage.WithNFLVerse` in `internal/eval/replay.go` still checks `HomeEPA != nil || AwayEPA != nil || Usage != nil`
  so existing `v1` CLI coverage assertions in `cmd/eval/main_test.go` stay unchanged; Phase 6 can include
  `ps.EWMA != nil` when adding `v2` eval reporting if desired.

## Next-phase notes

- `ComputeEWMA` and `BuildPlayerEWMAUsage` use normalized exponential weights `0.5^(i/3.0)` over games ordered
  `(Season DESC, SeasonType DESC, WeekNum DESC)`, matching `pandas.Series.ewm(halflife=3, adjust=True).mean()` so
  Phase 3/4 Go-Python feature parity is exact.
- `PlayerStats.Role` is `RoleQB` for any QB on the roster even in Week 1 (`0` prior weeks), and `RoleUnknown` (`""`) for
  WR/RB/TE with zero prior offensive participation; `Game.OpponentRoleDefense(team, role)` returns `nil` safely when
  missing.

## Commit

Derive role archetypes, EWMA, and role defense

The v2 grading model needs deterministic role tiers, sparse-week EWMA opportunity signals, and role-specific defense splits that work in Weeks 1-2 without breaking v1 rules or leaking future weeks during replay. Computing these once from trailing nflverse player-week rows keeps both live signal loading and historical evaluation consistent and leak-free.
