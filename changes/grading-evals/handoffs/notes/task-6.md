## Done

- Updated `docs/concepts/analyzer.md` to document versioned rubrics (`Rubric`, `RubricV1`, `ActiveRubric`, `ScoreLegWithRubric`, `ScoreSlipWithRubric`) and leak-free offline grading evals.
- Updated `docs/guides/local-dev.md` with an "Offline grading evals" section covering local SQLite runs, rubric comparison flags (`--baseline`, `--candidate`, `--candidate-file`), season/week/format flags, and Litestream restore workflows.
- Ran `good-vibes docs lint`, confirming zero errors or warnings on `docs/concepts/analyzer.md` and zero new warnings on `docs/guides/local-dev.md`.

## Deviations

None.

## Open issues

- `cmd/eval/main.go` and `internal/eval/replay.go` are not yet present in the worktree (see Task 5 notes); the CLI examples in `docs/guides/local-dev.md` match the design contract and will work once Phase 2 and Task 5 land.

## Next-phase notes

- None.

## Commit

Document versioned rubrics and offline grading evals

Operators and contributors need clear instructions for tuning analyzer weights safely and running offline evaluations without leaking post-kickoff data or touching live production state. Updating the analyzer concept doc and local-dev guide ensures the rubric registry and Litestream-restored eval workflow are easy to follow.
