# Task 8 notes

## Done

- Updated `docs/guides/gcp-setup.md` for `cloudscheduler.googleapis.com`, the optional `prefill-token` secret (seven secrets total), step 8's `gcloud scheduler jobs create http` command for Sunday 9:30 AM `America/New_York` (`30 9 * * 0`) with `--attempt-deadline=3m`, and troubleshooting for `POST /internal/prefill` returning `404`.
- Updated `docs/runbooks/deploy.md` for `prefill-token` in preconditions, `MOUNT_PREFILL_TOKEN` / `PREFILL_TOKEN` mounting in `.github/workflows/deploy.yml`, Cloud Scheduler free-tier coverage under Spend, and troubleshooting for `POST /internal/prefill` returning `404`.
- Removed retired `sources` and `last_verified` front-matter fields from `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`.
- Ran `good-vibes docs lint` (0 errors/warnings on `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`; pre-existing error on `docs/reference/config.md` left untouched) and `good-vibes docs index` (`wrote docs/llms.txt`).

## Deviations

None.

## Open issues

- `docs/reference/config.md` (modified in an earlier task) still reports `retired: last_verified and sources are retired` in `good-vibes docs lint`; left untouched as outside this task.

## Next-phase notes

- Phase 3 is complete.
