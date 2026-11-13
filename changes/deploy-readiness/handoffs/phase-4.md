# Phase 4: Phase 4: Align GCP secrets, build caching, and deploy preflight

## Done

### Task 8: Add `github-token` secret and deployer viewer bindings (`secretmanager.viewer`, `storage.bucketViewer`) to `deploy/setup.sh`, and add BuildKit cache mounts to `Dockerfile`.

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

### Task 9: Update `.github/workflows/deploy.yml` with pre-auth variable checks, post-auth GCP/secret preflight, Buildx GHA layer caching, `GITHUB_REPO` env var, and conditional `GITHUB_TOKEN` secret wiring.

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

### Task 10: Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document the six secrets, deployer viewer permissions, pre-flight checks, and build caching.

- `docs/guides/gcp-setup.md` now lists six secrets (`github-token` marked optional), the deployer's
  `secretmanager.viewer` and `storage.bucketViewer` grants, and a note to re-run `deploy/setup.sh` on an existing
  project. Step 3 is "Add a version to each required secret" and explains the five required versions plus how to add the
  optional `github-token` for GitHub issue filing.
- The variables step adds the optional `ISSUES_REPO` variable, says the workflow passes it as `GITHUB_REPO`, and notes
  the seven required variables are checked before any GCP call. The first-deploy step describes the new pre-flight.
- Both troubleshooting tables add rows for missing variables, a bad `BASE_URL`, unreadable secrets (missing
  `secretmanager.viewer`), missing bucket or Artifact Registry repo, missing secret versions, and bug reports that stay in
  SQLite because `github-token` has no version.
- `docs/runbooks/deploy.md` rewrites "What the workflow does" into nine steps covering the variable check, the GCP
  pre-flight, Buildx with the GitHub Actions layer cache, and the conditional `GITHUB_TOKEN` mount. It adds a "The build
  cache" section (warm versus cold builds, 7-day/10 GB eviction, no Artifact Registry usage) and a spend bullet saying
  the cache does not count against the registry free tier.
- Front matter: both docs bump `last_verified` to 2026-11-13; the deploy runbook adds `deploy/setup.sh` and `Dockerfile`
  to its sources. `rumdl` and `good-vibes docs lint` are clean and `good-vibes docs index` ran.

## Deviations

### Task 8

None. The secret list, both bindings, the optional note, and the two cache mounts match the design. `docs/guides/gcp-setup.md`
and `docs/runbooks/deploy.md` still describe five secrets and the old deployer roles; the phase's Docs task owns those edits.

### Task 9

- The design said `vars.GITHUB_REPO`, but GitHub rejects configuration variable names with the reserved `GITHUB_` prefix
  (actionlint flags it too). The workflow reads the optional `vars.ISSUES_REPO` and falls back to `github.repository`;
  the container still receives it as the `GITHUB_REPO` env var. The design's Approach section now says `ISSUES_REPO`.
- The design pinned `docker/setup-buildx-action@v3` and `docker/build-push-action@v6`, which run on the deprecated
  node20 runtime scheduled for removal in September 2026. The workflow uses `@v4` and `@v7`, the current majors, and the
  design now matches.
- The required-secret check prints the `gcloud` error when a version list fails, so a missing role reads as "cannot read
  secret; run deploy/setup.sh to create it and grant roles/secretmanager.viewer" instead of a false "missing" message.

### Task 10

None. The docs now describe the state left by Tasks 8 and 9, including `ISSUES_REPO` and the pre-flight's handling of a
versionless `github-token`.

## Open issues

### Task 8

- `deploy/setup.sh` creates `github-token` with no version. That is intended: the deploy pre-flight is supposed to treat a
  versionless `github-token` as "not configured" and skip mounting it. Any check that treats every secret in `SECRETS` as
  required would fail on a fresh project.
- The new bindings only land when someone re-runs `deploy/setup.sh`. An existing project that skips the re-run will see
  the post-auth pre-flight fail with permission errors instead of a clean "not configured" message.
- Docs still say five secrets and the two-role deployer; that is stale until the Docs task runs.

### Task 9

- The pre-flight only succeeds after `deploy/setup.sh` is re-run on an existing project: `parlay-deployer` needs
  `roles/secretmanager.viewer` and `roles/storage.bucketViewer` for the new checks.
- Nothing was run against a real GCP project. The workflow was validated with `actionlint`, `bash -n` on every `run:`
  block, and a stubbed `gcloud` that covers valid, empty-secret, unreadable-secret, and missing-resource cases.
- `ISSUES_REPO` is a new, optional repository variable. No repo variable named `GITHUB_REPO` can exist.

### Task 10

- Nothing was run against a real GCP project. The documented pre-flight messages and role requirements come from
  `.github/workflows/deploy.yml` and `deploy/setup.sh`, not from a live run.
- `docs/reference/config.md` still says `GITHUB_REPO` defaults to `kalverra/parlay-club` in prod. That is still the
  binary's default, but the deploy now always sets the env var from `vars.ISSUES_REPO` or the workflow's repo, so the
  deployed value is never the fallback. Left alone because this task names only the two docs.
- `docs/runbooks/google-oauth-setup.md` lists five secrets in its preconditions. The list is not exhaustive (`github-token`
  is unrelated to OAuth) and remains correct.

## Next-phase notes

### Task 8

- The workflow task needs `roles/secretmanager.viewer` for `gcloud secrets versions list` and
  `roles/storage.bucketViewer` for `gcloud storage buckets describe`; this task only grants them.
- Buildx GHA caching (`cache-from`/`cache-to: type=gha`) in the workflow plus these Dockerfile mounts is what makes a
  warm build reuse layers. The mounts alone do nothing for a fresh runner without the workflow change.
- `GITHUB_TOKEN=github-token:latest` must be appended only when the pre-flight finds an `ENABLED` version, since Cloud Run
  rejects a revision that points at a versionless secret.

### Task 9

- The Docs task owns `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md`. They still say five secrets, the old
  two-role deployer, raw `docker build`, and "the four Secret Manager secrets"; all of that is stale now. The docs
  should also note the pre-flight checks, the GHA build cache, and the `ISSUES_REPO` variable.
- The pre-auth `BASE_URL` check is deliberately simpler than Go's `url.Parse`: it requires `http://` or `https://`
  followed by a host-shaped string. It catches typos, not every edge case the Go validator rejects.
- A project that has never added a `github-token` version deploys fine; bug reports stay in SQLite until someone adds a
  version, and the next deploy mounts it automatically.

### Task 10

- Re-running `deploy/setup.sh` on an already-provisioned project is now documented as required before the first deploy
  with the new workflow: without `roles/secretmanager.viewer` and `roles/storage.bucketViewer`, the pre-flight reports
  unreadable secrets and the bucket instead of a clean missing-resource message.
- If a later task changes the workflow's variable names, secret list, or cache settings, both docs and the anchor
  `gcp-setup.md#3-add-a-version-to-each-required-secret` (linked from the deploy runbook) need to move with it.

## Files touched

- `.github/workflows/deploy.yml`
- `Dockerfile`
- `deploy/setup.sh`
- `docs/guides/gcp-setup.md`
- `docs/runbooks/deploy.md`

## Run log

- Mode: 
- Checkpoints: task-8, task-9, task-10
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
