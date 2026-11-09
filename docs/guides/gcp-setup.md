---
id: gcp-setup
title: GCP setup
type: guide
summary: Create the Google Cloud project resources once, add the secrets and GitHub variables, and run the first deploy that puts the app on Cloud Run.
tags: [gcp, setup, cloud-run, artifact-registry, secret-manager, workload-identity, github, deploy, litestream]
sources: [deploy/setup.sh, deploy/entrypoint.sh, .github/workflows/deploy.yml, Dockerfile, internal/config/config.go, .github/workflows/ci.yml]
last_verified: 2026-11-09
---

# GCP setup

## Purpose

Take a Google Cloud project with billing and turn it into everything the app needs to run on Cloud Run: the image repo,
the Litestream bucket, the secrets, the two service accounts, GitHub's keyless login, and a spend alert. Then set the
GitHub variables and run the first deploy.

You run this once per project. Every step is safe to repeat, and the setup script is idempotent.

## Preconditions

- A Google Cloud project with billing enabled. Note the project ID, not the display name.
- Your account is an **Owner** on the project and a **Billing Account Administrator** on the billing account. The script
  grants IAM roles and creates a budget, which need both.
- The [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) is installed and signed in:

  ```sh
  gcloud auth login
  gcloud config set project PROJECT_ID
  ```

- Admin access to the GitHub repo, enough to set Actions variables. The repo is `kalverra/parlay-club` unless you forked
  it.
