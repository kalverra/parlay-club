---
id: persistence
title: Persistence
type: concept
summary: How one SQLite file holds every slip and every cached response, how Litestream restores and replicates it on Cloud Run, why only one process may write to it, and how the local sandbox keeps its own file and stage row.
tags: [persistence, sqlite, litestream, gcs, backup, restore, cloud-run, single-instance, wal, sandbox]
---

# Persistence

## Summary

Everything the app owns lives in one SQLite file at `DB_PATH`. In production that is `/data/parlay.db` on the Cloud Run
instance. The container restores the file from a Litestream replica on boot, runs the server under Litestream, and
replicates every change back to a GCS bucket. There is no second database and nothing important in memory.

## One writer

`db.Open` configures the one connection the app uses:

- WAL mode, so reads continue while a write commits.
- `busy_timeout(5000)`, so a lock waits instead of failing.
- `foreign_keys(1)` and `synchronous(NORMAL)`, the safe pairing with WAL.
- `_txlock=immediate`, so a write transaction takes its lock up front instead of upgrading mid-flight.
- `SetMaxOpenConns(1)`. Traffic is tiny, and one connection rules out `SQLITE_BUSY` between handlers.

The in-memory SSE hub has the same assumption: one process. Cloud Run runs exactly one instance (`--max-instances 1`),
and it must stay that way. Two instances would mean two writers on one file and subscribers on one instance missing
events published by the other. Keep shared state in SQLite or the hub, never in a second process. See
[Live updates](live-updates.md) and [Groups](groups.md).

Slips, users, groups, weeks, games, odds, the sync state, bug reports (including capability-token screenshot blobs from
migration `0010_bug_reports.sql`), and the raw response archive all share this file. That is why Litestream backs up
both the domain data and the archive for free. See [Response archive](archive.md).

## The sandbox database

`SUNDAY_SANDBOX=1` uses `./data/sunday-sandbox.db` and ignores `DB_PATH`, so a sandbox run never opens the normal dev
file. It still runs the ordinary migrations, so the domain tables have the production shape.

The sandbox adds one table of its own: `sandbox_state`, a single row holding the active stage and the scenario time.
`internal/sandbox` creates that table; no production migration knows about it, so a normal database never gains it. A
first boot runs the morning sync and then writes the `morning` row. A later boot reads the row, restores the stage and
clock, and leaves the domain rows alone, which is why an Air restart keeps manual edits. Delete
`data/sunday-sandbox.db*` to rebuild from the morning.

`POST /dev/sandbox/reset` is the in-app rebuild. It deletes the disposable domain, sync, and archive rows in one
transaction but keeps the schema, the migration version, and `sandbox_state`. It then runs the morning ingest again and
recreates the fixed personas, shared group, outsider, editable slip, and named legs through the real sign-in, group,
slip, and leg services. The stage row is saved before the rebuild so the app and data agree while it runs; a failed
rebuild answers an error and the next reset can try again. The normal `./data/parlay.db` is never opened.

## Boot in the container

`deploy/entrypoint.sh` runs as the container's entrypoint:

```sh
litestream restore \
	-if-db-not-exists \
	-if-replica-exists \
	-integrity-check full \
	-config /etc/litestream.yml \
	"$DB_PATH"

exec litestream replicate -config /etc/litestream.yml -exec /app/server
```

- `DB_PATH` defaults to `/data/parlay.db`.
- The replica URL is `LITESTREAM_REPLICA_URL` when set, otherwise `gs://$LITESTREAM_BUCKET/parlay.db`. With neither set,
  the entrypoint exits 1 rather than starting without a backup.
- `-if-db-not-exists` keeps the file a warm instance already has, so a restart on the same instance does not restore
  over newer writes.
- `-if-replica-exists` lets the first boot start with an empty replica and create the database fresh.
- `-integrity-check full` runs `PRAGMA integrity_check` on the restored copy. A corrupt replica fails the boot, so Cloud
  Run keeps the previous revision serving instead of moving traffic to a database that answers 500s.
- `litestream replicate -exec` starts the server as a child and forwards signals to it. When the server exits,
  Litestream does a final sync before the container ends.

The image runs as a non-root user and owns `/data`. It copies `seed/history.yaml` too, because the loader's default path
is relative to `/app`. See [History file](../reference/history-file.md).

