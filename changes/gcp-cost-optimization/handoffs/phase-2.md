# Phase 2: Phase 2: Tune Litestream replication and GCS bucket retention

## Done

### Task 3: Update `deploy/litestream.yml` with `sync-interval: 10s`, reduced compaction `levels` (`5m`, `1h`, `24h`), and `snapshot` retention (`24h`), and verify config parsing and restore against `litestream:0.5.18`.

- Updated `deploy/litestream.yml` with `sync-interval: 10s`, compaction `levels` (`5m`, `1h`, `24h`), and `snapshot` (`interval: 24h`, `retention: 24h`).
- Verified against `litestream/litestream:0.5.18` in Docker that `sync-interval`, `levels`, and `snapshot` (`interval` and `retention`) are parsed as valid `time.Duration` fields (invalid durations fail YAML unmarshaling).
- Ran a full `replicate -once -force-snapshot -enforce-retention` and `restore -if-db-not-exists -if-replica-exists` smoke test using `deploy/litestream.yml` against `litestream/litestream:0.5.18`, confirming `sync-interval=10s`, snapshot creation, retention enforcement, and intact SQLite row restoration.

### Task 4: Update `deploy/setup.sh` to run `gcloud storage buckets update "gs://$BUCKET" --clear-soft-delete` idempotently.

- Updated `deploy/setup.sh` to run `gcloud storage buckets update "gs://$BUCKET" --project="$PROJECT_ID" --clear-soft-delete --quiet` unconditionally after the Litestream bucket create/describe check so both new and existing buckets have GCS's 7-day soft-delete policy disabled.
- Verified `deploy/setup.sh` passes `shellcheck deploy/setup.sh` and `bash -n deploy/setup.sh` with zero warnings or errors.

### Task 5: Docs: update `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md` with the new SSE deadlines, Litestream settings, and bucket soft-delete policy.

- Updated [live-updates.md](docs/concepts/live-updates.md) with the `15m` live / `3m` idle server SSE deadlines, `GRADED` slip `event: bye` short-circuit and omitted `data-live-url`, and the client's `3m` inactivity pause, `pagehide` teardown, and reopen refresh behavior.
- Updated [persistence.md](docs/concepts/persistence.md) with `deploy/litestream.yml`'s `sync-interval: 10s`, compaction `levels` (`5m`, `1h`, `24h`), `snapshot` (`interval: 24h`, `retention: 24h`), and `deploy/setup.sh` clearing GCS soft-delete.
- Updated [gcp-setup.md](docs/guides/gcp-setup.md) to document `--clear-soft-delete` on the Litestream GCS bucket during initial setup, idempotent re-runs, and spend expectations.
- Removed retired `sources` and `last_verified` front-matter keys from `docs/concepts/live-updates.md` and `docs/concepts/persistence.md` so `good-vibes docs lint` passes with `0 error(s)`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

- Remaining Phase 2 tasks cover `deploy/setup.sh` (`--clear-soft-delete`) and updating `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md`.

### Task 4

- The remaining Phase 2 task is updating `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md` to match the new SSE deadlines, Litestream settings, and bucket soft-delete policy.

### Task 5

All Phase 2 tasks are complete.

## Files touched

- `deploy/litestream.yml`
- `deploy/setup.sh`
- `docs/concepts/live-updates.md`
- `docs/concepts/persistence.md`
- `docs/guides/gcp-setup.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
