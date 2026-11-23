# Phase 2: Phase 2: Share pure signal helpers and scope `eval.LoadDataset` to referenced games

## Done

### Task 3: Export shared `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, and update `internal/eval/replay.go` to use them alongside `db.Archive*Key`, `LatestAPIResponseBefore`, and batched slip/leg loading.

- Exported `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, sharing `mergeTeamInjuryFeeds` with `MergeGameInjuries`.
- Updated `internal/eval/replay.go` to call `analyzer.DistinctLegPlayerIDs`, `analyzer.NFLverseSeasonTypes`, `analyzer.MergeInjuryFeeds`, `db.Archive*Key`, `store.LatestAPIResponseBefore`, and `store.SlipsWithLegsForWeek`, removing the duplicate local helpers.
- Added unit tests for `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals_test.go`.

### Task 4: Scope `eval.LoadDataset`'s per-game archive reads to games referenced by settled slips and replace full odds-body parsing in `hasArchivedOddsBefore` with metadata-only presence checks, with replay tests covering multi-game slates and coverage parity.

- Scoped `eval.LoadDataset`'s per-game archive reads (`loadArchivedGameInjuries` and `hasArchivedOddsBefore`) to games referenced by settled slips via `referencedGamesFromSlips`, and skipped archived-odds lookups when every leg on a referenced game already has `DecimalOdds > 1`.
- Replaced `hasArchivedOddsBefore`'s full JSON decompression and `odds.ParsePropsJSON`/`odds.ParseLinesJSON` parsing with a metadata-only `hasArchivedResponseBefore` check (`store.LatestAPIResponseBefore`), caching `odds:lines` presence per unique kickoff timestamp (`linesByKickoff`).
- Extracted `accumulateWeekCoverage` from `LoadDataset` to keep cyclomatic complexity within lint limits and removed the unused `internal/ingest/odds` import from `internal/eval/replay.go`.
- Added `TestLoadDatasetMultiGameSlateAndCoverageParity` in `internal/eval/replay_test.go` covering multi-game slates, unreferenced games, metadata-only archived odds presence, `odds:lines` leakage guards, and injury-merging parity with `analyzer.MergeGameInjuries`.

## Deviations

### Task 3

- `NFLverseSeasonTypes` returns `[]int` rather than `[]string` as `design.md` originally wrote; updated `design.md` to match the existing `nflverseSeasonTypes` signature and `db.Store` methods.
- `latestArchivedBodyBefore` adjusts `kickoff` via `beforeKickoff(kickoff)` when calling `LatestAPIResponseBefore` (which uses `fetched_at <= before` at RFC3339 1-second resolution) and still checks `resp.FetchedAt.Before(kickoff)` to preserve the strict `fetched_at < kickoff` leakage guard.

## Open issues

None.

## Next-phase notes

### Task 3

Task 4 can now scope `eval.LoadDataset` to referenced games in `rawLegs` and replace `hasArchivedOddsBefore`'s full JSON decompression/parsing with a metadata-only `LatestAPIResponseBefore` check.

### Task 4

Phase 2 is complete.

## Files touched

- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate 400ms, lint 4.1s, test 9s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
