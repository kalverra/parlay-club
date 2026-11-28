# Phase 1: Phase 1: Scale Cloud Run to 2 vCPU / 2 GiB / Gen 2 and align region defaults

## Done

### Task 1: Update `.github/workflows/deploy.yml` to pass `--cpu 2`, `--memory 2Gi`, and `--execution-environment=gen2` in `run_deploy`, and add `.github/workflows/deploy.yml` to the `Check for runtime changes` `git diff` path list; verify with `actionlint`.

- Added `.github/workflows/deploy.yml` to the `git diff --quiet` path allowlist in `Check for runtime changes` so workflow flag changes trigger deploys on `main`.
- Updated `run_deploy` in `.github/workflows/deploy.yml` to pass `--cpu 2`, `--memory 2Gi`, and `--execution-environment=gen2`.
- Verified `.github/workflows/deploy.yml` with `actionlint` (exited 0).

### Task 2: Update the default `REGION` in `deploy/setup.sh` from `us-central1` to `us-east4` (both usage text and fallback assignment); verify with `bash -n deploy/setup.sh`.

- Updated `REGION` in `deploy/setup.sh` from `us-central1` to `us-east4` in both the `usage()` help text and the `REGION="${REGION:-us-east4}"` fallback assignment.
- Verified `deploy/setup.sh` syntax with `bash -n deploy/setup.sh` (exited 0).

### Task 3: Docs: update `docs/runbooks/deploy.md` and `docs/guides/gcp-setup.md` for the `2 vCPU / 2 GiB / Gen 2` sizing, the `deploy.yml` runtime-change trigger, the `us-east4` default region, and updated monthly spend expectations; verify with `good-vibes docs lint`.

- Updated `docs/runbooks/deploy.md` to include `.github/workflows/deploy.yml` in the runtime-change check, document the `2 vCPU, 2 GiB, Gen 2` Cloud Run deploy flags, switch the rollback example region to `us-east4`, and record updated spend expectations (25 free active hours/month in `us-east4`, ~$5/month at ~55 active hours/month).
- Updated `docs/guides/gcp-setup.md` to use `us-east4` as the default `REGION` / `GCP_REGION` across the setup table, `gh variable set` snippet, and optional Cloud Scheduler command.
- Verified with `good-vibes docs lint` (0 errors, 0 warnings on both modified docs).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

Remaining tasks in Phase 1 align `deploy/setup.sh` and living docs (`docs/runbooks/deploy.md`, `docs/guides/gcp-setup.md`) to `us-east4` and the new 2 vCPU / 2 GiB / Gen 2 sizing.

### Task 2

Remaining Phase 1 work updates living docs (`docs/runbooks/deploy.md` and `docs/guides/gcp-setup.md`) to reflect `us-east4` and the 2 vCPU / 2 GiB / Gen 2 Cloud Run sizing.

### Task 3

All Phase 1 tasks are complete; post-merge rollout verification in `design.md` (confirming `deploy.yml` does not skip, `2 vCPU / 2 GiB / Gen 2` on the live Cloud Run revision, and peak memory headroom) remains after merge.

## Files touched

- `.github/workflows/deploy.yml`
- `deploy/setup.sh`
- `docs/guides/gcp-setup.md`
- `docs/runbooks/deploy.md`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
