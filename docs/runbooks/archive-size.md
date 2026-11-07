---
id: archive-size
title: Archive size check
type: runbook
summary: Measure the raw response archive and the SQLite file, know the number to act at, and prune old responses without losing the parsed tables.
tags: [archive, sqlite, size, growth, litestream, runbook]
sources: [internal/db/archive.go, internal/db/archive_read.go, internal/db/migrations/0003_api_archive.sql, cmd/server/main.go]
last_verified: 2026-11-07
---

# Archive size check

## Purpose

Keep the raw response archive from filling the one SQLite file that Litestream replicates and that the Cloud Run
instance holds in memory. The archive is the only part of the database that grows without a cap: `api_responses` gains a
row per outside call, and `api_bodies` gains a body whenever the bytes change.

## When

At the end of the first season, then once a month. Check after a Sunday or Monday sync, when the newest lines and props
bodies have landed. The check is cheap and reads only the database.

## Preconditions

- Access to the app's database file or a Litestream restore, and `sqlite3` on the machine where you run the queries.
- A maintenance window and a fresh backup before any prune. The app writes through one connection, so a prune is not a
  read-only check.

## Steps

Run the queries against the app's database file, `DB_PATH` (`/data/parlay.db` in prod, `./data/parlay.db` in dev). Use a
copy or a Litestream restore when you can, so the live instance keeps its single writer.

1. Measure the archive in uncompressed bytes:

   ```sql
   SELECT sum(size) AS uncompressed_bytes, count(*) AS bodies FROM api_bodies;
   ```

   `size` is the uncompressed body, not the gzipped blob on disk. JSON compresses roughly 10x, so divide by about ten to
   estimate the space it takes in the file.

2. Measure the file itself. Include the write-ahead log, because Litestream and the instance see both:

   ```sh
   ls -lh "$DB_PATH" "$DB_PATH-wal"
   ```

3. Break the archive down by source key to see what is growing:

   ```sql
   SELECT r.key, count(*) AS calls, sum(b.size) AS uncompressed_bytes
   FROM api_responses r
   JOIN api_bodies b ON b.sha256 = r.body_sha256
   GROUP BY r.key
   ORDER BY uncompressed_bytes DESC;
   ```

## Numbers

| Measure                            | Healthy      | Act at   |
| ---------------------------------- | ------------ | -------- |
| `parlay.db` plus `parlay.db-wal`   | under 150 MB | 200 MB   |
| `SELECT sum(size) FROM api_bodies` | under 1 GB   | 1.5 GB   |

The file size is the number that matters. The instance has 512 Mi of memory and the database file lives in it, so a file
near the memory limit leaves no room for the server, the WAL, or Litestream during a sync. The uncompressed body total
is the early warning, because gzip hides most of its size.

## When the number is crossed

1. Confirm the parsed tables are still small. The archive can go; `games`, `odds`, `weather` columns, and `sync_state`
   cannot. If those tables are the ones growing, this is not the archive's problem.
2. Pick a fix, in this order:
   - Raise the Cloud Run memory limit if the spend is fine and the file is expected to keep growing this season.
   - Prune the archive in a planned maintenance window, keeping the parsed rows. Stop the service first, back up the
     file, then run this in one `sqlite3` session:

     ```sql
     PRAGMA foreign_keys = ON;
     DELETE FROM api_responses WHERE fetched_at < '2026-09-01T00:00:00Z';
     DELETE FROM api_bodies WHERE sha256 NOT IN (SELECT body_sha256 FROM api_responses);
     VACUUM;
     ```

     Delete responses before bodies. `VACUUM` needs free space about the size of the file and holds the write lock for
     its duration. Prune the live file, not a restored copy, so Litestream replicates the change; the entrypoint
     restores only when the database file is missing.
   - Open a change that adds retention or moves the archive to its own database. The app deliberately has no delete or
     retention job today, so this is the honest long-term fix if the archive keeps growing.
3. Record the new size and date in the change or the deploy notes. The point of the check is the trend, not one reading.

## Rollback

- **A prune removed too much:** stop the app and restore the database from the backup taken before the prune, or let
  Litestream restore an older generation of the replica. The parsed tables were never touched, so the app keeps working;
  only the research history is lost.
- **The file grew again after a prune:** the archive is doing its job. Raise memory, add retention, or move the archive
  to its own database in a change instead of pruning on a schedule.
- **A query ran against the live file and locked a sync:** stop the query, let the write lock clear, and rerun against a
  copy.

## Verify

- `SELECT sum(size) FROM api_bodies` returns after the prune.
- The app still lists the week's games and lines, and `sync_state` still has every source key.
- Litestream reports a successful sync of the smaller file after the next write or shutdown.
