# Task 4

## Done

- Scoped `eval.LoadDataset`'s per-game archive reads (`loadArchivedGameInjuries` and `hasArchivedOddsBefore`) to games referenced by settled slips via `referencedGamesFromSlips`, and skipped archived-odds lookups when every leg on a referenced game already has `DecimalOdds > 1`.
- Replaced `hasArchivedOddsBefore`'s full JSON decompression and `odds.ParsePropsJSON`/`odds.ParseLinesJSON` parsing with a metadata-only `hasArchivedResponseBefore` check (`store.LatestAPIResponseBefore`), caching `odds:lines` presence per unique kickoff timestamp (`linesByKickoff`).
- Extracted `accumulateWeekCoverage` from `LoadDataset` to keep cyclomatic complexity within lint limits and removed the unused `internal/ingest/odds` import from `internal/eval/replay.go`.
- Added `TestLoadDatasetMultiGameSlateAndCoverageParity` in `internal/eval/replay_test.go` covering multi-game slates, unreferenced games, metadata-only archived odds presence, `odds:lines` leakage guards, and injury-merging parity with `analyzer.MergeGameInjuries`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 2 is complete.

## Commit

Scope replay archive reads to slip-referenced games

Avoids reading and decompressing archived summaries, rosters, and odds for unreferenced games on a week's slate and replaces full odds JSON parsing with metadata-only presence checks, speeding up dataset loading while preserving exact coverage and injury-merging parity.
