# Task 4

## Done

- Added `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile` and computed/ranked them in `BuildTeamDefenseProfiles` (`internal/analyzer/analyzer.go`, `internal/analyzer/signals.go`).
- Implemented `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` (with `OppDefenseGame`) in `internal/analyzer/signals.go`, supporting position normalization (`FB`/`HB` → `RB`), primary-market defaults, tie-breaking, `<5` games, and zero-value games.
- Expanded `PlayerResearchSignals` and `LoadPlayerResearchSignals` with `TeamDefProfiles`, `PosDefRanks`, `OppOverallDefRank`, `OppPosDefRank`, `OppDefenseL5`, and position-relevant L3 usage averages (`CarriesPerGame`, `TargetsPerGame`, `PassAttPerGame`, `AirYardsShare`, `RZOppsPerGame`, `EPAPerGame`).
- Added unit tests in `internal/analyzer/signals_test.go` covering overall defense ranks, position defense ranks, opposing defense L5 history, `<5` games, ties, `FB`→`RB` normalization, and stale nflverse fallback.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `PlayerResearchSignals` now exposes `TeamDefProfiles`, `PosDefRanks`, `OppOverallDefRank`, `OppPosDefRank`, `OppDefenseL5`, and L3 per-game usage averages (`CarriesPerGame`, `TargetsPerGame`, `PassAttPerGame`, `AirYardsShare`, `RZOppsPerGame`, `EPAPerGame`) for `internal/web/research.go` in Phase 3.

## Commit

Add overall/position defense ranks and opp L5 signals

Player research needs league-wide overall and position-specific defense
rankings alongside the opposing defense's last five games at that position.
Computing these in `internal/analyzer` keeps all matchup ranking and L3
usage aggregation deterministic and ready for the web research view.
