# Task 5: Authenticated `POST /bug-reports` Handler

## Done

- Added `bugHandlers` and `POST /bug-reports` in
  [bugs.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/bugs.go),
  validating authenticated users, capping request bodies at 2 MiB with `http.MaxBytesReader`, requiring a non-empty
  `description`, and decoding optional base64 `data:` image URLs (`image/jpeg`, `image/png`, `image/webp`).
- Added `bug-report-success` and `bug-report-error` HTMX partials in
  [partials.html](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/web/templates/partials.html)
  and registered them in
  [render.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/render.go).
- Wired `Bugs *bugs.Service` into `web.Dependencies` in
  [routes.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/routes.go)
  and constructed `bugs.NewService` in
  [main.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/cmd/server/main.go).
- Added unit tests in
  [bugs_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/bugs_test.go)
  and `TestMuxProtectsBugReports` in
  [main_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/cmd/server/main_test.go).
- Verified with `golangci-lint run ./internal/web/... ./cmd/server/...` (0 issues) and
  `go test ./internal/web/... ./cmd/server/...` (all passing).

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- `POST /bug-reports` returns HTTP 200 with `bug-report-success` on valid submissions and HTTP 400 with
  `bug-report-error` (`data-bug-report-error`, `role="alert"`) on validation failures. Phase 3 client JS/HTMX should
  allow 400 responses to swap into the error/body container (since HTMX ignores 4xx swaps by default).
