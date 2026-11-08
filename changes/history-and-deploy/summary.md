---
slug: history-and-deploy
run: 20261108-131219-history-and-deploy
outcome: done
generated: 2026-11-08T14:01:45Z
---

# Run summary: history-and-deploy

## Outcome

- Status: done
- Branch: history-and-deploy
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 11 of 11
- Phases: 4
- Checkpoints: 12
- Duration: 42m2s
- Cost: $2.14
- Input tokens: 32371340
- Output tokens: 271548
- Cache read tokens: 26863490

## Phases

### Phase 1: Grading by hand

Tasks:

- [x] Add SetLegResult with slip result and status recompute, its table tests for every Grading case, and the result chip menu on each leg with PUT /legs/{id}/result
- [x] Allow past weeks in the create-slip form and test a past-week slip with free-text OTHER legs
- [x] Docs: add grading to docs/concepts/slips.md; delete the manual grading parts from plan/product.md

Phase totals: 3 checkpoints, 13m50s, $0.78, 17593752 input / 93830 output / 15677034 cache tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

- `internal/slips.Service.SetLegResult` grades one leg by hand. It checks membership, records the acting member in
  `graded_by`, recomputes the slip result and status from every leg, writes one activity line, and publishes
  `slip:{id}`. Any slip member can grade any leg, including one they do not own.
- `db.SetLegResult` and `db.SetSlipGrade` write the two rows, with `db.ErrLegResultInvalid` and
  `db.ErrSlipResultInvalid` guarding the schema values. `parlay.Result.IsLegResult` and `IsSlipResult` hold the
  allowed sets.
- Status follows the slip: `GRADED` once the result settles, otherwise `LOCKED` when every member has locked in, else
  `OPEN`. Resetting a leg to `PENDING` reopens a graded slip.
- The result chip on each leg opens a menu of `PENDING`, `WON`, `LOST`, `PUSH`, and `VOID` and sends
  `PUT /legs/{id}/result`. Non-members see a plain chip. An unknown result answers 422 with "that result isn't
  valid".
- The create-slip form takes a typed season, season type, and week number for a week the slate never stored, and
  upserts that week. The stored-week picker now labels each option with its season, and a typed week later than the
  latest stored week is refused. Past weeks skip kickoff locks as before, so a free-text `OTHER` leg with no game
  works.
- Tests cover every Grading case in `plan/product.md`: all won, an early loss, pending and needs-grading holds, a
  `PUSH` on an exact line, a `VOID` leg, all push/void, reopening and relocking, membership refusal, unknown result,
  repeated grade as a no-op, and an override by another member. The web tests cover the chip menu, the 422, the
  past-week `OTHER` leg, and the future-week refusal.
- Docs: grading added to `docs/concepts/slips.md`, the new route in `docs/reference/routes.md`, the summary in
  `docs/llms.txt`, and the manual grading rules removed from `plan/product.md` (the auto-grading rules stay).
- `go test ./...` and `golangci-lint run ./...` pass.

### Phase 2: History file

Tasks:

- [x] Add internal/seed and an empty seed/history.yaml with a commented example, load it at boot, and test that loading twice creates each slip once, unknown users are created, and results compute slip result
- [x] Docs: add docs/reference/history-file.md; delete History from plan/product.md

Phase totals: 2 checkpoints, 8m13s, $0.41, 6067953 input / 54250 output / 5030326 cache tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

- `internal/seed.Load` reads `seed/history.yaml` and imports its slips into a group. It returns a `Report` with the
  slips created, slips skipped, and users created.
- The whole file is decoded with known fields only and validated before the first write: required season, season type,
  week, name, and at least one leg; `legs_per_member` from 1 to 5; each user at most that many legs; a known market and
  a valid leg result on every leg. Problems are joined and name the slip and leg. A file with any problem imports
  nothing.
- A slip is keyed by its week and name within the group. An existing slip is skipped before any users are resolved or
  legs added, so a second load creates no slips, users, legs, or activity.
- Each person resolves through `resolveUser`: a name match ignoring case, or a new active user with no Google identity.
  Names resolve through the new `db.Store.UserByName`; the target group comes from the new `db.Store.FirstGroup`.
- A missing slip is built in one transaction: `UpsertWeek`, `CreateSlip`, then `Service.AddLeg` and
  `Service.SetLegResult` for every leg whose result is not `PENDING`. The leg's owner grades it, so `graded_by` holds
  their name and the slip result and status settle the normal way.
