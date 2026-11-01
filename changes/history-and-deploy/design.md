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

## Risks

- Litestream config syntax differs by version: pin one release and confirm against its docs.
- Two writers during a deploy: covered by the blackout; documented in the persistence concept doc.
- Free-tier drift: the $5 budget alert is the backstop. The deploy runbook says how to check spend.
- The agent can't run GCP or GitHub setup; the runbook must let the user finish alone.

## TODO

### Phase 1: Grading by hand

- [ ] Add SetLegResult with slip result and status recompute; table tests for every Grading case
- [ ] Add the result chip menu on each leg and PUT /legs/{id}/result
- [ ] Allow past weeks in the create-slip form; test a past-week slip with free-text OTHER legs
- [ ] Docs: add grading to docs/concepts/slips.md; delete the manual grading parts from plan/product.md

### Phase 2: History file

- [ ] Add internal/seed and an empty seed/history.yaml with a commented example; load it at boot
- [ ] Tests: loading twice creates each slip once; unknown users are created; results compute slip result
- [ ] Docs: add docs/reference/history-file.md; delete History from plan/product.md

### Phase 3: Container and persistence

- [ ] Add the Dockerfile, deploy/litestream.yml, and deploy/entrypoint.sh with a pinned Litestream release
- [ ] Smoke test locally: build, run with a file replica, write a slip, restart, and confirm it restored
- [ ] Docs: add docs/concepts/persistence.md; delete Persistence from plan/architecture.md

### Phase 4: CI and deploy

- [ ] Add .github/workflows/ci.yml with generate-diff check, lint, and tests
- [ ] Add deploy/blackout.sh with a test script covering both windows via NOW
- [ ] Add deploy/setup.sh for every resource in plan/architecture.md One-time setup
- [ ] Add .github/workflows/deploy.yml with blackout gate, WIF auth, build, push, and gcloud run deploy
- [ ] Docs: add docs/runbooks/deploy.md and docs/guides/gcp-setup.md; delete Deploy and CI from plan/architecture.md
