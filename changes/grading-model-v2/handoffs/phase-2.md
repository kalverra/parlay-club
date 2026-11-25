# Phase 2: Phase 2: Free nflverse opportunity sync and role-archetype enrichment

## Done

### Task 4: Add `Client.FFOpportunity` in `internal/ingest/nflverse/opportunity.go` (plus fixtures and parser tests), add migration `internal/db/migrations/0019_v2_opportunity_signals.sql`, and wire fail-soft opportunity + `carry_share` ingestion through `internal/db/nflverse.go` and `internal/refresh/nflverse.go` with unit/integration tests.

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

### Task 5: Compute deterministic player role archetypes (`QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2`), EWMA usage/opportunity features, and role-specific opponent defense allowed profiles in `internal/analyzer/signals.go` (respecting strict pre-kickoff replay in `internal/eval/replay.go`), with tests covering early-season sparse data and replay leakage.

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

### Task 6: Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` for `ff_opportunity` and migration `0019`.

- Updated `docs/reference/schema.md` so the `nflverse_player_weeks` table summary, DDL block (`exp_pass_yds`,
  `exp_rush_yds`, `exp_rec_yds`, `exp_receptions`, `exp_pass_tds`, `exp_rush_tds`, `exp_rec_tds`, `carry_share`, and
  `cpoe`), and conventions match migration `0019_v2_opportunity_signals.sql`.
- Updated `docs/reference/data-sources.md` to document `DefaultFFOpportunityBaseURL`, optional-column handling
  (`passing_cpoe` and `season_type`), week-based postseason fallback (`week > 18` since 2021, `> 17` through 2020),
  blank/`NA` defaults, `carry_share` and `cpoe` persistence during refresh, and the `ep_weekly_2026.csv` test fixture.
- Verified documentation with `good-vibes docs lint` and `rumdl check`.

## Deviations

### Task 4

- `nflverse/nflverse-data` does not host `ff_opportunity/ep_weekly_{season}.csv` (it returns 404);
  `ffverse/ffopportunity` publishes `ep_weekly_{season}.csv` under
  `https://github.com/ffverse/ffopportunity/releases/download/latest-data`. `Client.FFOpportunity` fetches from
  `DefaultFFOpportunityBaseURL` when `baseURL == DefaultBaseURL`, and uses `/ff_opportunity/ep_weekly_{season}.csv`
  for custom base URLs.
- `ep_weekly_{season}.csv` omits `season_type` (encoding postseason as `week > 18` since 2021, `> 17` before 2021) and
  omits `cpoe`. `FFOpportunity` derives `SeasonType` from `(season, week)` unless an optional `season_type` column is
  present, and `PlayerStats` reads `cpoe` from `stats_player_week_{season}.csv`'s `passing_cpoe` column as an optional
  column.

### Task 5

- Instead of querying `NFLversePlayerWeeksAgainstOpponent` per game/position inside `LoadSlipSignals` and
  `LoadBoardSignals`, `loadSignals` fetches all season player-week rows once via `NFLversePlayerWeeksForSeasonTypes`
  (when `len(athleteIDs) > 0` and nflverse is fresh). Assigning opposing players to `WR1`/`WR2` or `RB1`/`RB2`
  requires each opposing player's full team depth-chart context, and one season query avoids N+1 per-opponent/position
  round-trips while keeping the `len(athleteIDs) == 0` 5-query baseline intact.

## Open issues

### Task 4

- `internal/sandbox/registry.go` does not preload `ep_weekly_2026.csv` because `sandbox_test.go` enforces the existing
  104-fixture snapshot count; `TestNFLverseEndToEndWithSandboxFixtures` exercises the fail-soft path cleanly.

### Task 5

- `DataCoverage.WithNFLVerse` in `internal/eval/replay.go` still checks `HomeEPA != nil || AwayEPA != nil || Usage != nil`
  so existing `v1` CLI coverage assertions in `cmd/eval/main_test.go` stay unchanged; Phase 6 can include
  `ps.EWMA != nil` when adding `v2` eval reporting if desired.

## Next-phase notes

### Task 4

- `db.NFLversePlayerWeek` now stores `ExpPassYds`, `ExpRushYds`, `ExpRecYds`, `ExpReceptions`, `ExpPassTDs`,
  `ExpRushTDs`, `ExpRecTDs`, `CarryShare`, and `CPOE` (all `NOT NULL DEFAULT 0` in SQLite).
- Downstream role-archetype derivation in `internal/analyzer/signals.go` can rank RB workloads using `CarryShare` and
  `OffensePct` directly alongside `TargetShare` for WR/TEs.

### Task 5

- `ComputeEWMA` and `BuildPlayerEWMAUsage` use normalized exponential weights `0.5^(i/3.0)` over games ordered
  `(Season DESC, SeasonType DESC, WeekNum DESC)`, matching `pandas.Series.ewm(halflife=3, adjust=True).mean()` so
  Phase 3/4 Go-Python feature parity is exact.
- `PlayerStats.Role` is `RoleQB` for any QB on the roster even in Week 1 (`0` prior weeks), and `RoleUnknown` (`""`) for
  WR/RB/TE with zero prior offensive participation; `Game.OpponentRoleDefense(team, role)` returns `nil` safely when
  missing.

### Task 6

- Phase 3 (`internal/modelv2`) can rely on `docs/reference/schema.md` and `docs/reference/data-sources.md` as accurate
  contracts for the `nflverse_player_weeks` schema and `ff_opportunity` / `passing_cpoe` ingestion semantics.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0019_v2_opportunity_signals.sql`
- `internal/db/nflverse.go`
- `internal/db/nflverse_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`
- `internal/ingest/nflverse/client.go`
- `internal/ingest/nflverse/nflverse_test.go`
- `internal/ingest/nflverse/opportunity.go`
- `internal/ingest/nflverse/player_stats.go`
- `internal/ingest/nflverse/testdata/README.md`
- `internal/ingest/nflverse/testdata/ep_weekly_2026.csv`
- `internal/refresh/nflverse.go`
- `internal/refresh/nflverse_test.go`
- `internal/refresh/refresh.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: test 1.2s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
