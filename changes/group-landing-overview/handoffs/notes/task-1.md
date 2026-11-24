# Task 1

## Done

- Added `RecentSlipRow` (pairing `Slip` and `Week`) and `Store.RecentSlipsForGroup(ctx, groupID, limit)` in `internal/db/slips.go`, ordering results by `week_id DESC, id DESC`, guarding `limit <= 0`, and hydrating custom-slip `GameIDs`.
- Added `TestRecentSlipsForGroupOrdersByWeekAndIDAndHonorsLimit` in `internal/db/slips_test.go` covering empty groups, non-positive limits, cross-group isolation, week/ID ordering, limit capping, week metadata, and custom-slip `GameIDs` hydration.
- Verified with `go test -short ./internal/db/...` and `golangci-lint run ./internal/db/...` (both passing with 0 issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`Store.RecentSlipsForGroup` returns all slips for `groupID` regardless of status or spotlight membership; callers on the group overview handler should exclude spotlight slip IDs and cap the displayed recent list as needed.

## Commit

Add RecentSlipsForGroup store query

The group overview page needs to list recent slips with their week labels across weeks instead of only showing slips from the latest week. Adding a single indexed query with week metadata avoids per-slip week lookups in the web handler.
