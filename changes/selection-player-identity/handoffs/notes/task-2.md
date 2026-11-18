# Task 2

## Done

- Removed `IsPlayerMarket` and `normalizePlayerName` from [internal/grader/grader.go](internal/grader/grader.go) and replaced them with `db.IsPlayerMarket` and `db.NormalizePlayerName` (also extracting `teamHint` via `db.StripPlayerTag` when `leg.Team` is empty).
- Updated [internal/refresh/grade.go](internal/refresh/grade.go) and [docs/concepts/data-refresh.md](docs/concepts/data-refresh.md) to reference `db.IsPlayerMarket` instead of `grader.IsPlayerMarket`.
- Added `TestGradeLeg_PlayerNameNormalization` in [internal/grader/grader_test.go](internal/grader/grader_test.go) covering accent-folded name matching and `(TEAM)`-tagged prop disambiguation, and updated `TestIsPlayerMarket` to call `db.IsPlayerMarket`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 1 (unifying player normalization and resolution across `db`, `grader`, `board`, and `research`) is complete; Phase 2 can proceed with `SameLineGroup`, `SamePlayer`, `MatchBookOdds`, and moved-line selection checks.
