# Phase 2: Phase 2: Web API Endpoints and Screenshot Delivery

## Done

### Task 4: Implement public `GET /screenshots/{token}.jpg` capability route

- Added `web.ScreenshotHandler` in
  [screenshots.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/screenshots.go)
  to serve bug report screenshots by capability token with `Content-Type`, `Content-Length`,
  `Cache-Control: public, max-age=31536000, immutable`, and `X-Content-Type-Options: nosniff`, returning 404 for unknown
  tokens, missing images, invalid extensions, or nil stores.
- Registered `GET /screenshots/{file}` in
  [routes.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/routes.go)
  and as a public unauthenticated route on the outer mux in
  [main.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/cmd/server/main.go).
- Added unit tests in
  [screenshots_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/internal/web/screenshots_test.go)
  and `TestMuxServesScreenshotsWithoutAuth` in
  [main_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/cmd/server/main_test.go).
- Verified with `golangci-lint run ./internal/web/... ./cmd/server/...` (0 issues) and
  `go test ./internal/web/... ./cmd/server/...` (all passing).

### Task 5: Implement authenticated `POST /bug-reports` handler with HTMX partials and tests

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

### Task 6: Docs: update `docs/reference/routes.md` with `/bug-reports` and `/screenshots/{token}.jpg`

- Added `internal/web/bugs.go` and `internal/web/screenshots.go` to the `sources` list in
  [routes.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/docs/reference/routes.md).
- Documented `GET /screenshots/{token}.jpg` under `## Middleware` and `## Public routes`, noting the
  `GET /screenshots/{file}` pattern, `.jpg` suffix check, immutable cache headers, `nosniff`, and 404 behavior.
- Documented `POST /bug-reports` under `### Bug reports` and updated the `400` status code description to include bug
  report validation errors, 2 MiB body cap, supported screenshot MIME types, and `bug-report-success`/`bug-report-error`
  partials.
- Verified with `good-vibes docs lint` (`ok`).

## Deviations

### Task 4

- Go's `net/http.ServeMux` requires wildcards to span an entire path segment, so `GET /screenshots/{token}.jpg` is not a
  valid mux pattern. Registered `GET /screenshots/{file}` and validated the `.jpg` suffix inside `ScreenshotHandler`
  (returning 404 for any non-`.jpg` suffix or empty token).

### Task 5

- None.

## Open issues

### Task 4

- None.

### Task 5

- None.

## Next-phase notes

### Task 4

- `newMux` in `cmd/server/main.go` now takes `screenshots http.Handler` right after `static` so public screenshot
  delivery bypasses `RequireUser` and `scope` while still sitting behind `CSRF` (which allows `GET`).

### Task 5

- `POST /bug-reports` returns HTTP 200 with `bug-report-success` on valid submissions and HTTP 400 with
  `bug-report-error` (`data-bug-report-error`, `role="alert"`) on validation failures. Phase 3 client JS/HTMX should
  allow 400 responses to swap into the error/body container (since HTMX ignores 4xx swaps by default).

### Task 6

- Phase 3 adds the front-end bug report trigger, canvas screenshot capture, and `<dialog id="bug-report-dialog">` HTMX
  form; remember to handle 400 `bug-report-error` responses on swap in client JS.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox_test.go`
- `docs/reference/routes.md`
- `internal/web/bugs.go`
- `internal/web/bugs_test.go`
- `internal/web/render.go`
- `internal/web/routes.go`
- `internal/web/screenshots.go`
- `internal/web/screenshots_test.go`
- `web/templates/partials.html`

## Run log

- Mode:
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
