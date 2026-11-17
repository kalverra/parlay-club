# Task 4: Update `deploy/setup.sh` to run `gcloud storage buckets update "gs://$BUCKET" --clear-soft-delete` idempotently.

## Done

- Updated `deploy/setup.sh` to run `gcloud storage buckets update "gs://$BUCKET" --project="$PROJECT_ID" --clear-soft-delete --quiet` unconditionally after the Litestream bucket create/describe check so both new and existing buckets have GCS's 7-day soft-delete policy disabled.
- Verified `deploy/setup.sh` passes `shellcheck deploy/setup.sh` and `bash -n deploy/setup.sh` with zero warnings or errors.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- The remaining Phase 2 task is updating `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md` to match the new SSE deadlines, Litestream settings, and bucket soft-delete policy.
