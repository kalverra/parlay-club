---
slug: unify-signal-and-archive-seam
run: 20261123-220212-unify-signal-and-archive-seam
outcome: done
generated: 2026-11-23T22:17:01Z
---

# Run summary: unify-signal-and-archive-seam

## Outcome

- Status: done
- Branch: unify-signal-and-archive-seam
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 4
- Duration: 12m7s
- Cost: $—
- Fresh input tokens: 447418
- Cache read tokens: 3898624
- Cache write tokens: 0
- Output tokens: 66098

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 4412140 | $— |

## Phases

### Phase 1: Add canonical archive keys, latest-before query, and batched slip-leg reads in `internal/db`

Tasks:

- [x] Add canonical archive key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) and `LatestAPIResponseBefore` in `internal/db`, wire `internal/refresh/refresh.go` source key helpers to use them, and add unit tests verifying 2xx/cutoff selection and key parity.
- [x] Add batched slip-and-leg loading for a week in `internal/db` with SQLite tests covering group and status filtering.

Phase totals: 2 checkpoints, 5m46s, $—, 2117492 input / 32657 output / 1897567 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add canonical archive key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) and `LatestAPIResponseBefore` in `internal/db`, wire `internal/refresh/refresh.go` source key helpers to use them, and add unit tests verifying 2xx/cutoff selection and key parity.

- Added canonical archive/sync key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) in `internal/db/archive_keys.go`.
- Added `Store.LatestAPIResponseBefore` and shared `scanAPIResponse` helper in `internal/db/archive_read.go`, querying `api_responses` on the existing `(key, fetched_at)` index with `status >= 200 AND status < 300`, `ORDER BY fetched_at DESC, id DESC LIMIT 1`.
- Wired `internal/refresh/refresh.go` (`OddsLinesSource`, `OddsPropsSource`, `RosterSource`, `GameLogSource`, `SummarySource`) to delegate to `db.Archive*Key`.
- Added `TestLatestAPIResponseBefore` in `internal/db/archive_test.go` and `TestSourceKeysMatchArchiveKeys` in `internal/refresh/refresh_test.go`.

### Task 2: Add batched slip-and-leg loading for a week in `internal/db` with SQLite tests covering group and status filtering.

- Added `Store.LegsForSlips` in `internal/db/legs.go` to batch-load legs for multiple slip IDs in one query ordered by `(slip_id, id)`.
- Added `SlipWithLegs` and `Store.SlipsWithLegsForWeek` in `internal/db/slips.go` to load a week's slips (oldest slip ID first) and their legs with optional `groupID` (`0` = all) and `SlipStatus` (`""` = all) filtering.
- Added SQLite tests `TestLegsForSlipsBatchesAndPreservesOrder` in `internal/db/legs_test.go` and `TestSlipsWithLegsForWeekFiltersByGroupAndStatus` in `internal/db/slips_test.go`.

### Phase 2: Share pure signal helpers and scope `eval.LoadDataset` to referenced games

Tasks:

- [x] Export shared `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, and update `internal/eval/replay.go` to use them alongside `db.Archive*Key`, `LatestAPIResponseBefore`, and batched slip/leg loading.
- [x] Scope `eval.LoadDataset`'s per-game archive reads to games referenced by settled slips and replace full odds-body parsing in `hasArchivedOddsBefore` with metadata-only presence checks, with replay tests covering multi-game slates and coverage parity.

Phase totals: 2 checkpoints, 5m43s, $—, 1485700 input / 22294 output / 1334038 cache read / 0 cache write tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Export shared `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, and update `internal/eval/replay.go` to use them alongside `db.Archive*Key`, `LatestAPIResponseBefore`, and batched slip/leg loading.

- Exported `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals.go`, sharing `mergeTeamInjuryFeeds` with `MergeGameInjuries`.
- Updated `internal/eval/replay.go` to call `analyzer.DistinctLegPlayerIDs`, `analyzer.NFLverseSeasonTypes`, `analyzer.MergeInjuryFeeds`, `db.Archive*Key`, `store.LatestAPIResponseBefore`, and `store.SlipsWithLegsForWeek`, removing the duplicate local helpers.
- Added unit tests for `DistinctLegPlayerIDs`, `NFLverseSeasonTypes`, and `MergeInjuryFeeds` in `internal/analyzer/signals_test.go`.

### Task 4: Scope `eval.LoadDataset`'s per-game archive reads to games referenced by settled slips and replace full odds-body parsing in `hasArchivedOddsBefore` with metadata-only presence checks, with replay tests covering multi-game slates and coverage parity.

- Scoped `eval.LoadDataset`'s per-game archive reads (`loadArchivedGameInjuries` and `hasArchivedOddsBefore`) to games referenced by settled slips via `referencedGamesFromSlips`, and skipped archived-odds lookups when every leg on a referenced game already has `DecimalOdds > 1`.
- Replaced `hasArchivedOddsBefore`'s full JSON decompression and `odds.ParsePropsJSON`/`odds.ParseLinesJSON` parsing with a metadata-only `hasArchivedResponseBefore` check (`store.LatestAPIResponseBefore`), caching `odds:lines` presence per unique kickoff timestamp (`linesByKickoff`).
- Extracted `accumulateWeekCoverage` from `LoadDataset` to keep cyclomatic complexity within lint limits and removed the unused `internal/ingest/odds` import from `internal/eval/replay.go`.
- Added `TestLoadDatasetMultiGameSlateAndCoverageParity` in `internal/eval/replay_test.go` covering multi-game slates, unreferenced games, metadata-only archived odds presence, `odds:lines` leakage guards, and injury-merging parity with `analyzer.MergeGameInjuries`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
