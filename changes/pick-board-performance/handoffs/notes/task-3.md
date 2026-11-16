# Task 3 Notes

## Done

- Updated `internal/db/db.go` to support `:memory:` DSNs without creating disk directories and added `openReadDB` for a 4-connection `query_only(ON)` WAL read pool.
- Updated `internal/db/store.go` so `NewStore` inspects `PRAGMA database_list`, opens the separate read pool for file-backed databases while falling back to the single writer handle for `:memory:` DBs, exposes `ReadDB()` and `Close()`, and routes read-only queries outside transactions through `s.readConn()`.
- Updated `UpsertOdds` in `internal/db/odds.go` to prepare the upsert statement once per transaction via `tx.conn().PrepareContext` (added to `queryer`).
- Added `TestStoreOpensReadPoolForFileDB`, `TestStoreFallsBackToSingleHandleForMemoryDB`, and `TestConcurrentReadsDuringWrite` in `internal/db/db_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `BenchmarkOddsForWeek`, `BenchmarkUpsertOdds`, and `BenchmarkConcurrentReadsDuringWrite` in `internal/db/sync_test.go` can now exercise the split read/write pools directly through `db.NewStore(database)` (calling `defer store.Close()` in benchmarks/tests that open a read pool).
