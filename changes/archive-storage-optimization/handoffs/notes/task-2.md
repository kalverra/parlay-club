## Done

- Added `internal/db/migrations/0022_prune_bulk_archive.sql` to delete `source IN ('nflverse', 'nfelo')` rows from `api_responses` and then delete orphaned `api_bodies` rows not referenced by any remaining `api_responses` row.
- Updated `latestSchemaVersion` in `internal/db/migrate_test.go` and the version assertion in `internal/db/migrate_internal_test.go` from 21 to 22.
- Added `TestMigration0022PruneBulkArchive` in `internal/db/migrate_test.go` verifying that `nflverse` and `nfelo` archive rows and their unreferenced bodies are deleted while `espn`, `odds`, `weather`, and shared bodies survive intact.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Deleted rows leave freed pages in SQLite until `VACUUM` runs; Task 3 should document the one-time post-deploy `VACUUM` step in `docs/runbooks/archive-size.md` alongside updating `docs/concepts/archive.md` and `docs/reference/data-sources.md`.

## Commit

Prune nflverse and nfelo rows from raw archive

Historical nflverse and nfelo responses account for most of the raw HTTP archive bloat while already being stored in parsed week-keyed tables. Removing them via a forward migration shrinks the archive to only the ESPN, Odds, and Weather snapshots needed for eval replay and debugging.
