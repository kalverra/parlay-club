---
slug: deepen-signal-loader
run: 20261118-020037-deepen-signal-loader
outcome: done
generated: 2026-11-18T02:16:29Z
---

# Run summary: deepen-signal-loader

## Outcome

- Status: done
- Branch: deepen-signal-loader
- Close-out: drafted
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 2
- Checkpoints: 3
- Duration: 10m18s
- Cost: $—
- Fresh input tokens: 0
- Cache read tokens: 0
- Cache write tokens: 0
- Output tokens: 0

## Phases

### Phase 1: Batch `NfeloForGames` and move signal profile builders into `internal/analyzer`

Tasks:

- [x] Add `Store.NfeloForGames` in `internal/db/nfelo.go` (with tests in `internal/db/nfelo_test.go`) and create `internal/analyzer/signals.go` with `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`, moving unit tests from `internal/web/analyzer_internal_test.go` into `internal/analyzer`.

Phase totals: 1 checkpoints, 4m24s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `Store.NfeloForGames` in `internal/db/nfelo.go` (with tests in `internal/db/nfelo_test.go`) and create `internal/analyzer/signals.go` with `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`, moving unit tests from `internal/web/analyzer_internal_test.go` into `internal/analyzer`.

- Added `Store.NfeloForGames` in `internal/db/nfelo.go` using a single `IN (...)` query and shared `scanNfeloGame` helper, with empty/partial/full tests in `internal/db/nfelo_test.go`.
- Extended `analyzer.PlayerUsage` in `internal/analyzer/analyzer.go` with `Last3WOPR` and `AvgCarries`.
- Created `internal/analyzer/signals.go` with freshness constants (`NfeloSource`, `NfeloFreshFor`, `NFLverseFreshFor`, `NFLverseSource`), `LoadFreshTeamProfiles`, `LoadPlayerUsageForIDs`, `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`.
- Delegated `internal/web/analyzer.go` to `internal/analyzer` helpers and `Store.NfeloForGames`, removing the duplicated profile/usage math from `package web`.
- Moved `TestPostseasonNFLverseMergesRegularAndPostseason` from `internal/web/analyzer_internal_test.go` into `internal/analyzer/signals_test.go` and added unit tests for `BuildTeamDefenseProfiles`, `BuildPlayerUsage`, `SummarizeRecentUsage`, `GameFromDB`, and `LegFromDB`.

### Phase 2: Wire `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals` into `web` and `board`

Tasks:

- [x] Add `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, update `internal/web/analyzer.go`, `internal/web/slips.go`, `internal/web/pick.go`, `internal/web/research.go`, and `internal/board/signals.go` to use them, and verify with tests in `internal/analyzer` and `internal/web`.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` if any package-level references name where signal aggregation lives.

Phase totals: 2 checkpoints, 5m5s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Add `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, update `internal/web/analyzer.go`, `internal/web/slips.go`, `internal/web/pick.go`, `internal/web/research.go`, and `internal/board/signals.go` to use them, and verify with tests in `internal/analyzer` and `internal/web`.

- Added `LoadSlipSignals`, `LoadGameSignals`, `PlayerResearchSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, centralizing 72h `nfelo` freshness, 7d `nflverse` freshness, team defense profiles, player usage, and `player_gamelogs` JSON decoding.
- Added fallback-aware `PlayerUsage` accessor methods (`EffectiveTargetShare`, `EffectiveSnapAvg`, `EffectiveSeasonSnapAvg`, `RecentWOPR`, `AverageCarries`) in `internal/analyzer/analyzer.go` and updated `internal/board/signals.go` to use them, removing duplicate fallback loops from `internal/board`.
- Updated `internal/web/slips.go` (`slipPageData`), `internal/web/pick.go` (`dockFrom`, `boardSignalsForGame`), and `internal/web/research.go` (`populateNFLverseResearch`) to call the new `analyzer` signal loaders, and removed the obsolete wrappers from `internal/web/analyzer.go`.
- Added unit tests for `LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`, staleness cutoffs, and `PlayerUsage` accessor fallbacks in `internal/analyzer/signals_test.go`.

### Task 3: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` if any package-level references name where signal aggregation lives.

- Updated `docs/concepts/analyzer.md` to reflect that `internal/analyzer` (`LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`) now owns signal loading, freshness checks, team EPA aggregation, and player usage aggregation, while `AnalyzeSlip`, `ScoreLeg`, and `ScoreSlip` remain pure Go.
- Updated `docs/reference/analyzer-rules.md` (`How rules see a slip`) to point model, player stats, and `nflverse` EPA/usage loading references to `internal/analyzer` (`LoadSlipSignals`).

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
