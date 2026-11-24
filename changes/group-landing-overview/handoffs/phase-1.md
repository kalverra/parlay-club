# Phase 1: Phase 1: Group overview data queries and spotlight classification

## Done

### Task 1: Add `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go` (returning recent slips with their week metadata) with store tests.

- Added `RecentSlipRow` (pairing `Slip` and `Week`) and `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go`, ordering results by `week_id DESC, id DESC`, guarding `limit <= 0`, and hydrating custom-slip `GameIDs`.
- Added `TestRecentSlipsForGroupOrdersByWeekAndIDAndHonorsLimit` in `internal/db/slips_test.go` covering empty groups, non-positive limits, cross-group isolation, week/ID ordering, limit capping, week metadata, and custom-slip `GameIDs` hydration.
- Verified with `go test -short ./internal/db/...` and `golangci-lint run ./internal/db/...` (both passing with 0 issues).

### Task 2: Add `GroupOverviewPage` view model and pure `isSpotlightSlip` classifier in `internal/web/home.go` that separates active/upcoming spotlight slips from recent/completed slips and assembles the group record snapshot, recurring schedules, and members, with unit tests covering open, locked-live, closed, graded, and empty states.

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

### Task 1

`Store.RecentSlipsForGroup` returns all slips for `groupID` regardless of status or spotlight membership; callers on the group overview handler should exclude spotlight slip IDs and cap the displayed recent list as needed.

### Task 2

`groupWeek` in `internal/web/home.go` can call `buildGroupOverviewPage(groupOverviewInput{...})` directly once it loads `UpcomingWeeks`, `RecentSlipsForGroup`, `GradedSlipsForGroupSeason`, `GroupSlipSchedules`, and `Members`. Note `GroupRecordSnapshot.Record` formats `"W-L"` when `Pushes == 0` and `"W-L-P"` when `Pushes > 0`, and `Streak` is `"W<n>"`, `"L<n>"`, or `"–"`.

## Files touched

- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/web/home.go`
- `internal/web/home_overview_test.go`
- `web/static/app.css`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 700ms, lint 4.6s, test 4.9s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
