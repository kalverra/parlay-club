# Phase 2: Phase 2: Compute defense ranks, opponent L5 history, and market-specific O/U lookup

## Done

### Task 4: Add `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile`, implement `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` in `internal/analyzer/signals.go`, and expand `LoadPlayerResearchSignals` with defense rank maps, opposing defense L5 games, and position-relevant L3 usage averages, with unit tests covering edge cases (`<5` games, ties, `FB`→`RB`, stale nflverse).

- Added `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile` and computed/ranked them in `BuildTeamDefenseProfiles` (`internal/analyzer/analyzer.go`, `internal/analyzer/signals.go`).
- Implemented `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` (with `OppDefenseGame`) in `internal/analyzer/signals.go`, supporting position normalization (`FB`/`HB` → `RB`), primary-market defaults, tie-breaking, `<5` games, and zero-value games.
- Expanded `PlayerResearchSignals` and `LoadPlayerResearchSignals` with `TeamDefProfiles`, `PosDefRanks`, `OppOverallDefRank`, `OppPosDefRank`, `OppDefenseL5`, and position-relevant L3 usage averages (`CarriesPerGame`, `TargetsPerGame`, `PassAttPerGame`, `AirYardsShare`, `RZOppsPerGame`, `EPAPerGame`).
- Added unit tests in `internal/analyzer/signals_test.go` covering overall defense ranks, position defense ranks, opposing defense L5 history, `<5` games, ties, `FB`→`RB` normalization, and stale nflverse fallback.

### Task 5: Add `Store.LatestPropForPlayerMarket` in `internal/db/odds.go` so player research resolves the current week's O/U line for a specific market without bleeding lines across market tabs, with unit tests in `internal/db/players_test.go`.

- Added `Store.LatestPropForPlayerMarket(ctx, athleteID, playerName, team, market)` in `internal/db/odds.go` to return the newest prop odds row with a non-nil line filtered by `market_type` when `market` is non-empty.
- Delegated `Store.LatestPropForPlayer` to `LatestPropForPlayerMarket(..., "")` so candidate resolution and query logic stay in one place.
- Added `TestLatestPropForPlayerMarket` in `internal/db/players_test.go` covering empty-market fallback, market-specific filtering without cross-market bleed, newest-row selection, `player_matches` lookup, nil-line exclusion, and team filtering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

- `PlayerResearchSignals` now exposes `TeamDefProfiles`, `PosDefRanks`, `OppOverallDefRank`, `OppPosDefRank`, `OppDefenseL5`, and L3 per-game usage averages (`CarriesPerGame`, `TargetsPerGame`, `PassAttPerGame`, `AirYardsShare`, `RZOppsPerGame`, `EPAPerGame`) for `internal/web/research.go` in Phase 3.

### Task 5

- Phase 3 (`internal/web/research.go`) can switch to `Store.LatestPropForPlayerMarket(ctx, athleteID, player.FullName, player.Team, activeMarket)` and drop `&line=...` from `buildMarketOptions` tab links so each market tab resolves its own O/U line.

## Files touched

- `internal/analyzer/analyzer.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/db/odds.go`
- `internal/db/players_test.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: generate 400ms, lint 6.7s, test 18.3s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
