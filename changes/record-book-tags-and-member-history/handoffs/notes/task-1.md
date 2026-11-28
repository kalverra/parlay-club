## Done

- Added `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go`, selecting the matching engine report (falling back to `ActiveRubric`, then `Reports[0]`) and filtering out market legs, non-positive leg IDs, and ungraded/blank bands.
- Added `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go` to return net `SUM(value)` vote tallies keyed by leg ID for `GRADED` slips in the given group and season.
- Added `TestSuiteReportLegGrades` in `internal/eval/metrics_test.go` and `TestVotesForGroupSeasonScopesAndSums` in `internal/db/record_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `SuiteReport.LegGrades` returns `nil` when `Reports` is empty, and `Store.VotesForGroupSeason` returns a non-nil empty map when no votes match. Both are safe for direct map lookups (`grades[id]`, `votes[id]`).

## Commit

Expose pre-kickoff leg grades and season vote tallies

The Record Book tag engine needs historical pre-kickoff leg grades and per-leg net vote counts without adding new grade tables or N+1 per-slip queries. Exposing `LegGrades` on `SuiteReport` and `VotesForGroupSeason` on `Store` provides both inputs cleanly for `record.Build`.
