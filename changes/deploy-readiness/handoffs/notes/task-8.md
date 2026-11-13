# Task 8 notes

## Done

- `deploy/setup.sh` adds `github-token` to `SECRETS`, so Secret Manager creates it empty and grants `parlay-runtime`
  `roles/secretmanager.secretAccessor` on it like the rest.
- The script grants `parlay-deployer` `roles/secretmanager.viewer` on the project (metadata only, no secret values) and
  `roles/storage.bucketViewer` on `gs://$BUCKET`, so the deploy pre-flight can list secret versions and describe the
  bucket after a re-run.
- The `Next:` block marks `session-key`, the Google pair, `odds-api-key`, and `the-odds-api-key` as the required secrets,
  and calls `github-token` optional: adding a version turns on GitHub issues from bug reports, and leaving it empty keeps
  reports in SQLite.
- `Dockerfile` adds BuildKit cache mounts: `/go/pkg/mod` on `go mod download` and both `/go/pkg/mod` and
  `/root/.cache/go-build` on `go build`. The existing `# syntax=docker/dockerfile:1` line already enables them.

## Deviations

None. The secret list, both bindings, the optional note, and the two cache mounts match the design. `docs/guides/gcp-setup.md`
and `docs/runbooks/deploy.md` still describe five secrets and the old deployer roles; the phase's Docs task owns those edits.

## Open issues

- `deploy/setup.sh` creates `github-token` with no version. That is intended: the deploy pre-flight is supposed to treat a
  versionless `github-token` as "not configured" and skip mounting it. Any check that treats every secret in `SECRETS` as
  required would fail on a fresh project.
- The new bindings only land when someone re-runs `deploy/setup.sh`. An existing project that skips the re-run will see
  the post-auth pre-flight fail with permission errors instead of a clean "not configured" message.
- Docs still say five secrets and the two-role deployer; that is stale until the Docs task runs.

## Next-phase notes

- The workflow task needs `roles/secretmanager.viewer` for `gcloud secrets versions list` and
  `roles/storage.bucketViewer` for `gcloud storage buckets describe`; this task only grants them.
- Buildx GHA caching (`cache-from`/`cache-to: type=gha`) in the workflow plus these Dockerfile mounts is what makes a
  warm build reuse layers. The mounts alone do nothing for a fresh runner without the workflow change.
- `GITHUB_TOKEN=github-token:latest` must be appended only when the pre-flight finds an `ENABLED` version, since Cloud Run
  rejects a revision that points at a versionless secret.