- Access to [Parlay API](https://parlay-api.com) for a key, or nothing if you skip odds.

## Steps

### 1. Create the resources

Run the script from the repo root. The second argument is the billing account ID, which looks like
`012345-6789AB-CDEF01`; find it with `gcloud billing accounts list`.

```sh
deploy/setup.sh PROJECT_ID BILLING_ACCOUNT
```

The script stops at the first failure and prints what it finished. It creates:

| Resource             | Details                                                                                   |
| -------------------- | ----------------------------------------------------------------------------------------- |
| APIs                 | Cloud Run, Artifact Registry, Secret Manager, IAM, STS, billing budgets, GCS              |
| GCS bucket           | `gs://PROJECT_ID-parlay-litestream` in `us-central1`, for Litestream                      |
| Artifact Registry    | Docker repo `parlay-club` with a cleanup policy that keeps the last 3 images               |
| Secrets              | `session-key`, `google-client-id`, `google-client-secret`, `odds-api-key`, all empty       |
| Runtime account      | `parlay-runtime`, read/write on the bucket and read on the four secrets                   |
| Deployer account     | `parlay-deployer`, `run.admin` and `artifactregistry.writer`, act-as on the runtime account |
| Cloud Run agent      | read on the image repo, so Cloud Run can pull the image                                   |
| Workload Identity    | pool `github` and provider `github`, limited to `kalverra/parlay-club`                    |
| Billing budget       | `$5` a month with email alerts at 50%, 90%, and 100%                                       |

Override defaults with environment variables when your names differ:

| Variable        | Default                           |
| --------------- | --------------------------------- |
| `REGION`        | `us-central1`                     |
| `GITHUB_REPO`   | `kalverra/parlay-club`            |
| `SERVICE`       | `parlay-club`                     |
| `AR_REPO`       | `parlay-club`                     |
| `BUCKET`        | `PROJECT_ID-parlay-litestream`    |
| `BUDGET_AMOUNT` | `5`                               |

For a fork, run it like this so GitHub can use the deployer account:

```sh
GITHUB_REPO=you/parlay-club deploy/setup.sh PROJECT_ID BILLING_ACCOUNT
```

The script prints the values you need in the next steps. Copy them.

### 2. Create the Google OAuth client

The app needs a Web client before it can boot in prod. Follow [Google OAuth setup](../runbooks/google-oauth-setup.md) to
create it, set the consent screen to production, and register the local redirect URI. Leave the prod redirect URI for
step 6, after the Cloud Run URL exists.

### 3. Add a version to every secret

Cloud Run refuses to start a revision that points at a secret with no version, so add one to all four now.

The session key signs cookies. Generate 32 or more random bytes and keep it secret:

```sh
openssl rand -base64 48 | tr -d '\n' | gcloud secrets versions add session-key --data-file=-
```

Add the Google client values from step 2:

```sh
printf '%s' "$GOOGLE_CLIENT_ID" | gcloud secrets versions add google-client-id --data-file=-
printf '%s' "$GOOGLE_CLIENT_SECRET" | gcloud secrets versions add google-client-secret --data-file=-
```

Add the Parlay API key. An empty value disables odds calls, which is fine for a first deploy:

```sh
printf '%s' "$ODDS_API_KEY" | gcloud secrets versions add odds-api-key --data-file=-
```

To replace a value later, add a new version of that secret and deploy again. `:latest` points Cloud Run at the newest
version, so no flag changes.

### 4. Set the GitHub repository variables

The deploy workflow reads these from **Settings → Secrets and variables → Actions → Variables**. They are variables, not
secrets: none of them is sensitive.

| Variable            | Value                                                   |
| ------------------- | ------------------------------------------------------- |
| `GCP_PROJECT`       | the project ID                                          |
| `GCP_REGION`        | `us-central1` unless you changed it                     |
| `WIF_PROVIDER`      | the full Workload Identity provider name the script prints |
| `DEPLOY_SA`         | `parlay-deployer@PROJECT_ID.iam.gserviceaccount.com`    |
| `RUNTIME_SA`        | `parlay-runtime@PROJECT_ID.iam.gserviceaccount.com`     |
| `LITESTREAM_BUCKET` | the bucket name, without `gs://`                        |
| `AR_REPO`           | `parlay-club` unless you changed it                     |

`BASE_URL` comes after the first deploy, in step 6. Set all of these at once with the GitHub CLI:

```sh
gh variable set GCP_PROJECT --body PROJECT_ID
gh variable set GCP_REGION --body us-central1
gh variable set WIF_PROVIDER --body 'projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/github/providers/github'
gh variable set DEPLOY_SA --body 'parlay-deployer@PROJECT_ID.iam.gserviceaccount.com'
gh variable set RUNTIME_SA --body 'parlay-runtime@PROJECT_ID.iam.gserviceaccount.com'
gh variable set LITESTREAM_BUCKET --body 'PROJECT_ID-parlay-litestream'
gh variable set AR_REPO --body parlay-club
```

The setup script prints the exact `gh` commands with your values filled in.

### 5. Run the first deploy

Open **Actions → Deploy → Run workflow**, choose `main`, and leave `force` off unless a game is on; the
[deploy runbook](../runbooks/deploy.md#the-blackout) explains the gate. The first deploy takes a few minutes.

The app needs `BASE_URL`, and no service URL exists yet, so the first revision can fail to boot. The workflow notices,
reads the new service's URL, and redeploys once with it. A yellow `first revision had no BASE_URL; retrying with ...`
warning is expected on this run, not an error.

When the job turns green it prints the URL in the summary and verifies `GET /healthz`. Open the URL. You should get the
login page, and `GET URL/healthz` should print `ok`.

### 6. Pin the URL and the redirect

Copy the URL from the job summary, then:

1. Set the repo variable so later deploys pass it without the retry:

   ```sh
   gh variable set BASE_URL --body 'https://SERVICE_URL'
   ```

2. Add the prod callback to the Google client from step 2:

   ```text
   https://SERVICE_URL/auth/google/callback
   ```

   Use `https` and no trailing slash. It must match `BASE_URL` exactly, or sign-in stops at Google with
   `redirect_uri_mismatch`.

### 7. Create a group and restart so history imports

`seed/history.yaml` imports into the app's first group, the group with the lowest id. With no group it logs
`history: no group yet, skipping seed/history.yaml` and waits.

1. Sign in at `SERVICE_URL/login`. The first sign-in creates a "Just me" group; that is the first group, so it receives
   the history. If history belongs in a shared group, that group must be the first one created.
2. Run **Actions → Deploy → Run workflow** again, or update any env var, to start a new revision. Boot reruns the
   import.
3. Open the group and check the slips. The boot log names the group and counts the slips created, skipped, and users
   created.

The file is idempotent by week and name, so a restart adds nothing twice. To change a slip that already loaded, edit it
in the app or give the file entry a new name. An edit to the file is not a re-import of the same slip.

## Verify

- `curl -s SERVICE_URL/healthz` prints `ok`.
- `SERVICE_URL/login` loads at phone width with a Google button.
- Sign-in returns to `/` and sets `pc_session`.
- The first revision's logs show `no matching backups found`, which is normal for an empty replica. Later boots restore
  from `gs://LITESTREAM_BUCKET`.
- Write a slip, deploy again, and the slip is still there. A new revision starts with a fresh `/data`, so the data came
  back from the Litestream replica.
- The first boot's history log does not say `no group yet` after step 7.
- **Billing → Budgets** shows the $5 budget and its alert thresholds.

## Spend

The budget is the backstop, not a hard cap; Google emails the billing account's administrators and users at 50%, 90%,
and 100%. Add yourself under **Billing → Account management** if you do not get the mail. The console's
**Billing → Reports** page breaks spend down by service.

The free-tier pieces this app leans on:

- Cloud Run scales to zero (`--min-instances 0`) and runs at most one instance.
- Artifact Registry keeps only the last 3 images.
- The bucket holds the SQLite replica, which is small.
- Parlay API has its own 1,000-credit monthly budget, tracked in the app; see
  [Data refresh](../concepts/data-refresh.md).

## Troubleshooting

| Symptom                                              | Likely cause and fix                                                                     |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `setup: gcloud not found`                            | Install the Cloud SDK and reopen the shell.                                              |
| `cannot read project`                                | Wrong project ID, or your account cannot see it. Check `gcloud projects list`.           |
| Permission denied while granting roles or creating a budget | You need Owner on the project and Billing Account Administrator on the billing account. |
| The deploy cannot authenticate to Google Cloud       | A GitHub variable is wrong or missing. Recheck `WIF_PROVIDER`, `DEPLOY_SA`, `GCP_PROJECT`. |
| Startup says `GOOGLE_CLIENT_ID is required in prod`  | `google-client-id` has no version, or the client value is empty. Add one.                |
| Startup says `BASE_URL is required in prod`          | Set the `BASE_URL` repo variable, then deploy again.                                     |
| Startup says `entrypoint: set LITESTREAM_BUCKET`     | `LITESTREAM_BUCKET` is missing from the repo variables.                                  |
| `secret ... not found` on deploy                     | Run `deploy/setup.sh` first; it creates the four secrets.                                |
| Cloud Run cannot pull the image                      | Re-run `deploy/setup.sh`; it grants the Cloud Run service agent read on the repo.        |
| History does not appear                              | No group existed at boot, or the history belongs to another group. Create the first group and redeploy. |
| Budget emails never arrive                           | Add yourself as a billing administrator or user on the billing account.                  |
