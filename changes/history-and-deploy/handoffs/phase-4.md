# Phase 4 handoff: CI and deploy

## Done

- `.github/workflows/ci.yml` runs on pull requests and pushes to `main`. `jdx/mise-action` installs Go, golangci-lint,
  and the pinned Tailwind CLI; the job then runs `go generate ./...`, `git diff --exit-code` (a template change without a
  regenerated `app.css` fails), `golangci-lint run ./...`, `go test -short -race ./...`, and `deploy/blackout_test.sh`.
  The blackout test is in CI because it guards a deploy gate, so a broken gate fails before it can allow or block a
  deploy.
- `.github/workflows/deploy.yml` triggers on `workflow_run` after CI succeeds on `main`, and on manual
  `workflow_dispatch` with a `force` boolean. It refuses a `workflow_run` that came from a pull request, checks out the
  commit CI ran (`head_sha`), and queues behind a non-cancelling `deploy` concurrency group. The job runs the blackout
  gate (skipped only when `force` is true), authenticates through Workload Identity as `parlay-deployer`, builds and
  pushes the image tagged with the commit SHA, deploys with one instance at most and the four Secret Manager secrets,
  verifies `GET /healthz`, and writes the image and URL to the job summary.
- `deploy/blackout.sh` exits 1 from Sunday 09:00 through 23:59 and Monday 20:00 through 23:59 in
  `America/New_York`, 0 outside them, and 2 when `NOW` cannot be parsed. It accepts `Z` and `±hh:mm` offsets and falls
  back from GNU `date` to BSD `date`. `deploy/blackout_test.sh` covers both windows, both sides of every edge, EST and
  EDT, offset timestamps, and bad input: 14 cases, all passing.
- `deploy/setup.sh PROJECT_ID BILLING_ACCOUNT` is idempotent and creates the APIs, the Litestream bucket, the Artifact
  Registry repo with a keep-last-three cleanup policy, the four empty secrets, `parlay-runtime` and `parlay-deployer`
  with their roles, read on the image repo for the Cloud Run service agent, the Workload Identity pool and GitHub
  provider restricted to `kalverra/parlay-club`, and the $5 budget with 50/90/100% alerts. It prints the GitHub
  variables, the matching `gh variable set` commands, and the three follow-up steps.
- Docs: added `docs/runbooks/deploy.md` (triggers, blackout and force, workflow steps, manual deploy, rollback, verify,
  history import, spend, troubleshooting) and `docs/guides/gcp-setup.md` (setup through the first deploy, URL pinning,
  and the history restart), registered both in `docs/llms.txt`, deleted Deploy and CI from `plan/architecture.md`, and
  fixed the `plan/README.md` wording for that doc.
- Verified: `go test -short ./...` passes, `golangci-lint run ./...` reports 0 issues, `go generate ./...` leaves no
  unstaged diff, `actionlint` and `typos` are clean, and `shellcheck` is clean on `deploy/*.sh`.

## Deviations

- The deploy trigger is `workflow_run` plus `workflow_dispatch`, not a plain push trigger. That is what lets the workflow
  check out the exact commit CI tested even if `main` moved, and lets it refuse runs that came from a pull request.
- The first deploy has no `BASE_URL`, and prod refuses to boot without it. The workflow deploys once without it, reads
  the new service URL, and redeploys once with it; a failed first revision is expected, not fatal. The guide and runbook
  say the yellow `first revision had no BASE_URL; retrying` warning is normal.
- `concurrency: deploy` queues with `cancel-in-progress: false`. Cancelling a deploy mid-flight is one way to get two
  revisions overlapping, which is what the blackout exists to avoid.
- `deploy/setup.sh` also grants the Cloud Run service agent `roles/artifactregistry.reader` on the image repo. The
  architecture's setup list did not name that binding, but without it the first revision cannot pull its image.
- `deploy/blackout.sh` exits 2 on an unparsable `NOW` instead of treating it as clear, so a typo in the clock cannot
  silently allow a deploy.
- `rumdl` reflowed line length in the two new docs. Repo-wide markdown has older MD013 findings that this phase did not
  touch.

## Open issues

- No workflow and no part of `deploy/setup.sh` has run against real GCP or GitHub; the agent cannot. The setup script,
  the first deploy, the BASE_URL retry, and the GCS restore are all first exercised by the user. The runbooks are the
  only check until then.
- `workflow_run` only fires once `deploy.yml` is on the default branch, and the trigger assumes `main`. Until the file
  merges, only the manual dispatch path exists.
- The Workload Identity provider's attribute condition is pinned to `kalverra/parlay-club`. A fork must run setup with
  `GITHUB_REPO=owner/repo`, or GitHub cannot assume the deployer.
- Setup creates the secrets empty and does not add versions. The first deploy fails until all four have a version;
  `odds-api-key` may hold an empty value, but the secret still needs one.
- The blackout windows are hardcoded to Sunday 09:00 and Monday 20:00 New York. A weekday or Thursday game does not gate
  a deploy; that is acceptable for now and the runbook says so.
- Restore from a GCS replica was still not exercised end to end. The first Cloud Run deploy is the real check.

## Next-phase notes

- This was the last phase. The change closes with a close-out, then `good-vibes done history-and-deploy`.
- The user's order of operations: run `deploy/setup.sh`, add a version to all four secrets, set the seven repository
  variables, run the first deploy, set `BASE_URL` and the prod OAuth redirect, then create the first group and deploy
  again so `seed/history.yaml` imports.
- Deploy outside the game windows. A forced deploy inside one accepts the write-loss gap it exists to prevent.
- A new revision always starts with a fresh `/data`, so it restores from GCS. Rerunning boot, including for the history
  import, means dispatching the workflow again.

## Files touched

- `.github/workflows/ci.yml`
- `.github/workflows/deploy.yml`
- `deploy/blackout.sh`
- `deploy/blackout_test.sh`
- `deploy/setup.sh`
- `docs/guides/gcp-setup.md`
- `docs/llms.txt`
- `docs/runbooks/deploy.md`
- `plan/README.md`
- `plan/architecture.md`

## Run log

- Mode: 
- Checkpoints: task-8, task-9, task-10, task-11
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
