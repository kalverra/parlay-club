# Task 4

## Done

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

## Deviations

- Go's `net/http.ServeMux` requires wildcards to span an entire path segment, so `GET /screenshots/{token}.jpg` is not a
  valid mux pattern. Registered `GET /screenshots/{file}` and validated the `.jpg` suffix inside `ScreenshotHandler`
  (returning 404 for any non-`.jpg` suffix or empty token).

## Open issues

- None.

## Next-phase notes

- `newMux` in `cmd/server/main.go` now takes `screenshots http.Handler` right after `static` so public screenshot
  delivery bypasses `RequireUser` and `scope` while still sitting behind `CSRF` (which allows `GET`).
