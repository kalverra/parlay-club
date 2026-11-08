# Design: History, manual grading, and deploy

## Context

The app works locally. This change makes results recordable, loads past slips, and puts the app on Cloud Run before
Sunday, Nov 8. It runs interactively: the user runs the GCP setup and the first deploy. Read before starting:

- `plan/product.md`: Grading (manual parts and slip result), History.
- `plan/architecture.md`: Persistence, Deploy, CI.
- `docs/concepts/slips.md`, `docs/reference/config.md`.

## Approach

- Leg results and slip results live in `internal/slips`: `SetLegResult` enforces membership, records `graded_by`,
  recomputes the slip result and status, writes activity, and publishes.
- The create-slip form accepts any past season and week; `internal/slips` already skips kickoff rules for past weeks.
- `internal/seed` reads `seed/history.yaml` at boot after migrations. Each slip is keyed by week and name; existing
  slips are skipped. It goes through `internal/slips` so results and statuses are computed the normal way.
- Container: multi-stage `Dockerfile` (Go build, then alpine with a pinned Litestream). `deploy/entrypoint.sh` restores
  then runs `litestream replicate -exec`. A `LITESTREAM_REPLICA_URL` override lets local runs use a `file` replica.
- `deploy/setup.sh` is idempotent `gcloud` and prints the GitHub variables. The agent writes it; the user runs it.
- Workflows: `ci.yml` and `deploy.yml` per `plan/architecture.md` CI, with `deploy/blackout.sh` gating deploys.
- Groups and `google-oauth` later: `internal/seed` resolves each person through one function. Today it creates a user by
  name; once Google sign-in lands, that function matches by email or an alias, and `groups` points it at a group. Keep
  `seed/history.yaml` free of group fields for now.

## Verification

- `go test ./...` and `golangci-lint run ./...` pass.
- `SetLegResult` table tests cover every case in `plan/product.md` "Grading", including a VOID leg and a PUSH on an
  exact line; a past-week slip accepts free-text `OTHER` legs.
- Loading the history file twice creates each slip once, creates unknown users, and computes each slip result from its
  legs.
- A local container smoke test writes a slip, restarts the container, and confirms Litestream restored the data.
- `deploy/blackout.sh` exits 1 inside both game windows and 0 outside them, using `NOW`.
- `ci.yml` runs the generate-diff check, lint, and tests; `deploy.yml` refuses a blacked-out deploy unless `force` is
  set.
- A user can follow the setup runbook alone and reach a working Cloud Run URL with data that survives a restart.

## Risks

- Litestream config syntax differs by version: pin one release and confirm against its docs.
- Two writers during a deploy: covered by the blackout; documented in the persistence concept doc.
- Free-tier drift: the $5 budget alert is the backstop. The deploy runbook says how to check spend.
- The agent can't run GCP or GitHub setup; the runbook must let the user finish alone.

## TODO

### Phase 1: Grading by hand

- [x] Add SetLegResult with slip result and status recompute, its table tests for every Grading case, and the result
  chip menu on each leg with PUT /legs/{id}/result
- [ ] Allow past weeks in the create-slip form and test a past-week slip with free-text OTHER legs
- [ ] Docs: add grading to docs/concepts/slips.md; delete the manual grading parts from plan/product.md

### Phase 2: History file

- [ ] Add internal/seed and an empty seed/history.yaml with a commented example, load it at boot, and test that loading
  twice creates each slip once, unknown users are created, and results compute slip result
- [ ] Docs: add docs/reference/history-file.md; delete History from plan/product.md

### Phase 3: Container and persistence

- [ ] Add the Dockerfile, deploy/litestream.yml, and deploy/entrypoint.sh with a pinned Litestream release, and smoke
  test locally: build, run with a file replica, write a slip, restart, and confirm it restored
- [ ] Docs: add docs/concepts/persistence.md; delete Persistence from plan/architecture.md

### Phase 4: CI and deploy

- [ ] Add .github/workflows/ci.yml with generate-diff check, lint, and tests
- [ ] Add deploy/blackout.sh with a test script covering both windows via NOW, and deploy/setup.sh for every resource in
  plan/architecture.md One-time setup
- [ ] Add .github/workflows/deploy.yml with blackout gate, WIF auth, build, push, and gcloud run deploy
- [ ] Docs: add docs/runbooks/deploy.md and docs/guides/gcp-setup.md; delete Deploy and CI from plan/architecture.md
