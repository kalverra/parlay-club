## Done

- Extended `record.Leg` with `ID`, `Grade`, and `NetVotes`, and `record.MemberRow` with `ImpliedLift`, `CurrentWinStreak`, `CurrentLossStreak`, and `Tags` in `internal/record/record.go`.
- Sorted legs stably by `ID` within each graded slip so chronological active win/loss leg streaks are deterministic while skipping `PUSH` and `VOID` legs.
- Added `internal/record/tags.go` with `TagTone`, `Tag`, and the 14 deterministic praise and insult tags ordered with praise first and `parlay_terrorist` first among insults, each carrying a plain-English `Rule` and member-specific `Explanation`.
- Added unit tests in `internal/record/record_test.go` covering `ImpliedLift`, active leg streaks across `PUSH`/`VOID` legs, minimum-sample and threshold boundaries for all 14 tags, and deterministic tag ordering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/web/record.go` can now populate `record.Leg.ID`, `record.Leg.Grade` (from `SuiteReport.LegGrades`), and `record.Leg.NetVotes` (from `Store.VotesForGroupSeason`), and render `MemberRow.Tags` and `MemberRow.ImpliedLift`.
- `MemberRow.ParlayTerrorist` and `MemberRow.TerroristCount` remain unchanged alongside the `parlay_terrorist` tag.

## Commit

Add Record Book tag engine and implied lift stats

Compute member implied-odds lift, active leg streaks, and the 14 deterministic praise and insult tags in `internal/record` so the Record Book leaderboard and member history views can display earned badges with plain-English rules and member-specific explanations.
