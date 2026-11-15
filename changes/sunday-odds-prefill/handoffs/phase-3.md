# Phase 3: Phase 3: Cloud Scheduler deploy wiring and setup docs

## Done

### Task 7: Enable `cloudscheduler.googleapis.com`, add the `prefill-token` secret, and print the Sunday 9:30 AM `America/New_York` scheduler setup command in `deploy/setup.sh`, plus mount optional `PREFILL_TOKEN` in `.github/workflows/deploy.yml` (`bash -n` and `deploy/blackout_test.sh` pass)

- Enabled `cloudscheduler.googleapis.com` in `deploy/setup.sh`.
- Added `prefill-token` to `SECRETS` in `deploy/setup.sh` (creating the empty secret and granting
  `roles/secretmanager.secretAccessor` to `parlay-runtime`), and documented it as optional alongside `github-token`.
- Added the Sunday 9:30 AM `America/New_York` (`30 9 * * 0`) `gcloud scheduler jobs create http` command targeting
  `<BASE_URL>/internal/prefill` to the `Next:` instructions printed by `deploy/setup.sh`.
- Updated `.github/workflows/deploy.yml` to check for an `ENABLED` version of `prefill-token` during GCP pre-flight
  (`MOUNT_PREFILL_TOKEN=true`) and mount `PREFILL_TOKEN=prefill-token:latest` in `gcloud run deploy` when present.
- Verified `bash -n deploy/setup.sh` and `deploy/blackout_test.sh` (`22 passed, 0 failed`).

### Task 8: Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` for `prefill-token` and the Cloud Scheduler job, then run `good-vibes docs lint` and `good-vibes docs index`

- Updated `docs/guides/gcp-setup.md` for `cloudscheduler.googleapis.com`, the optional `prefill-token` secret (seven
  secrets total), step 8's `gcloud scheduler jobs create http` command for Sunday 9:30 AM `America/New_York`
  (`30 9 * * 0`) with `--attempt-deadline=3m`, and troubleshooting for `POST /internal/prefill` returning `404`.
- Updated `docs/runbooks/deploy.md` for `prefill-token` in preconditions, `MOUNT_PREFILL_TOKEN` / `PREFILL_TOKEN`
  mounting in `.github/workflows/deploy.yml`, Cloud Scheduler free-tier coverage under Spend, and troubleshooting for
  `POST /internal/prefill` returning `404`.
- Removed retired `sources` and `last_verified` front-matter fields from `docs/guides/gcp-setup.md` and
  `docs/runbooks/deploy.md`.
- Ran `good-vibes docs lint` (0 errors/warnings on `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`;
  pre-existing error on `docs/reference/config.md` left untouched) and `good-vibes docs index` (`wrote docs/llms.txt`).

## Deviations

None.

## Open issues

### Task 8

- `docs/reference/config.md` (modified in an earlier task) still reports
  `retired: last_verified and sources are retired` in `good-vibes docs lint`; left untouched as outside this task.

## Next-phase notes

### Task 7

- Task 8 can update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to match the `prefill-token` secret,
  `MOUNT_PREFILL_TOKEN` behavior in `.github/workflows/deploy.yml`, and the `gcloud scheduler jobs create http` command
  printed by `deploy/setup.sh`.

### Task 8

- Phase 3 is complete.

## Files touched

- `.github/workflows/deploy.yml`
- `deploy/setup.sh`
- `docs/guides/gcp-setup.md`
- `docs/llms.txt`
- `docs/runbooks/deploy.md`

## Run log

- Mode:
- Checkpoints: task-7, task-8
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
