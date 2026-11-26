---
id: database-recovery
title: Database recovery
type: runbook
summary: Recover from a corrupt Litestream replica by finding the last good transaction, seeding a fresh replica, and repointing Cloud Run without losing user data.
tags: [litestream, sqlite, corruption, restore, recovery, cloud-run, gcs, runbook]
---

# Database recovery

## Purpose

Bring the site back when the Litestream replica restores a database that fails `PRAGMA integrity_check`. The symptom is
a burst of `database disk image is malformed (11)` errors, usually on the first page after a cold start or a deploy. The
app is fine; the replica is not.

Do not redeploy onto the corrupt replica, and do not point a new revision at the old prefix. Litestream will replay the
bad transactions on top of whatever it restores.

## Preconditions

- `gcloud` is authenticated with read access to the Litestream bucket and write access to a new prefix.
- `litestream` 0.5.18 and `sqlite3` are installed locally.
- You can update the Cloud Run service and the `LITESTREAM_REPLICA_URL` GitHub variable.
- You know which user actions happened after the last good transaction, so you can tell the user what is missing.

## Steps

### Find the last good transaction

Download the replica and list its transaction ranges:

```sh
mkdir -p /tmp/parlay-replica
gcloud storage cp -r gs://BUCKET/parlay.db /tmp/parlay-replica/
litestream ltx -level all file:///tmp/parlay-replica/parlay.db
```

Restore one transaction at a time, starting with the newest and walking back, and check each result:

```sh
litestream restore \
  -txid 0000000000000061 \
  -o /tmp/parlay-good.db \
  file:///tmp/parlay-replica/parlay.db
sqlite3 /tmp/parlay-good.db 'PRAGMA integrity_check;'
```

`-txid` must match the end of an LTX file shown by `litestream ltx`; a value inside a file returns
`no matching backup files`. A clean copy prints `ok`. Keep the newest clean one.

If you have a nightly verified backup, that is faster:

```sh
gcloud storage ls gs://BUCKET/backups/
gcloud storage cp gs://BUCKET/backups/parlay-YYYY-MM-DD.db /tmp/parlay-good.db
sqlite3 /tmp/parlay-good.db 'PRAGMA integrity_check;'
```

### Check what the rollback costs

The user tables are the ones that matter: `users`, `groups`, `group_members`, `slips`, `slip_members`, `legs`,
`activity`, and `wagers`. Compare the clean copy with the latest restore:

```sh
litestream restore -o /tmp/parlay-latest.db file:///tmp/parlay-replica/parlay.db
for t in users groups group_members slips slip_members legs activity wagers; do
  a=$(sqlite3 /tmp/parlay-good.db ".dump $t" | shasum | cut -d' ' -f1)
  b=$(sqlite3 /tmp/parlay-latest.db ".dump $t" | shasum | cut -d' ' -f1)
  [ "$a" = "$b" ] && echo "$t ok" || echo "$t differs"
done
```

Only `api_bodies`, `api_responses`, and `sync_state` normally differ. Those are cache and sync markers, and the next
refresh rebuilds them.

### Seed a fresh replica

Never write the recovered file back into the corrupt prefix. Start a new prefix:

```sh
mkdir -p /tmp/parlay-seed
cp /tmp/parlay-good.db /tmp/parlay-seed/parlay.db
cat > /tmp/parlay-seed/litestream.yml <<EOF
sync-interval: 1s
dbs:
  - path: /tmp/parlay-seed/parlay.db
    replica:
      url: file:///tmp/parlay-seed/replica
EOF
litestream replicate -config /tmp/parlay-seed/litestream.yml
```

Wait for `snapshot complete`, then stop it with `Ctrl-C`. Restore the seed once to prove it is clean:

```sh
litestream restore -o /tmp/parlay-seed/verify.db file:///tmp/parlay-seed/replica
sqlite3 /tmp/parlay-seed/verify.db 'PRAGMA integrity_check;'
```

Upload it under a dated prefix:

```sh
gcloud storage cp -r /tmp/parlay-seed/replica/ltx gs://BUCKET/parlay-recovered-YYYYMMDD/
```

### Repoint the service

Set the repository variable so future deploys use the recovered prefix:

```sh
gh variable set LITESTREAM_REPLICA_URL \
  --body 'gs://BUCKET/parlay-recovered-YYYYMMDD'
```

Then deploy. The deploy workflow drains the old revision, verifies the new replica, and boots the new revision. For an
emergency change that skips the workflow:

```sh
gcloud run services update parlay-club \
  --project PROJECT_ID --region us-east4 \
  --update-env-vars LITESTREAM_REPLICA_URL=gs://BUCKET/parlay-recovered-YYYYMMDD
```

## Verify

- `curl -fsS https://parlay.hamricklabs.io/readyz` prints `ok`.
- Cloud Run logs for the new revision show no restore error and no `database disk image is malformed`.
- Sign in and load `/`. The home page queries `games` first, so it is the fastest corruption check.
- Open a slip and check that the latest expected user actions are present.

## Rollback

If the recovered prefix fails after the switch, send traffic back to the previous revision:

```sh
gcloud run services update-traffic parlay-club \
  --project PROJECT_ID --region us-east4 \
  --to-revisions REVISION=100
```

That revision still points at the old replica, so it is only a stopgap. Repeat this runbook from the nightly backup, not
from the corrupt prefix.

## Clean up

Leave the corrupt prefix in place until the new one has run for a day. Delete it later with
`gcloud storage rm -r gs://BUCKET/parlay.db/` once a verified backup exists.

## Why this keeps happening

Litestream 0.5 has known restore-corruption bugs in the checkpoint and snapshot path. `0.5.18` is still affected. The
guardrails are:

- the deploy workflow verifies the replica and drains the old revision before starting a new one;
- the entrypoint runs a full integrity check on every restore;
- a nightly workflow verifies the replica and keeps a 30-day set of plain SQLite backups.

See [Persistence](../concepts/persistence.md#verified-backups).
