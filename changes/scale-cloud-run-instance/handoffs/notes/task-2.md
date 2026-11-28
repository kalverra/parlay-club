# Task 2

## Done

- Updated `REGION` in `deploy/setup.sh` from `us-central1` to `us-east4` in both the `usage()` help text and the `REGION="${REGION:-us-east4}"` fallback assignment.
- Verified `deploy/setup.sh` syntax with `bash -n deploy/setup.sh` (exited 0).

## Deviations

None.

## Open issues

None.

## Next-phase notes

Remaining Phase 1 work updates living docs (`docs/runbooks/deploy.md` and `docs/guides/gcp-setup.md`) to reflect `us-east4` and the 2 vCPU / 2 GiB / Gen 2 Cloud Run sizing.

## Commit

Align setup script default region to us-east4

Production already runs in us-east4 via the GCP_REGION GitHub variable, while the bootstrap script still defaulted to us-central1. Aligning the default prevents new setups from provisioning resources in the wrong region when REGION is unset.
