---
id: deploy
title: Deploy
type: runbook
summary: How a commit reaches Cloud Run, why deploys pause during game windows, how to force one, and how to verify, roll back, and read the spend.
tags: [deploy, cloud-run, github-actions, blackout, litestream, rollback, runbook, cloud-scheduler]
---

# Deploy

## Purpose

Ship a commit to Cloud Run without losing writes, and know how to check, undo, and pay for it. The one-time project
setup lives in [GCP setup](../guides/gcp-setup.md); this runbook covers every deploy after it.

## Preconditions

- The [GCP setup](../guides/gcp-setup.md) is finished: the bucket, secrets, service accounts, Workload Identity, and
  GitHub variables all exist.
- The five required secrets have an `ENABLED` version. `github-token` and `prefill-token` are optional: with a
  version, bug reports open GitHub issues and `POST /internal/prefill` accepts bearer-token requests; without one,
  bug reports stay in SQLite and `POST /internal/prefill` returns `404`.
- `parlay-deployer` can read Secret Manager metadata and the Litestream bucket. A project set up before the pre-flight
  checks existed needs `deploy/setup.sh` run once more; it grants `roles/secretmanager.viewer` and
  `roles/storage.bucketViewer`.
- You can open the repo's **Actions** tab, or you have `gcloud` signed in for traffic changes and log checks.
- For a normal deploy, CI passes on `main`. For a manual deploy, you know which branch head will ship.
- If a game is on, you have decided whether to wait or to force the deploy and accept the write-loss risk.

## Steps

### How a deploy starts

- **On a merge to `main`:** CI runs, and when it succeeds the Deploy workflow runs on the same commit. A failed or
  cancelled CI run deploys nothing. Pull request CI never deploys, even if a fork branch is named `main`.
- **By hand:** **Actions → Deploy → Run workflow**, choose the branch, and optionally set `force`. This deploys that
  branch's head; it does not wait for CI.

Deploys queue. The `deploy` concurrency group runs one at a time and does not cancel a run in progress, because two
overlapping deploys are exactly what the blackout exists to prevent.

### The blackout

`deploy/blackout.sh` refuses a deploy from Thursday 20:00 through 23:59, Sunday 09:00 through 23:59, and Monday 20:00
through 23:59, in `America/New_York`, daylight saving included. It exits 1 inside those windows and 0 outside them.

