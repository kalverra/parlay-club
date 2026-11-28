# Phase 2: Phase 2: Board grouping modes and pre-pick row scoring

## Done

### Task 3: Add `GroupBy` (`category`, `player`, `grade`), `SortGrade`, `Row.Grade`, `DetailSection`, and `ScoreRows` to `internal/board/board.go`, updating `board.Detail` so empty `GroupBy` defaults to `GroupByCategory` across all non-empty categories (with optional `Category` narrowing), `GroupByPlayer` merges each player's props across categories alongside a `Game lines` section, and `GroupByGrade` buckets individual sides by `A`–`F` with tests in `internal/board/board_test.go`.

- Added `GroupBy` (`GroupByCategory`, `GroupByPlayer`, `GroupByGrade`), `DetailGroupBys`, `SortGrade` (and added it to `DetailSorts`), `Row.Grade`, `Query.GroupBy`, `DetailSection`, and `GameDetail.GroupBy` / `GameDetail.Sections` in `internal/board/board.go`.
- Added `board.ScoreRows(rows, games, stats, engine)` to score non-`OTHER` rows whose game is in `games`, defaulting `nil` engine to `analyzer.ActiveEngine`.
- Updated `board.Detail` to populate `Detail.Sections` across `GroupByCategory` (default, all non-empty categories in tab order with optional `Category` narrowing), `GroupByPlayer` (merging each player's props across categories alongside a `Game lines` section), and `GroupByGrade` (bucketing individual sides by `A`–`F` plus `Ungraded`, omitting empty buckets, and sorting by descending `Grade.Score` with active sort tiebreakers).
- Updated `sortLineGroups` and `sortPlayers` to support `SortGrade`.
- Added unit tests in `internal/board/board_test.go` covering `GroupBy` / `SortGrade` labels, `ScoreRows`, `GroupByCategory` (default and narrowing), `GroupByPlayer`, `GroupByGrade`, and `SortGrade`.

## Deviations

### Task 3

Kept legacy `GameDetail.Category`, `GameDetail.Tabs`, `GameDetail.Lines`, and `GameDetail.Players` populated via `tabCategory` so `internal/web/pick.go` and `internal/web/pick_test.go` continue passing before Phase 3 updates the web layer to render `Detail.Sections`.

## Open issues

None.

## Next-phase notes

### Task 3

Phase 3 should switch `pickDetailView` in `internal/web/pick.go` to render `model.Sections` (and treat empty `q.Category` as "All") and update `pick_test.go` expectations for the default grouped view.

## Files touched

- `internal/board/board.go`
- `internal/board/board_test.go`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: generate 400ms, lint 4.9s, test 8.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
