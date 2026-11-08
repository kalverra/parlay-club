---
id: deploy
title: Deploy
type: runbook
summary: How a commit reaches Cloud Run, why deploys pause during game windows, how to force one, and how to verify, roll back, and read the spend.
tags: [deploy, cloud-run, github-actions, blackout, litestream, rollback, runbook]
sources: [.github/workflows/deploy.yml, .github/workflows/ci.yml, deploy/blackout.sh, deploy/entrypoint.sh, docs/concepts/persistence.md, internal/seed/seed.go, cmd/server/main.go, internal/db/migrate.go]
last_verified: 2026-11-08
---

# Deploy

## Purpose

Ship a commit to Cloud Run without losing writes, and know how to check, undo, and pay for it. The one-time project
setup lives in [GCP setup](../guides/gcp-setup.md); this runbook covers every deploy after it.

## Preconditions

- The [GCP setup](../guides/gcp-setup.md) is finished: the bucket, secrets, service accounts, Workload Identity, and
  GitHub variables all exist.
- Every secret has a version. Cloud Run will not start a revision that points at an empty secret.
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

`deploy/blackout.sh` refuses a deploy from Sunday 09:00 through 23:59 and Monday 20:00 through 23:59, in
`America/New_York`, daylight saving included. It exits 1 inside those windows and 0 outside them.

The reason is the database. A new revision restores `/data/parlay.db` from the Litestream replica while the old revision
may still be writing. Writes made in that gap can be lost. Outside the game windows the app is quiet enough that the gap
costs nothing; during a game it is the busiest it gets. See
[Persistence](../concepts/persistence.md#two-writers-during-a-deploy).

To deploy during a window, run the workflow manually and set `force`. Accept the write-loss risk first; a forced deploy
is the only way past the gate.

Check the gate locally by pinning the clock with `NOW`:

```sh
NOW=2026-11-08T15:00:00Z deploy/blackout.sh   # Sunday 10:00 New York: exits 1
NOW=2026-11-09T15:00:00Z deploy/blackout.sh   # Monday 10:00 New York: exits 0
```

`deploy/blackout_test.sh` covers both windows, both sides of their edges, and both EST and EDT. CI runs it on every
push, so a broken gate fails before it can block or allow a deploy.

### What the workflow does

1. Checks out the tested commit. For a CI-triggered run that is the commit CI ran, not whatever `main` points at now.
2. Runs the blackout gate unless `force` is set.
3. Authenticates to Google Cloud through Workload Identity Federation as `parlay-deployer`. No service account key is
   stored in GitHub.
4. Builds the `Dockerfile` and pushes the image to Artifact Registry tagged with the commit SHA.
5. Runs `gcloud run deploy` with one instance at most, concurrency 80, 1 CPU, 512 MiB, a 3600 second timeout, CPU boost,
   public access, the runtime account, `APP_ENV=prod`, `DB_PATH=/data/parlay.db`, `LITESTREAM_BUCKET`, `BASE_URL`, and
   the four Secret Manager secrets.
6. Reads the service URL and retries once with `BASE_URL` set to that URL if the first revision could not boot. That is
   the first-deploy path; later deploys get `BASE_URL` from the repo variable.
7. Waits for `GET /healthz` on the new revision, then writes the image and URL to the job summary.

The image holds the server, the pinned Litestream binary, its config, the entrypoint, `seed/history.yaml`, and `web/`.
The entrypoint restores the database, then runs the server under `litestream replicate`.

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
- Odds credits are separate from GCP spend. The app tracks them in `sync_state`; see
  [Data refresh](../concepts/data-refresh.md).

## Troubleshooting

| Symptom                                              | Likely cause and fix                                                                       |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `blackout: deploys paused` and exit 1                | A game window is open. Wait, or dispatch with `force=true` and accept the write-loss risk.  |
| `blackout: cannot parse NOW`                         | `NOW` is set to something that is not ISO-8601. Unset it or use `2026-11-08T15:00:00Z`.     |
| Deploy fails at authentication                       | A GitHub variable is wrong: `WIF_PROVIDER`, `DEPLOY_SA`, or `GCP_PROJECT`.                  |
| Deploy fails building or pushing the image           | Check the build log. A denied push means `artifactregistry.writer` is missing; rerun setup. |
| Revision starts, then exits with a config message    | Read the message: a missing secret version, `BASE_URL`, or `LITESTREAM_BUCKET`.             |
| `entrypoint: set LITESTREAM_BUCKET or ...`           | The `LITESTREAM_BUCKET` variable is unset, so the entrypoint refuses to run without a backup. |
| Health check times out                               | The revision did not boot. Read Cloud Run logs for the startup error or GCS permission.     |
| `secret ... has no versions`                          | Add a version, then deploy again. See [GCP setup](../guides/gcp-setup.md#3-add-a-version-to-every-secret). |
| `first revision had no BASE_URL; retrying` warning   | Expected on the first deploy. Set the `BASE_URL` repo variable to the printed URL.          |
| History is missing after a deploy                    | No group existed at boot, or it is not the first group. Create the first group, deploy again.|
| First boot logs `no matching backups found`          | The replica is empty. Normal. The revision starts a fresh database and replicates it.       |
| A deploy lost writes                                 | It ran during a game window, probably forced. Prefer waiting, or `force` only when quiet.   |
