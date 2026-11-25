# Task 12

## Done

- Updated `docs/guides/local-dev.md` with a `Train the v2 grading model` section covering `uv` prerequisites, `.dockerignore` exclusion from Cloud Run builds, `mise run train-v2` usage and flags, walk-forward validation gates, generated outputs (`internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, `ml/reports/latest.md`), retraining cadence, and PR verification commands.
- Updated `docs/guides/local-dev.md` front matter (`summary`, `tags`, and `budget`) and regenerated `docs/llms.txt` via `good-vibes docs index`.
- Verified `docs/guides/local-dev.md` and `docs/llms.txt` pass `good-vibes docs lint` (`0` errors) and `typos`, with zero new `rumdl` warnings on the added section.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 5 (`internal/analyzer/overlays.go`, `internal/analyzer/grade_v2.go`) can build directly on `internal/modelv2` and the training/verification workflow documented in `docs/guides/local-dev.md`.

## Commit

Document local train-v2 workflow and PR checks

Add uv setup, walk-forward gates, retraining cadence, and PR verification steps to the local development guide so contributors can safely refresh and validate v2 model artifacts.
