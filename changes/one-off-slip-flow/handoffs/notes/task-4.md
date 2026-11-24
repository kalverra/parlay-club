# Task 4

## Done

- Replaced `groupStep` in `internal/web/home.go` with `groupWeekSummary`, `betterStep`, and `stepPriority`, loading `GamesForWeek` once in `home` and picking the highest-priority `slips.NextStep` across all slips in the latest week (`StepPick`/`StepLockIn`/`StepJoin` > `StepWaiting`/`StepExport` > `StepClosed` > `StepDone`, tie-breaking by earliest upcoming `Deadline` then `SlipsForWeek` order).
- Added `TestHomeGroupSummaryPrioritizesActionableSlips` in `internal/web/home_test.go` covering graded-vs-open, earliest-deadline tie-breaking, locked-vs-open, and export-vs-closed-vs-done ordering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Prioritize actionable slip summaries on Groups home

When a group has multiple slips in the latest week, summarizing only the newest slip hides open picks on an earlier slip once a one-off slip is locked or graded. Ranking all slips by viewer action and earliest upcoming deadline keeps the most urgent next step on the group card.
