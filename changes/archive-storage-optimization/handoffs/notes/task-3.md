## Done

- Updated `docs/concepts/archive.md` to document that `nfelo` and `nflverse` use a plain `*http.Client` (`newBulkClient` in `cmd/server/main.go`) without `archive.Transport`, added the required `## Overview` Mermaid diagram, and brought the front matter and fences into `good-vibes docs lint` compliance.
- Updated `docs/reference/data-sources.md` to exclude `nfelo` and `nflverse` from the raw response archive and remove stale references to archiving those bulk CSVs.
- Updated `docs/runbooks/archive-size.md` to note the bulk-source exclusion and document the one-time post-deploy `VACUUM` step after `internal/db/migrations/0022_prune_bulk_archive.sql`, while updating SQL snippets to `sh` `sqlite3` commands for `good-vibes docs lint`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- After deploying `internal/db/migrations/0022_prune_bulk_archive.sql`, run `sqlite3 "$DB_PATH" "VACUUM;"` during a quiet maintenance window per `docs/runbooks/archive-size.md` so freed SQLite pages are returned to the OS and Litestream snapshots shrink.

## Commit

Document bulk archive exclusion and post-deploy VACUUM

Living docs still claimed nfelo and nflverse responses were archived raw, and the archive-size runbook lacked the post-deploy VACUUM procedure needed to reclaim disk space after migration 0022 deletes historical bulk rows. Updating the concept, reference, and runbook docs keeps operators and contributors aligned with the non-archiving bulk client wiring and SQLite compaction steps.
