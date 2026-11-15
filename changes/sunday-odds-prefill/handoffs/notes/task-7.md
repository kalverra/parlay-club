## Done

- Enabled `cloudscheduler.googleapis.com` in `deploy/setup.sh`.
- Added `prefill-token` to `SECRETS` in `deploy/setup.sh` (creating the empty secret and granting
  `roles/secretmanager.secretAccessor` to `parlay-runtime`), and documented it as optional alongside `github-token`.
- Added the Sunday 9:30 AM `America/New_York` (`30 9 * * 0`) `gcloud scheduler jobs create http` command targeting
  `<BASE_URL>/internal/prefill` to the `Next:` instructions printed by `deploy/setup.sh`.
- Updated `.github/workflows/deploy.yml` to check for an `ENABLED` version of `prefill-token` during GCP pre-flight
  (`MOUNT_PREFILL_TOKEN=true`) and mount `PREFILL_TOKEN=prefill-token:latest` in `gcloud run deploy` when present.
- Verified `bash -n deploy/setup.sh` and `deploy/blackout_test.sh` (`22 passed, 0 failed`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 8 can update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to match the `prefill-token` secret,
  `MOUNT_PREFILL_TOKEN` behavior in `.github/workflows/deploy.yml`, and the `gcloud scheduler jobs create http` command
  printed by `deploy/setup.sh`.
