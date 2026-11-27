---
id: adr-history-and-deploy
title: History, manual grading, and deploy
type: decision
summary: Deploy to Cloud Run inside the free tier, with the database backed up to Cloud Storage and deploys blocked during game windows.
area: infra-and-dev
---

# History, manual grading, and deploy

## Context

The app only helps if it is online on Sunday, and the group already has past parlays worth tracking.

There is no way to deploy, to keep data across restarts, to record results, or to load the group's past slips.

## Goals and non-goals

**Goals:**

- Deploy the app to Cloud Run inside the free tier before Sunday, Nov 8.
- Keep the SQLite database across restarts and deploys with Litestream backups to Cloud Storage.
- Grade legs by hand and recompute each slip's result.
- Create slips for past weeks and load the group's history from a file.
- Run CI on every change and block deploys during game windows.

**Non-goals:**

- Auto-grading from box scores or the Record Book. `grading-record-book` adds those.
- Scheduled jobs or a second Cloud Run instance.
- More paid infrastructure than the ~$20 GCP safety net covers.
- Sportsbook integrations or money handling.

## Decision

Deploy to Cloud Run inside the free tier, with the database backed up to Cloud Storage and deploys blocked during game
windows. Add hand grading of legs with automatic slip results, slips for past weeks, and a history file that loads the
group's past parlays. This is the last change before the Nov 8 target.

Heads up on `groups` and `google-oauth`: this change ships the single club first, with the history file creating users
by name. Once Google sign-in lands, those users must be matched to real accounts, and `groups` will load the history
into the club's first group. The history loader should stay easy to point at a group and a person later.

## Consequences

Parlays can now be graded leg-by-leg in the web interface. Leg updates recalculate slip results and statuses
automatically, handling wins, losses, pushes, and voids deterministically. Reopening a leg recalculates the slip state
without corrupting history.

Past-week slips can be created manually or seeded in bulk via `seed/history.yaml`. Historic slips bypass kickoff lock
restrictions and accept arbitrary leg types, allowing groups to preserve records from earlier in the season.

The application runs in Google Cloud Run within the free tier. Litestream continuously backs up SQLite transactions to
Google Cloud Storage and restores the database on container startup, maintaining state across restarts and revisions.

GitHub Actions now guards builds and deploys. CI tests every pull request and push to `main`. Deploys trigger only after
CI succeeds and are automatically blocked during Sunday and Monday game windows to prevent write conflicts and database
rollbacks.

Operating within free-tier constraints introduces minor friction:

- Deployments during game windows fail by default and require a manual workflow dispatch with `force: true`.
- History seeding requires an initial group to exist. A fresh deployment must boot once, create a group, and restart or
  redeploy to seed history slips.
- Litestream replication introduces a small write-loss window if Cloud Run stops abruptly before an LTX frame syncs.
- History seed imports match users by display name alone, collapsing duplicate names to the first match until Google
  OAuth linking is added.

## Alternatives considered

- **Automated box-score grading:** Deferred to the upcoming `grading-record-book` change. Manual grading was prioritized
  to establish deterministic slip calculations, UI feedback, and tie-breaking logic without relying on external scoring
  APIs.
- **Managed relational database (Cloud SQL / Postgres):** Rejected to preserve the $0–$5 operational cost. Litestream
  with GCS keeps SQLite persistent on single-instance Cloud Run without recurring database server costs.
- **Deploying on direct git pushes:** Rejected in favor of `workflow_run` after CI passes on `main`. This ensures only
  commits that pass all linters, template diff checks, and unit tests are deployed.
- **Cancelling in-progress deploy runs:** Rejected. `deploy.yml` uses non-cancelling concurrency queues to avoid
  overlapping container deployments that could create two active Litestream writers.
- **Allowing future typed weeks in slip creation:** Rejected. While past weeks can be typed freely to backfill history,
  typing a week beyond the latest stored slate is blocked to prevent accidental schedule corruption.

## Implementation notes

- **First deploy URL bootstrap:** Production Cloud Run instances require `BASE_URL`, but GCP does not assign the service
  URL until the first deploy succeeds. `deploy.yml` deploys without `BASE_URL` on initial run, captures the assigned
  URL, and redeploys with the variable configured.
- **Litestream 0.5 file replica structure:** Litestream 0.5 uses directory-based LTX structures rather than flat files.
  Entrypoint scripts and local container testing account for this layout.
- **Atomic seed validation:** `seed/history.yaml` validates the entire file before applying writes. Any schema error,
  invalid market, or bad result prevents any rows from importing. Slips already present in the database are skipped
  completely and cannot be updated through the file.
- **Artifact Registry service agent permission:** Cloud Run requires `roles/artifactregistry.reader` explicitly granted
  to the Cloud Run service agent to pull images from Artifact Registry. This was added to `deploy/setup.sh`.
- **Known limits:**
  - Leg outcome menus expose `WON`, `LOST`, `PUSH`, `VOID`, and `PENDING`; `NEEDS_GRADING` is reserved for automated
    graders.
  - Blackout windows are hardcoded to Sunday 09:00–23:59 and Monday 20:00–23:59 ET. Thursday and Saturday games are not
    automatically gated.
  - `seed/history.yaml` runs only at startup and only imports into the first group found.

## Follow-ups

- Implement automated box score grading and record book tracking (`grading-record-book`).
- Update `internal/seed` user resolution to match Google OAuth email addresses or aliases once account authentication is
  required.
- Add dynamic schedule detection to `deploy/blackout.sh` to gate deploys during Thursday night and holiday games.
- Add input range validation for odds and points on the manual past-week leg entry form.
