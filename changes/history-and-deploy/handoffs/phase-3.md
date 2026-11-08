# Phase 3 handoff: Container and persistence

## Done

- `Dockerfile` is a three-stage build: `golang:1.27.3-alpine` builds `./cmd/server` with `CGO_ENABLED=0`,
  `-trimpath`, and stripped flags; `litestream/litestream:0.5.18` supplies the pinned binary; `alpine:3.22` is the
  runtime. The runtime copies the server, the Litestream binary, `deploy/litestream.yml`, `deploy/entrypoint.sh`,
  `seed/`, and `web/`, creates `/data` owned by a non-root `app` user, and defaults `APP_ENV=prod`,
  `DB_PATH=/data/parlay.db`, `PORT=8080`.
- `.dockerignore` keeps `.git`, `.good-vibes`, `.rumdl_cache`, `data`, `tmp`, and `.env` out of the build context.
- `deploy/litestream.yml` is 0.5 syntax: one `dbs` entry whose `path` is `${DB_PATH}` and whose `replica.url` is
  `${LITESTREAM_REPLICA_URL}`, both read from the environment.
- `deploy/entrypoint.sh` defaults `DB_PATH` to `/data/parlay.db`, derives the replica URL from `LITESTREAM_BUCKET` when
  `LITESTREAM_REPLICA_URL` is unset, exits 1 when neither is set, restores with `-if-db-not-exists` and
  `-if-replica-exists`, then `exec`s `litestream replicate -exec /app/server` so signals reach both processes.
- Local container smoke test passed. The image built, ran with `APP_ENV=dev` and
  `LITESTREAM_REPLICA_URL=file:///replica/parlay.db`, signed in through `POST /dev/login`, created a group, created a
  past-week slip (`2026` regular season, week 7, one member), and served it. After an LTX snapshot appeared under the
  replica path, the container stopped cleanly. A second container started with a fresh `/data` volume and the same
  replica, and the slip page still showed `Week 7`, so the restore path works.
- Docs: added `docs/concepts/persistence.md` (one writer, container boot, replication and shutdown, the deploy
  two-writer gap, local restore, guardrails), added it to `docs/llms.txt`, and deleted the Persistence section from
  `plan/architecture.md`.
- `go test ./...` and `golangci-lint run ./...` pass, and `good-vibes docs lint` reports `ok`.

## Deviations

- A Litestream 0.5 file replica is a directory, not a file. `file:///replica/parlay.db` becomes
  `/replica/parlay.db/ltx/<level>/<txid>.ltx`. The persistence doc now says so and notes the first boot's
  `no matching backups found` line.
- The smoke test needs a group before it can create a slip, so it signs in with the dev login and creates a group over
  HTTP. A fresh database logs `history: no group yet, skipping seed/history.yaml` on that first boot, exactly as phase
  2 warned.
- The runtime image also copies `web/`. `APP_ENV=prod` serves embedded templates, but `APP_ENV=dev` in the container
  reads them from disk, which the smoke test uses.
- The container runs as the non-root `app` user, so bind-mounted `/data` and `/replica` directories must be writable by
  that user. The smoke test chmods them; a mounted volume that is not writable stops Litestream at boot.
- `.dockerignore` was added, which the design did not name.
- Litestream 0.5 only writes a replica after its sync interval, so the smoke test polls for the first `.ltx` file
  instead of stopping immediately after the write.

## Open issues

- Restore from GCS was not exercised; the smoke test used a file replica. The first Cloud Run deploy is the real check
  that `gcs://$LITESTREAM_BUCKET/parlay.db` restores, and phase 4 must set `LITESTREAM_BUCKET` or the entrypoint exits 1.
- The image is pinned to Litestream 0.5.18. A bump can change config syntax, so treat it as a config change and rerun
  the smoke test.
- The history import still waits for a group. A brand-new deploy creates an empty database, so `seed/history.yaml` lands
  only on a later boot after someone creates a group. The setup runbook should say to create a group and restart once.
- Writes between the last WAL sync and a crash or hard stop are lost; only the instance disk is guaranteed at write
  time. This is documented as the loss window and is acceptable at this traffic.
- The deploy overlap still has no other guard. The blackout in phase 4 is the mitigation named in the design and the
  persistence doc.

## Next-phase notes

- `deploy/setup.sh` must create the GCS bucket and give the runtime service account `storage.objectAdmin` on it. The
  deploy workflow must pass `LITESTREAM_BUCKET` and let the entrypoint derive the `gcs://` URL.
- `/healthz` is public and returns `ok`; Cloud Run health checks and the smoke test both use it. The server listens on
  `$PORT` (8080).
- The deploy runbook should repeat: first boot logs `no matching backups found`, history imports after a group exists,
  and the seed file only adds slips that are not already there.
- `deploy/blackout.sh` must land in phase 4 before the first automated deploy, or the new-instance restore can drop the
  old instance's last writes.
- Keep the restart-instance path in mind for the runbook: a Cloud Run restart uses a fresh `/data`, so it always
  restores from GCS rather than reusing the file.

## Files touched

- `.dockerignore`
- `Dockerfile`
- `deploy/entrypoint.sh`
- `deploy/litestream.yml`
- `docs/concepts/persistence.md`
- `docs/llms.txt`
- `plan/architecture.md`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
