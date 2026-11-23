# Phase 1: Phase 1: Add canonical archive keys, latest-before query, and batched slip-leg reads in `internal/db`

## Done

### Task 1: Add canonical archive key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) and `LatestAPIResponseBefore` in `internal/db`, wire `internal/refresh/refresh.go` source key helpers to use them, and add unit tests verifying 2xx/cutoff selection and key parity.

- Added canonical archive/sync key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) in `internal/db/archive_keys.go`.
- Added `Store.LatestAPIResponseBefore` and shared `scanAPIResponse` helper in `internal/db/archive_read.go`, querying `api_responses` on the existing `(key, fetched_at)` index with `status >= 200 AND status < 300`, `ORDER BY fetched_at DESC, id DESC LIMIT 1`.
- Wired `internal/refresh/refresh.go` (`OddsLinesSource`, `OddsPropsSource`, `RosterSource`, `GameLogSource`, `SummarySource`) to delegate to `db.Archive*Key`.
- Added `TestLatestAPIResponseBefore` in `internal/db/archive_test.go` and `TestSourceKeysMatchArchiveKeys` in `internal/refresh/refresh_test.go`.

### Task 2: Add batched slip-and-leg loading for a week in `internal/db` with SQLite tests covering group and status filtering.

- Added `Store.LegsForSlips` in `internal/db/legs.go` to batch-load legs for multiple slip IDs in one query ordered by `(slip_id, id)`.
- Added `SlipWithLegs` and `Store.SlipsWithLegsForWeek` in `internal/db/slips.go` to load a week's slips (oldest slip ID first) and their legs with optional `groupID` (`0` = all) and `SlipStatus` (`""` = all) filtering.
- Added SQLite tests `TestLegsForSlipsBatchesAndPreservesOrder` in `internal/db/legs_test.go` and `TestSlipsWithLegsForWeekFiltersByGroupAndStatus` in `internal/db/slips_test.go`.

## Deviations

### Task 1

`TestLatestAPIResponseBefore` only inserts 2xx rows because `api_responses` already enforces `CHECK (status BETWEEN 200 AND 299)` in `0003_api_archive.sql`, so non-2xx rows cannot be inserted via `SaveAPIResponse`. The SQL `WHERE` clause in `LatestAPIResponseBefore` still includes `status >= 200 AND status < 300` as specified.

### Task 2

`SlipsWithLegsForWeek` takes `status SlipStatus` instead of `string` and validates non-empty values with `SlipStatus.Valid()`, matching `SetSlipStatus` and `SetSlipGrade` in `internal/db/slips.go`.

## Open issues

None.

## Next-phase notes

### Task 1

`api_responses_key` index (`api_responses(key, fetched_at)`) is already present in `internal/db/migrations/0003_api_archive.sql`. The next task in Phase 1 adds the batched slip/leg read helper in `internal/db`.

### Task 2

`internal/eval/replay.go` (`loadWeekSlips`) can now replace its per-slip `SlipIDsForWeek` + `SlipByID` + `LegsForSlip` loop with `store.SlipsWithLegsForWeek(ctx, weekID, groupID, db.SlipGraded)`.

## Files touched

- `internal/db/archive_keys.go`
- `internal/db/archive_read.go`
- `internal/db/archive_test.go`
- `internal/db/legs.go`
- `internal/db/legs_test.go`
- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/refresh_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 500ms, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
