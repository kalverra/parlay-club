# Task 1 Notes

## Done

- Added migration
  [0010_bug_reports.sql](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/db/migrations/0010_bug_reports.sql)
  creating `bug_reports` and `idx_bug_reports_token`.
- Added
  [bugs.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/db/bugs.go)
  with `BugReport`, `GenerateBugReportToken`, `CreateBugReport`, `BugReportByID`, `BugReportByToken`,
  `ScreenshotByToken` / `GetScreenshotByToken`, `SetGitHubIssue`, and `SetGitHubIssueByToken`.
- Updated migration version assertions in
  [migrate_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/db/migrate_test.go)
  and
  [migrate_internal_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/db/migrate_internal_test.go)
  to schema version 10 and added `TestMigration0010BugReports`.
- Added store tests in
  [bugs_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/db/bugs_test.go)
  and verified `go test ./internal/db/...` and `golangci-lint run ./internal/db/...` pass.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Store.CreateBugReport` generates a 32-char hex token when `Token` is empty, trims `Description`, and defaults
  `ScreenshotMIME` to `"image/jpeg"`.
- Both `ScreenshotByToken` and `GetScreenshotByToken` are available on `db.Store`, and both `SetGitHubIssue` (by ID) and
  `SetGitHubIssueByToken` (by token) are implemented.