- Defaults match the app: a missing market is `OTHER`, a missing result is `PENDING`, missing odds are `-110`, and a
  missing `legs_per_member` is 2. An `ANYTIME_TD` leg without a side gets `YES`.
- `cmd/server/main.go` calls `loadHistory` after migrations. It logs the group and the three counts. No group yet, a
  missing file, or a bad file logs and lets the app serve.
- `seed/history.yaml` ships comments only, with a commented example.
- Tests cover created slips, users, and computed results (won, lost, and a push that leaves the rest to win); loading
  twice; the shipped empty file; a missing file; and a validation table for unknown result, unknown market, missing
  selection, missing user, too many legs, missing name, a week out of range, a bad season type, and an unknown field.
- Docs: added `docs/reference/history-file.md`, added it to `docs/llms.txt`, and deleted History from
  `plan/product.md`.
- `go test ./...` and `golangci-lint run ./...` pass.

### Phase 3: Container and persistence

Tasks:

- [x] Add the Dockerfile, deploy/litestream.yml, and deploy/entrypoint.sh with a pinned Litestream release, and smoke test locally: build, run with a file replica, write a slip, restart, and confirm it restored
- [x] Docs: add docs/concepts/persistence.md; delete Persistence from plan/architecture.md

Phase totals: 2 checkpoints, 8m21s, $0.41, 4038655 input / 40691 output / 2893458 cache tokens

Checkpoints: task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

- `Dockerfile` is a three-stage build: `golang:1.27.3-alpine` builds `./cmd/server` with `CGO_ENABLED=0`,
  `-trimpath`, and stripped flags; `litestream/litestream:0.5.18` supplies the pinned binary; `alpine:3.22` is the
  runtime. The runtime copies the server, the Litestream binary, `deploy/litestream.yml`, `deploy/entrypoint.sh`,
  `seed/`, and `web/`, creates `/data` owned by a non-root `app` user, and defaults `APP_ENV=prod`,
  `DB_PATH=/data/parlay.db`, `PORT=8080`.
- `.dockerignore` keeps `.git`, `.good-vibes`, `.rumdl_cache`, `data`, `tmp`, and `.env` out of the build context.
- `deploy/litestream.yml` is 0.5 syntax: one `dbs` entry whose `path` is `${DB_PATH}` and whose `replica.url` is
  `${LITESTREAM_REPLICA_URL}`, both read from the environment.
- `deploy/entrypoint.sh` defaults `DB_PATH` to `/data/parlay.db`, derives the replica URL from `LITESTREAM_BUCKET` when
  `LITESTREAM_REPLICA_URL` is unset, exits 1 when neither is set, restores with `-if-db-not-exists` and
  `-if-replica-exists`, then `exec`s `litestream replicate -exec /app/server` so signals reach both processes.
- Local container smoke test passed. The image built, ran with `APP_ENV=dev` and
  `LITESTREAM_REPLICA_URL=file:///replica/parlay.db`, signed in through `POST /dev/login`, created a group, created a
  past-week slip (`2026` regular season, week 7, one member), and served it. After an LTX snapshot appeared under the
  replica path, the container stopped cleanly. A second container started with a fresh `/data` volume and the same
  replica, and the slip page still showed `Week 7`, so the restore path works.
- Docs: added `docs/concepts/persistence.md` (one writer, container boot, replication and shutdown, the deploy
  two-writer gap, local restore, guardrails), added it to `docs/llms.txt`, and deleted the Persistence section from
  `plan/architecture.md`.
- `go test ./...` and `golangci-lint run ./...` pass, and `good-vibes docs lint` reports `ok`.

### Phase 4: CI and deploy

Tasks:

- [x] Add .github/workflows/ci.yml with generate-diff check, lint, and tests
- [x] Add deploy/blackout.sh with a test script covering both windows via NOW, and deploy/setup.sh for every resource in plan/architecture.md One-time setup
- [x] Add .github/workflows/deploy.yml with blackout gate, WIF auth, build, push, and gcloud run deploy
- [x] Docs: add docs/runbooks/deploy.md and docs/guides/gcp-setup.md; delete Deploy and CI from plan/architecture.md

Phase totals: 5 checkpoints, 11m38s, $0.54, 4670980 input / 82777 output / 3262672 cache tokens

Checkpoints: task-8, task-9, task-10, task-11, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