`deploy/litestream.yml` is Litestream 0.5 syntax: it sets `sync-interval: 10s`, compaction `levels` (`5m`, `1h`, `24h`),
and `snapshot` (`interval: 24h`, `retention: 24h`), alongside a `dbs` list with one `path` and its `replica.url`, both
expanded from the environment. The Dockerfile pins Litestream 0.5.18 in its own build stage and copies the binary into
the runtime image, so the config and the binary always match.

## Replication and shutdown

Litestream tails the WAL and ships new frames to `gs://$LITESTREAM_BUCKET/parlay.db` every `sync-interval: 10s`. That
batching cuts GCS write churn compared to the 1-second default; an ungraceful crash can lose up to 10 seconds of writes,
while Cloud Run's normal `SIGTERM` stop still flushes final frames before exit. Cloud Run also throttles CPU between
requests, so replication can lag until the next request or shutdown. That is fine at this traffic, and a write is
durable on the instance disk before it reaches GCS.

Compaction runs at `levels` of `5m`, `1h`, and `24h`, and `snapshot` keeps 24-hour snapshots (`interval: 24h`,
`retention: 24h`) for point-in-time recovery without 30-second L1 rewrites. `deploy/setup.sh` clears GCS's default 7-day
soft-delete policy on the replica bucket (`--clear-soft-delete`) so compacted and expired LTX segments are freed instead
of retained for a week.

On SIGTERM the server ends open SSE streams, drains in-flight requests for up to 8 seconds (`http.Server.Shutdown`),
then cancels and waits for background refreshes inside that same budget. `database.Close()` runs only after both drains,
and then Litestream forwards the exit, syncs, and stops. Cloud Run allows 10 seconds, so the two seconds of slack are
deliberate. Keep the server from needing more. See [Data refresh](data-refresh.md) for the refresh drain.

## Verified backups

Litestream 0.5 has known restore corruption bugs when a snapshot overlaps a checkpoint. The replica can report no errors
and still restore a database that fails `PRAGMA integrity_check`. Three layers catch that:

- The deploy workflow restores the live replica and runs a full integrity check before it builds or drains anything.
- The entrypoint runs `-integrity-check full` on every restore, so a cold start fails instead of serving 500s.
- A nightly workflow restores the latest replica, verifies it, and uploads the verified database to
  `gs://$LITESTREAM_BUCKET/backups/parlay-YYYY-MM-DD.db`. The bucket keeps 30 days of them. A failed check opens an
  issue.

The backups are plain SQLite files, not LTX segments. See [Database recovery](../runbooks/database-recovery.md) for
using one, or a specific transaction ID, to rebuild a replica.

## Two writers during a deploy

A normal Cloud Run deploy starts the new instance before it stops the old one. Both run Litestream against the same
replica for a few seconds, and Litestream explicitly forbids concurrent replicators. The new instance also restores
while the old one may still commit writes, so writes in that gap can be lost.

The deploy workflow now drains first: it sets the service to manual scaling with zero instances, waits 15 seconds for
the old Litestream process to finish its final sync, then deploys and restores automatic scaling. That trades a short
outage for one writer. The deploy blackout still keeps routine deploys out of the game windows, when the app is busiest.

If a deploy fails after the drain, the workflow's last step sets scaling back to automatic so the service is not left
disabled.

## Restoring locally

Point the replica at a local directory to test the container without GCS:

```sh
docker build -t parlay-club .
docker run --rm -p 8080:8080 \
	-e APP_ENV=dev \
	-e LITESTREAM_REPLICA_URL=file:///replica/parlay.db \
	-v "$PWD/tmp/replica:/replica" \
	parlay-club
```

Dev mode supplies the other settings, so the smoke test can sign in and write a slip. Write a slip, restart the
container, and the slip should still be there: the first run replicates to the local path, and the second restores from
it. Litestream 0.5 writes that path as a directory (`parlay.db/ltx/...`), not a single file. A first boot with an empty
replica logs `no matching backups found` and starts a fresh database. Dev runs outside the container skip all of this
and use a plain file at `DB_PATH` with no Litestream.

## Guardrails

- Never run a second instance or a second process against the same file or replica.
- Read or edit a restored copy, not the live file. A second `sqlite3` process breaks the single-writer assumption, and a
  hand edit has no undo.
- Do not add scheduled jobs that write. Sync is lazy, triggered by page loads, per [Data refresh](data-refresh.md).
- Keep the 8 second shutdown drain under Cloud Run's 10 second limit, or the final sync gets cut off.
