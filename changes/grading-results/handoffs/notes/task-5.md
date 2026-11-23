# Task 5 Notes

## Done

- Added the `Grading eval benchmark summary` step to `.github/workflows/ci.yml` after `Test` to run `go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown >> "$GITHUB_STEP_SUMMARY"`.
- Verified `.github/workflows/ci.yml` with `actionlint` and tested the `cmd/eval` markdown command against `internal/eval/testdata/benchmark_dataset.json`.
- Left `lefthook.yml` `pre-push` unchanged.

## Deviations

None.

## Open issues

None.

## Next-phase notes

The next tasks can add `[tasks.eval-prod]` in `mise.toml`, `scripts/eval-prod.sh`, and the living docs updates.

## Commit

Add offline grading eval summary to CI workflow

Publish deterministic benchmark calibration and factor attribution tables to the GitHub Actions job summary on every run without touching production data or slowing down local pre-push hooks.
