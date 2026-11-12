# Task 2

## Done

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

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- `bugs.NewService` accepts `bugs.Deps{Store, GitHub, Repo, BaseURL, GitHubToken, Logger}` and automatically creates a
  `RESTClient` when `GitHub` is nil and `GitHubToken` is non-empty.
- `BuildIssueBody` omits the `### Screenshot` section when `report.Screenshot` is empty so reports without a captured
  image do not link to a 404 screenshot URL.
