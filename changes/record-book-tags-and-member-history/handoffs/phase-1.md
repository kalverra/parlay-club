# Phase 1: Phase 1: Pure Record Book tag engine and season data wiring

## Done

### Task 1: Add `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go` and `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go`, with unit tests covering engine selection, ungraded/market filtering, and group/season/graded-slip vote scoping.

- Added `SuiteReport.LegGrades(engineName string) map[int64]string` in `internal/eval/metrics.go`, selecting the matching engine report (falling back to `ActiveRubric`, then `Reports[0]`) and filtering out market legs, non-positive leg IDs, and ungraded/blank bands.
- Added `Store.VotesForGroupSeason(ctx, groupID, season)` in `internal/db/record.go` to return net `SUM(value)` vote tallies keyed by leg ID for `GRADED` slips in the given group and season.
- Added `TestSuiteReportLegGrades` in `internal/eval/metrics_test.go` and `TestVotesForGroupSeasonScopesAndSums` in `internal/db/record_test.go`.

### Task 2: Extend `internal/record` (`record.go` and new `tags.go`) with `Leg.ID`, `Leg.Grade`, `Leg.NetVotes`, `MemberRow.ImpliedLift`, active leg streaks, and the 14 deterministic praise/insult tags with plain-English rules and member-specific explanations; add boundary and streak unit tests in `internal/record/record_test.go`.

- Extended `record.Leg` with `ID`, `Grade`, and `NetVotes`, and `record.MemberRow` with `ImpliedLift`, `CurrentWinStreak`, `CurrentLossStreak`, and `Tags` in `internal/record/record.go`.
- Sorted legs stably by `ID` within each graded slip so chronological active win/loss leg streaks are deterministic while skipping `PUSH` and `VOID` legs.
- Added `internal/record/tags.go` with `TagTone`, `Tag`, and the 14 deterministic praise and insult tags ordered with praise first and `parlay_terrorist` first among insults, each carrying a plain-English `Rule` and member-specific `Explanation`.
- Added unit tests in `internal/record/record_test.go` covering `ImpliedLift`, active leg streaks across `PUSH`/`VOID` legs, minimum-sample and threshold boundaries for all 14 tags, and deterministic tag ordering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `SuiteReport.LegGrades` returns `nil` when `Reports` is empty, and `Store.VotesForGroupSeason` returns a non-nil empty map when no votes match. Both are safe for direct map lookups (`grades[id]`, `votes[id]`).

### Task 2

- `internal/web/record.go` can now populate `record.Leg.ID`, `record.Leg.Grade` (from `SuiteReport.LegGrades`), and `record.Leg.NetVotes` (from `Store.VotesForGroupSeason`), and render `MemberRow.Tags` and `MemberRow.ImpliedLift`.
- `MemberRow.ParlayTerrorist` and `MemberRow.TerroristCount` remain unchanged alongside the `parlay_terrorist` tag.

## Files touched

- `internal/db/record.go`
- `internal/db/record_test.go`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/record/record.go`
- `internal/record/record_test.go`
- `internal/record/tags.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 700ms, lint 5.4s, test 8.8s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
