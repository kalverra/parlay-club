## Done

- Updated `docs/runbooks/deploy.md` to include `.github/workflows/deploy.yml` in the runtime-change check, document the `2 vCPU, 2 GiB, Gen 2` Cloud Run deploy flags, switch the rollback example region to `us-east4`, and record updated spend expectations (25 free active hours/month in `us-east4`, ~$5/month at ~55 active hours/month).
- Updated `docs/guides/gcp-setup.md` to use `us-east4` as the default `REGION` / `GCP_REGION` across the setup table, `gh variable set` snippet, and optional Cloud Scheduler command.
- Verified with `good-vibes docs lint` (0 errors, 0 warnings on both modified docs).

## Deviations

None.

## Open issues

None.

## Next-phase notes

All Phase 1 tasks are complete; post-merge rollout verification in `design.md` (confirming `deploy.yml` does not skip, `2 vCPU / 2 GiB / Gen 2` on the live Cloud Run revision, and peak memory headroom) remains after merge.

## Commit

Update deploy and setup docs for 2 vCPU, 2 GiB, Gen 2, and us-east4

Keep the deploy runbook and GCP setup guide aligned with the new Cloud Run
sizing, workflow diff trigger, and production default region so operators
have accurate flags, commands, and monthly spend expectations.