The reason is the database. A new revision restores `/data/parlay.db` from the Litestream replica while the old revision
may still be writing. Writes made in that gap can be lost. Outside the game windows the app is quiet enough that the gap
costs nothing; during a game it is the busiest it gets. See
[Persistence](../concepts/persistence.md#two-writers-during-a-deploy).

To deploy during a window, run the workflow manually and set `force`. Accept the write-loss risk first; a forced deploy
is the only way past the gate.

Check the gate locally by pinning the clock with `NOW`:

```sh
NOW=2026-11-12T21:00:00-05:00 deploy/blackout.sh   # Thursday 21:00 New York: exits 1
NOW=2026-11-08T15:00:00Z deploy/blackout.sh         # Sunday 10:00 New York: exits 1
NOW=2026-11-09T15:00:00Z deploy/blackout.sh         # Monday 10:00 New York: exits 0
```

`deploy/blackout_test.sh` covers all three windows, both sides of their edges, and both EST and EDT. CI runs it on every
push, so a broken gate fails before it can block or allow a deploy.

### What the workflow does

1. Checks out the tested commit. For a CI-triggered run that is the commit CI ran, not whatever `main` points at now.
2. Runs the blackout gate unless `force` is set.
3. Checks the deployment variables before any GCP call. `GCP_PROJECT`, `GCP_REGION`, `WIF_PROVIDER`, `DEPLOY_SA`,
   `RUNTIME_SA`, `LITESTREAM_BUCKET`, and `AR_REPO` must be set, and a set `BASE_URL` must be `http` or `https` with a
   host. A missing or malformed value fails in seconds and names the variable.
4. Authenticates to Google Cloud through Workload Identity Federation as `parlay-deployer`. No service account key is
   stored in GitHub.
5. Pre-flights the GCP project: the Artifact Registry repo, the Litestream bucket, and an `ENABLED` version of each
   required secret. It collects every failure, then exits before the build. `github-token` and `prefill-token` are
   checked separately; when each has an `ENABLED` version the job sets `MOUNT_GITHUB_TOKEN=true` or
   `MOUNT_PREFILL_TOKEN=true` so the deploy mounts it, and when one does not the job logs a notice and leaves bug
   reports in SQLite or `POST /internal/prefill` disabled (`404`).
6. Builds the `Dockerfile` with Buildx and pushes the image to Artifact Registry tagged with the commit SHA, reusing
   layers from the GitHub Actions cache.
7. Runs `gcloud run deploy` with one instance at most, concurrency 80, 1 CPU, 512 MiB, a 3600 second timeout, CPU boost,
   public access, the runtime account, `APP_ENV=prod`, `DB_PATH=/data/parlay.db`, `LITESTREAM_BUCKET`, `BASE_URL`, and
   `GITHUB_REPO` (from the optional `ISSUES_REPO` variable, or the repo that runs the workflow). It mounts the five
   required Secret Manager secrets, plus `GITHUB_TOKEN` when `github-token` has a version and `PREFILL_TOKEN` when
   `prefill-token` has a version.
8. Reads the service URL and retries once with `BASE_URL` set to that URL if the first revision could not boot. That is
   the first-deploy path; later deploys get `BASE_URL` from the repo variable.
9. Waits for `GET /healthz` on the new revision, then writes the image and URL to the job summary.

The image holds the server, the pinned Litestream binary, its config, the entrypoint, `seed/history.yaml`, and `web/`.
The entrypoint restores the database, then runs the server under `litestream replicate`.

### The build cache

The build runs under Buildx and stores image layers in the GitHub Actions cache (`cache-from` and `cache-to: type=gha`).
The `Dockerfile` also keeps the Go module cache and the Go build cache in BuildKit cache mounts. A build with a warm
cache skips dependency downloads and unchanged packages; a cold one compiles everything, so the first run after a cache
miss takes several minutes longer. Neither outcome changes the image.

GitHub evicts cache entries unused for seven days or beyond 10 GB. A cold build is not a failure. The cache lives in
GitHub, not Artifact Registry, so it does not count against the registry's 0.5 GB free tier.

### Deploy by hand

Use this when the merge path is blocked, or when you need a revision to rerun boot, for example to import
`seed/history.yaml` after the first group exists.

1. Confirm the branch is the one you mean and its CI is green if it needs to be.
2. **Actions → Deploy → Run workflow**.
3. Leave `force` off unless a game is on and you accept the write-loss risk.
4. Watch the job. A green run prints the URL; a red run names the failing step.

To rerun boot without changing code, dispatch the workflow again. A new revision always starts with a fresh `/data`, so
it restores from GCS instead of reusing a file.

## Rollback

Roll back the app by sending traffic to an earlier revision:

```sh
gcloud run services update-traffic parlay-club \
  --project PROJECT_ID --region us-central1 \
  --to-revisions REVISION=100
```

The console path is **Cloud Run → parlay-club → Revisions → Manage traffic**. The previous revision keeps its own
`BASE_URL` and secrets; nothing else needs to change.

A traffic rollback does **not** roll back the database. Migrations run at boot and only move forward:
`PRAGMA user_version` records the last applied file, and an older binary never undoes one. If the new revision applied a
migration, an older revision can meet a schema it does not know. Roll back when the schema did not change, or roll
forward with a fix. Restoring the replica to an earlier point is a last resort; treat it as data loss and say so.

A deploy that will not boot leaves the previous revision serving unless Cloud Run has already moved traffic. The service
URL keeps working while you fix the image or a secret.

## Verify

- The workflow's last step fetches `/healthz`; a green job means the revision booted.
- `curl -s SERVICE_URL/healthz` prints `ok` from your machine.
- `SERVICE_URL/login` loads, sign-in returns to `/`, and a slip opens.
- Cloud Run logs show the startup lines and no config error. On an empty replica the first boot logs
  `no matching backups found`; that is normal, not a failure.
- The first boot after someone creates a group logs the history import with the group and the number of slips created,
  skipped, and users created. Before that it logs `history: no group yet, skipping seed/history.yaml`.
- Write a slip, deploy again, and find the slip after the new revision is live. The revision started with a fresh
  `/data`, so the slip came back from the Litestream replica.
- The app serves at phone width.

## History import after a deploy

`internal/seed` runs at boot after migrations, into the app's first group. It is idempotent by week and name, so a
restart or a second deploy adds nothing twice. A slip edited in the file after it loaded does not update; edit it in the
app, or give the entry a new name.

If no group exists, the import waits and logs. Create the first group, then deploy again. The file and its fields are in
[History file](../reference/history-file.md).

## Spend

The $5 monthly budget from setup emails the billing account's administrators and users at 50%, 90%, and 100%. Add
yourself under **Billing → Account management** if those mails never arrive.

- **Billing → Reports** shows spend by service. Cloud Run, the bucket, and the secrets are normally pennies.
- **Billing → Budgets** shows the budget and its thresholds.
- Artifact Registry keeps only the last 3 images; older ones are deleted by the setup cleanup policy. If the 0.5 GB free
  tier is a worry, check the repo size after a busy week.
- Build layers live in the GitHub Actions cache, not Artifact Registry, so caching does not add to the repo size or the
  0.5 GB free tier.
- Cloud Scheduler's free tier includes three jobs per billing account; the single Sunday morning pre-fill job
  (`parlay-club-prefill`, set up in [GCP setup](../guides/gcp-setup.md#8-schedule-the-sunday-morning-pre-fill-optional))
  stays within it.
- Odds credits are separate from GCP spend. The app tracks them in `sync_state`; see
  [Data refresh](../concepts/data-refresh.md).

## Troubleshooting

| Symptom                                              | Likely cause and fix                                                                       |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `blackout: deploys paused` and exit 1                | A game window is open. Wait, or dispatch with `force=true` and accept the write-loss risk.  |
| `blackout: cannot parse NOW`                         | `NOW` is set to something that is not ISO-8601. Unset it or use `2026-11-08T15:00:00Z`.     |
| `missing GitHub repository variables`                | One of the seven required variables is unset. Set it to the value `deploy/setup.sh` printed. |
| `BASE_URL must be an http or https URL with a host`  | Fix the `BASE_URL` variable, or unset it on the first deploy.                              |
| Deploy fails at authentication                       | A GitHub variable is wrong: `WIF_PROVIDER`, `DEPLOY_SA`, or `GCP_PROJECT`.                  |
| `cannot read secret ...`                             | `parlay-deployer` lacks `roles/secretmanager.viewer`. Re-run `deploy/setup.sh`.            |
| `secret ... has no ENABLED version`                  | Add a version, then deploy again. See [GCP setup](../guides/gcp-setup.md#3-add-a-version-to-each-required-secret). |
| `Litestream bucket ... is missing`                   | `LITESTREAM_BUCKET` is wrong, or the bucket was never created. Re-run `deploy/setup.sh`.    |
| `Artifact Registry repo ... is missing`              | `AR_REPO` or `GCP_REGION` is wrong, or the repo was never created. Re-run `deploy/setup.sh`. |
| Deploy fails building or pushing the image           | Check the build log. A denied push means `artifactregistry.writer` is missing; rerun setup. |
| Revision starts, then exits with a config message    | Read the message: a missing secret version, `BASE_URL`, or `LITESTREAM_BUCKET`.             |
| `entrypoint: set LITESTREAM_BUCKET or ...`           | The `LITESTREAM_BUCKET` variable is unset, so the entrypoint refuses to run without a backup. |
| Health check times out                               | The revision did not boot. Read Cloud Run logs for the startup error or GCS permission.     |
| `first revision had no BASE_URL; retrying` warning   | Expected on the first deploy. Set the `BASE_URL` repo variable to the printed URL.          |
| Bug reports save but no GitHub issue appears         | `github-token` has no `ENABLED` version, so the deploy skipped it. Add one and deploy again. |
| `POST /internal/prefill` returns `404`               | `prefill-token` has no `ENABLED` version, so the deploy skipped it. Add one and deploy again. |
| History is missing after a deploy                    | No group existed at boot, or it is not the first group. Create the first group, deploy again.|
| First boot logs `no matching backups found`          | The replica is empty. Normal. The revision starts a fresh database and replicates it.       |
| A deploy lost writes                                 | It ran during a game window, probably forced. Prefer waiting, or `force` only when quiet.   |
