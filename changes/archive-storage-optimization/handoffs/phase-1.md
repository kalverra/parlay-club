# Phase 1: Phase 1: Exclude nflverse and nfelo from raw archiving and prune existing bloat

## Done

### Task 1: Wire `nfelo.NewClient` and `nflverse.NewClient` in `cmd/server/main.go` with a non-archiving `*http.Client` (preserving timeout and `User-Agent`), and update `cmd/server/main_test.go` (`TestRefreshArchivesEveryIngestCall`) to assert that `espn`, `weather`, and `odds` are archived while `nflverse` and `nfelo` are not.

- Added `newBulkClient(ingestClient *http.Client) *http.Client` in `cmd/server/main.go`, preserving the ingest timeout (`ingestTimeout` or `ingestClient.Timeout`) with a plain `http.DefaultTransport` (no `archive.Transport`).
- Wired `nfelo.NewClient` and `nflverse.NewClient` in `newProdServices` with `newBulkClient(ingestClient)` so `nfelo` and `nflverse` responses are no longer written to `api_responses` or `api_bodies`, while `User-Agent: parlay-club` continues to be set by each client.
- Updated `TestRefreshArchivesEveryIngestCall` in `cmd/server/main_test.go` with `fakeNfelo` and `fakeNFLverse` servers to assert that `espn`, `weather`, and `odds` are archived while `nfelo` and `nflverse` are not, and that `nfelo`/`nflverse` sync states and parsed `nflverse_team_weeks` rows are still populated.

### Task 2: Add a SQLite migration in `internal/db/migrations/` to delete `source IN ('nflverse', 'nfelo')` from `api_responses` and orphaned rows from `api_bodies`, with a migration test in `internal/db/migrate_test.go` confirming ESPN and Odds archive rows survive intact.

- Added `internal/db/migrations/0022_prune_bulk_archive.sql` to delete `source IN ('nflverse', 'nfelo')` rows from `api_responses` and then delete orphaned `api_bodies` rows not referenced by any remaining `api_responses` row.
- Updated `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in `internal/db/migrate_internal_test.go` from 21 to 22.
- Added `TestMigration0022PruneBulkArchive` in `internal/db/migrate_test.go` verifying that `nflverse` and `nfelo` archive rows and their unreferenced bodies are deleted while `espn`, `odds`, `weather`, and shared bodies survive intact.

### Task 3: Docs: update `docs/concepts/archive.md`, `docs/reference/data-sources.md`, and `docs/runbooks/archive-size.md` to document the bulk-source exclusion and post-deploy `VACUUM` step.

- Updated `docs/concepts/archive.md` to document that `nfelo` and `nflverse` use a plain `*http.Client` (`newBulkClient` in `cmd/server/main.go`) without `archive.Transport`, added the required `## Overview` Mermaid diagram, and brought the front matter and fences into `good-vibes docs lint` compliance.
- Updated `docs/reference/data-sources.md` to exclude `nfelo` and `nflverse` from the raw response archive and remove stale references to archiving those bulk CSVs.
- Updated `docs/runbooks/archive-size.md` to note the bulk-source exclusion and document the one-time post-deploy `VACUUM` step after `internal/db/migrations/0022_prune_bulk_archive.sql`, while updating SQL snippets to `sh` `sqlite3` commands for `good-vibes docs lint`.

## Deviations

### Task 3

- None.

## Open issues

### Task 3

- None.

## Next-phase notes

### Task 1

- Task 2 can now add the forward migration pruning historical `nflverse` and `nfelo` rows from `api_responses` and orphaned `api_bodies` without new rows being re-archived by `cmd/server`.

### Task 2

- Deleted rows leave freed pages in SQLite until `VACUUM` runs; Task 3 should document the one-time post-deploy `VACUUM` step in `docs/runbooks/archive-size.md` alongside updating `docs/concepts/archive.md` and `docs/reference/data-sources.md`.

### Task 3

- After deploying `internal/db/migrations/0022_prune_bulk_archive.sql`, run `sqlite3 "$DB_PATH" "VACUUM;"` during a quiet maintenance window per `docs/runbooks/archive-size.md` so freed SQLite pages are returned to the OS and Litestream snapshots shrink.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/concepts/archive.md`
- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/runbooks/archive-size.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0022_prune_bulk_archive.sql`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 11.1s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
