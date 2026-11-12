# Phase 1: Phase 1: Bug Report Storage, Configuration, and GitHub Client

## Done

### Task 1: Add migration `0010_bug_reports.sql` and `internal/db` bug report store methods

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

### Task 2: Add `GITHUB_TOKEN`/`GITHUB_REPO` config and `internal/bugs` GitHub REST client

- Added `GitHubToken` (`GITHUB_TOKEN`) and `GitHubRepo` (`GITHUB_REPO`, defaulting to `kalverra/parlay-club`) to
  [config.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/config/config.go),
  keeping both optional in dev and prod, with tests in
  [config_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/config/config_test.go).
- Implemented `internal/bugs` with
  [github.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/github.go)
  (`GitHubClient`, `CreateIssueRequest`, `IssueResult`, and `RESTClient` targeting `POST /repos/{owner}/{repo}/issues`),
  [markdown.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/markdown.go)
  (`BuildIssueTitle` and `BuildIssueBody`), and
  [service.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/service.go)
  (`Service.SubmitReport` persisting to SQLite first and degrading gracefully when GitHub is unconfigured or fails).
- Added unit tests in
  [github_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/github_test.go),
  [markdown_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/markdown_test.go),
  and
  [service_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/bugs/service_test.go),
  and verified `go test -count=1 ./internal/config/... ./internal/bugs/...` and
  `golangci-lint run ./internal/config/... ./internal/bugs/...` pass with 0 issues.

### Task 3: Docs: update `docs/reference/config.md` with GitHub issue reporting environment variables

- Updated `docs/reference/config.md` with `GITHUB_TOKEN` and `GITHUB_REPO` in the `## Variables` table, including
  defaults, whitespace trimming, and optional behavior in dev and prod.
- Updated `docs/reference/config.md` front matter (`summary`, `tags`, `last_verified`) and regenerated `docs/llms.txt`.
- Verified `good-vibes docs lint` (`ok`) and `good-vibes docs index`.

## Deviations

### Task 2

- None.

## Open issues

### Task 2

- None.

## Next-phase notes

### Task 1

- `Store.CreateBugReport` generates a 32-char hex token when `Token` is empty, trims `Description`, and defaults
  `ScreenshotMIME` to `"image/jpeg"`.
- Both `ScreenshotByToken` and `GetScreenshotByToken` are available on `db.Store`, and both `SetGitHubIssue` (by ID) and
  `SetGitHubIssueByToken` (by token) are implemented.

### Task 2

- `bugs.NewService` accepts `bugs.Deps{Store, GitHub, Repo, BaseURL, GitHubToken, Logger}` and automatically creates a
  `RESTClient` when `GitHub` is nil and `GitHubToken` is non-empty.
- `BuildIssueBody` omits the `### Screenshot` section when `report.Screenshot` is empty so reports without a captured
  image do not link to a 404 screenshot URL.

### Task 3

- Phase 2 wires `internal/bugs` into `cmd/server` and `internal/web` (`POST /bug-reports` and
  `GET /screenshots/{token}.jpg`).
- Remember to update `docs/reference/routes.md` and `docs/runbooks/deploy.md` in the later documentation tasks once the
  HTTP endpoints and deployment env vars are wired up.

## Files touched

- `docs/llms.txt`
- `docs/reference/config.md`
- `internal/bugs/github.go`
- `internal/bugs/github_test.go`
- `internal/bugs/markdown.go`
- `internal/bugs/markdown_test.go`
- `internal/bugs/service.go`
- `internal/bugs/service_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/db/bugs.go`
- `internal/db/bugs_test.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0010_bug_reports.sql`

## Run log

- Mode:
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
