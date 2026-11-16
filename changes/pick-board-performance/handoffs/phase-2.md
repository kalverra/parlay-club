# Phase 2: Phase 2: Separate SQLite WAL read pool and batch `UpsertOdds`

## Done

### Task 3: Update `internal/db/db.go` and `internal/db/store.go` to open a multi-connection `query_only=ON` read pool alongside the single-writer handle (falling back to one handle for `:memory:` DBs), prepare statements once in `UpsertOdds`, and test concurrent read-during-write in `internal/db/db_test.go`.

- Updated `internal/db/db.go` to support `:memory:` DSNs without creating disk directories and added `openReadDB` for a 4-connection `query_only(ON)` WAL read pool.
- Updated `internal/db/store.go` so `NewStore` inspects `PRAGMA database_list`, opens the separate read pool for file-backed databases while falling back to the single writer handle for `:memory:` DBs, exposes `ReadDB()` and `Close()`, and routes read-only queries outside transactions through `s.readConn()`.
- Updated `UpsertOdds` in `internal/db/odds.go` to prepare the upsert statement once per transaction via `tx.conn().PrepareContext` (added to `queryer`).
- Added `TestStoreOpensReadPoolForFileDB`, `TestStoreFallsBackToSingleHandleForMemoryDB`, and `TestConcurrentReadsDuringWrite` in `internal/db/db_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

- `BenchmarkOddsForWeek`, `BenchmarkUpsertOdds`, and `BenchmarkConcurrentReadsDuringWrite` in `internal/db/sync_test.go` can now exercise the split read/write pools directly through `db.NewStore(database)` (calling `defer store.Close()` in benchmarks/tests that open a read pool).

## Files touched

- `internal/db/activity.go`
- `internal/db/archive_read.go`
- `internal/db/bugs.go`
- `internal/db/db.go`
- `internal/db/db_test.go`
- `internal/db/games.go`
- `internal/db/groups.go`
- `internal/db/legs.go`
- `internal/db/nfelo.go`
- `internal/db/nflverse.go`
- `internal/db/odds.go`
- `internal/db/players.go`
- `internal/db/record.go`
- `internal/db/schedules.go`
- `internal/db/slips.go`
- `internal/db/store.go`
- `internal/db/sync.go`
- `internal/db/users.go`
- `internal/db/votes.go`
- `internal/db/wagers.go`
- `internal/db/weeks.go`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: lint 5.9s, test reused
- Rounds used: 1 of 2; verify failures: 0
