# Task 9 notes

## Done

- `deploy.yml` gains a pre-auth **Check the deployment variables** step. It fails before any GCP call when one of
  `GCP_PROJECT`, `GCP_REGION`, `WIF_PROVIDER`, `DEPLOY_SA`, `RUNTIME_SA`, `LITESTREAM_BUCKET`, or `AR_REPO` is empty,
  and rejects a `BASE_URL` that is not `http`/`https` with a host.
- A post-auth **Pre-flight the GCP project** step checks the Artifact Registry repo, the Litestream bucket, and an
  `ENABLED` version for each of `session-key`, `google-client-id`, `google-client-secret`, `odds-api-key`, and
  `the-odds-api-key`. It collects every failure before exiting. `github-token` is checked separately and writes
  `MOUNT_GITHUB_TOKEN=true` to `GITHUB_ENV` only when it has an `ENABLED` version; otherwise it logs a notice.
- The build path is now `docker/setup-buildx-action` plus `docker/build-push-action` with `cache-from: type=gha` and
  `cache-to: type=gha,mode=max`. The image tag is resolved once into both `GITHUB_ENV` (for the deploy step) and a step
  output (for the build action).
- `gcloud run deploy` passes `GITHUB_REPO` in `--update-env-vars` and appends `GITHUB_TOKEN=github-token:latest` to
  `--set-secrets` only when `MOUNT_GITHUB_TOKEN` is `true`.

## Deviations

- The design said `vars.GITHUB_REPO`, but GitHub rejects configuration variable names with the reserved `GITHUB_` prefix
  (actionlint flags it too). The workflow reads the optional `vars.ISSUES_REPO` and falls back to `github.repository`;
  the container still receives it as the `GITHUB_REPO` env var. The design's Approach section now says `ISSUES_REPO`.
- The design pinned `docker/setup-buildx-action@v3` and `docker/build-push-action@v6`, which run on the deprecated
  node20 runtime scheduled for removal in September 2026. The workflow uses `@v4` and `@v7`, the current majors, and the
  design now matches.
- The required-secret check prints the `gcloud` error when a version list fails, so a missing role reads as "cannot read
  secret; run deploy/setup.sh to create it and grant roles/secretmanager.viewer" instead of a false "missing" message.

## Open issues

- The pre-flight only succeeds after `deploy/setup.sh` is re-run on an existing project: `parlay-deployer` needs
  `roles/secretmanager.viewer` and `roles/storage.bucketViewer` for the new checks.
- Nothing was run against a real GCP project. The workflow was validated with `actionlint`, `bash -n` on every `run:`
  block, and a stubbed `gcloud` that covers valid, empty-secret, unreadable-secret, and missing-resource cases.
- `ISSUES_REPO` is a new, optional repository variable. No repo variable named `GITHUB_REPO` can exist.

## Next-phase notes

- The Docs task owns `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`. They still say five secrets, the old
  two-role deployer, raw `docker build`, and "the four Secret Manager secrets"; all of that is stale now. The docs
  should also note the pre-flight checks, the GHA build cache, and the `ISSUES_REPO` variable.
- The pre-auth `BASE_URL` check is deliberately simpler than Go's `url.Parse`: it requires `http://` or `https://`
  followed by a host-shaped string. It catches typos, not every edge case the Go validator rejects.
- A project that has never added a `github-token` version deploys fine; bug reports stay in SQLite until someone adds a
  version, and the next deploy mounts it automatically.
