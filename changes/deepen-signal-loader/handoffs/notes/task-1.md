# Task 1

## Done

- Added `Store.NfeloForGames` in `internal/db/nfelo.go` using a single `IN (...)` query and shared `scanNfeloGame` helper, with empty/partial/full tests in `internal/db/nfelo_test.go`.
- Extended `analyzer.PlayerUsage` in `internal/analyzer/analyzer.go` with `Last3WOPR` and `AvgCarries`.
- Created `internal/analyzer/signals.go` with freshness constants (`NfeloSource`, `NfeloFreshFor`, `NFLverseFreshFor`, `NFLverseSource`), `LoadFreshTeamProfiles`, `LoadPlayerUsageForIDs`, `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`.
- Delegated `internal/web/analyzer.go` to `internal/analyzer` helpers and `Store.NfeloForGames`, removing the duplicated profile/usage math from `package web`.
- Moved `TestPostseasonNFLverseMergesRegularAndPostseason` from `internal/web/analyzer_internal_test.go` into `internal/analyzer/signals_test.go` and added unit tests for `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/web/analyzer.go` still has thin wrappers (`analyzerLeg`, `loadFreshNFLverseTeamProfiles`, `loadNFLversePlayerWeeks`, `nflverseUsageForPlayers`, `nflverseSeasonTypes`) so `internal/web/pick.go` and `internal/web/research.go` continue to compile unchanged until the next phases wire in `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals`.
- `internal/board/signals.go` has not yet been updated to read `PlayerUsage.Last3WOPR` and `PlayerUsage.AvgCarries`.
