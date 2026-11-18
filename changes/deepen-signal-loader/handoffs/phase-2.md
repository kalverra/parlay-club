# Phase 2: Phase 2: Wire `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals` into `web` and `board`

## Done

### Task 2: Add `LoadSlipSignals`, `LoadGameSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, update `internal/web/analyzer.go`, `internal/web/slips.go`, `internal/web/pick.go`, `internal/web/research.go`, and `internal/board/signals.go` to use them, and verify with tests in `internal/analyzer` and `internal/web`.

- Added `LoadSlipSignals`, `LoadGameSignals`, `PlayerResearchSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, centralizing 72h `nfelo` freshness, 7d `nflverse` freshness, team defense profiles, player usage, and `player_gamelogs` JSON decoding.
- Added fallback-aware `PlayerUsage` accessor methods (`EffectiveTargetShare`, `EffectiveSnapAvg`, `EffectiveSeasonSnapAvg`, `RecentWOPR`, `AverageCarries`) in `internal/analyzer/analyzer.go` and updated `internal/board/signals.go` to use them, removing duplicate fallback loops from `internal/board`.
- Updated `internal/web/slips.go` (`slipPageData`), `internal/web/pick.go` (`dockFrom`, `boardSignalsForGame`), and `internal/web/research.go` (`populateNFLverseResearch`) to call the new `analyzer` signal loaders, and removed the obsolete wrappers from `internal/web/analyzer.go`.
- Added unit tests for `LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`, staleness cutoffs, and `PlayerUsage` accessor fallbacks in `internal/analyzer/signals_test.go`.

### Task 3: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` if any package-level references name where signal aggregation lives.

- Updated `docs/concepts/analyzer.md` to reflect that `internal/analyzer` (`LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`) now owns signal loading, freshness checks, team EPA aggregation, and player usage aggregation, while `AnalyzeSlip`, `ScoreLeg`, and `ScoreSlip` remain pure Go.
- Updated `docs/reference/analyzer-rules.md` (`How rules see a slip`) to point model, player stats, and `nflverse` EPA/usage loading references to `internal/analyzer` (`LoadSlipSignals`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 2

- `internal/web/analyzer.go` now only handles view-model mapping (`AnalyzerView`, `GradeView`, `analyzerGuide`, `legModel`, `analyzerSeverity`) and `internal/analyzer` owns all signal loading and aggregation.

### Task 3

- Both `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` now match the Phase 2 signal-loading boundary in `internal/analyzer`.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/board/signals.go`
- `internal/web/analyzer.go`
- `internal/web/pick.go`
- `internal/web/research.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
