## Done

- Added `.github/workflows/deploy.yml` to the `git diff --quiet` path allowlist in `Check for runtime changes` so workflow flag changes trigger deploys on `main`.
- Updated `run_deploy` in `.github/workflows/deploy.yml` to pass `--cpu 2`, `--memory 2Gi`, and `--execution-environment=gen2`.
- Verified `.github/workflows/deploy.yml` with `actionlint` (exited 0).

## Deviations

None.

## Open issues

None.

## Next-phase notes

Remaining tasks in Phase 1 align `deploy/setup.sh` and living docs (`docs/runbooks/deploy.md`, `docs/guides/gcp-setup.md`) to `us-east4` and the new 2 vCPU / 2 GiB / Gen 2 sizing.
