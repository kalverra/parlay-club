# Task 10: Docs: update `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document the six secrets, deployer viewer permissions, pre-flight checks, and build caching.

## Done

- `docs/guides/gcp-setup.md` now lists six secrets (`github-token` marked optional), the deployer's
  `secretmanager.viewer` and `storage.bucketViewer` grants, and a note to re-run `deploy/setup.sh` on an existing
  project. Step 3 is "Add a version to each required secret" and explains the five required versions plus how to add the
  optional `github-token` for GitHub issue filing.
- The variables step adds the optional `ISSUES_REPO` variable, says the workflow passes it as `GITHUB_REPO`, and notes
  the seven required variables are checked before any GCP call. The first-deploy step describes the new pre-flight.
- Both troubleshooting tables add rows for missing variables, a bad `BASE_URL`, unreadable secrets (missing
  `secretmanager.viewer`), missing bucket or Artifact Registry repo, missing secret versions, and bug reports that stay in
  SQLite because `github-token` has no version.
- `docs/runbooks/deploy.md` rewrites "What the workflow does" into nine steps covering the variable check, the GCP
  pre-flight, Buildx with the GitHub Actions layer cache, and the conditional `GITHUB_TOKEN` mount. It adds a "The build
  cache" section (warm versus cold builds, 7-day/10 GB eviction, no Artifact Registry usage) and a spend bullet saying
  the cache does not count against the registry free tier.
- Front matter: both docs bump `last_verified` to 2026-11-13; the deploy runbook adds `deploy/setup.sh` and `Dockerfile`
  to its sources. `rumdl` and `good-vibes docs lint` are clean and `good-vibes docs index` ran.

## Deviations

None. The docs now describe the state left by Tasks 8 and 9, including `ISSUES_REPO` and the pre-flight's handling of a
versionless `github-token`.

## Open issues

- Nothing was run against a real GCP project. The documented pre-flight messages and role requirements come from
  `.github/workflows/deploy.yml` and `deploy/setup.sh`, not from a live run.
- `docs/reference/config.md` still says `GITHUB_REPO` defaults to `kalverra/parlay-club` in prod. That is still the
  binary's default, but the deploy now always sets the env var from `vars.ISSUES_REPO` or the workflow's repo, so the
  deployed value is never the fallback. Left alone because this task names only the two docs.
- `docs/runbooks/google-oauth-setup.md` lists five secrets in its preconditions. The list is not exhaustive (`github-token`
  is unrelated to OAuth) and remains correct.

## Next-phase notes

- Re-running `deploy/setup.sh` on an already-provisioned project is now documented as required before the first deploy
  with the new workflow: without `roles/secretmanager.viewer` and `roles/storage.bucketViewer`, the pre-flight reports
  unreadable secrets and the bucket instead of a clean missing-resource message.
- If a later task changes the workflow's variable names, secret list, or cache settings, both docs and the anchor
  `gcp-setup.md#3-add-a-version-to-each-required-secret` (linked from the deploy runbook) need to move with it.
