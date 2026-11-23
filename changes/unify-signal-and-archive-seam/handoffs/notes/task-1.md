## Done

- Added canonical archive/sync key builders (`ArchiveOddsLinesKey`, `ArchiveOddsPropsKey`, `ArchiveRosterKey`, `ArchiveGameLogKey`, `ArchiveSummaryKey`) in `internal/db/archive_keys.go`.
- Added `Store.LatestAPIResponseBefore` and shared `scanAPIResponse` helper in `internal/db/archive_read.go`, querying `api_responses` on the existing `(key, fetched_at)` index with `status >= 200 AND status < 300`, `ORDER BY fetched_at DESC, id DESC LIMIT 1`.
- Wired `internal/refresh/refresh.go` (`OddsLinesSource`, `OddsPropsSource`, `RosterSource`, `GameLogSource`, `SummarySource`) to delegate to `db.Archive*Key`.
- Added `TestLatestAPIResponseBefore` in `internal/db/archive_test.go` and `TestSourceKeysMatchArchiveKeys` in `internal/refresh/refresh_test.go`.

## Deviations

`TestLatestAPIResponseBefore` only inserts 2xx rows because `api_responses` already enforces `CHECK (status BETWEEN 200 AND 299)` in `0003_api_archive.sql`, so non-2xx rows cannot be inserted via `SaveAPIResponse`. The SQL `WHERE` clause in `LatestAPIResponseBefore` still includes `status >= 200 AND status < 300` as specified.

## Open issues

None.

## Next-phase notes

`api_responses_key` index (`api_responses(key, fetched_at)`) is already present in `internal/db/migrations/0003_api_archive.sql`. The next task in Phase 1 adds the batched slip/leg read helper in `internal/db`.

## Commit

Add canonical archive keys and LatestAPIResponseBefore

Centralize archive and sync_state key construction in internal/db so refresh and eval share one source of truth without an import cycle. Add LatestAPIResponseBefore to fetch the latest 2xx response before a cutoff in SQL instead of loading and scanning all historical rows in Go.
