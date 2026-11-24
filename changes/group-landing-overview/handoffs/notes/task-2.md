# Task 2

## Done

- Added `GroupOverviewPage`, `SpotlightSlipCard`, `GroupRecordSnapshot`, and `RecentSlipCard` view models in `internal/web/home.go`.
- Added pure `isSpotlightSlip` classifier in `internal/web/home.go` separating active/upcoming spotlight slips (open, lock-in, join, waiting, export, or locked with live/upcoming eligible games) from closed and graded slips.
- Added `buildGroupOverviewPage`, `buildGroupRecordSnapshot`, `activeStreakLabel`, `recentSlipResult`, and shared `slipProgressSummary`/`buildScheduleViews` helpers to assemble sorted spotlight cards, deduplicated recent slips (capped at 5), record W-L/streak snapshot, recurring schedules, and group members.
- Added `internal/web/home_overview_test.go` covering `isSpotlightSlip` and `buildGroupOverviewPage` across open, multi-slip, locked-live, closed, graded, and empty states.
- Verified with `go test -short ./internal/web/...` and `golangci-lint run ./internal/web/...` (both passing with 0 issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`groupWeek` in `internal/web/home.go` can call `buildGroupOverviewPage(groupOverviewInput{...})` directly once it loads `UpcomingWeeks`, `RecentSlipsForGroup`, `GradedSlipsForGroupSeason`, `GroupSlipSchedules`, and `Members`. Note `GroupRecordSnapshot.Record` formats `"W-L"` when `Pushes == 0` and `"W-L-P"` when `Pushes > 0`, and `Streak` is `"W<n>"`, `"L<n>"`, or `"–"`.

## Commit

Add GroupOverviewPage view model and spotlight classifier

Introduces the view model, pure spotlight slip classification, and overview page assembler so the per-group landing handler can highlight active or upcoming slips while keeping group record, recent results, schedules, and members visible below.
