# Task 3

## Done

- Added `TestMuxKickBoundary` in `cmd/server/main_test.go` exercising `newMux` with real `auth.RequireUser`, `groups.Scope`, `newHealthzHandler`, and `auth.Handlers` (`GET /login`).
- Verified unauthenticated requests (`GET /`, `GET /games`), `GET /healthz`, `GET /readyz`, `GET /login` (unauthenticated and authenticated), HTMX partials (`HX-Request: true`), and SSE streams (`/events` with `Accept: text/event-stream` and `/slips/7/events`) never invoke `kick`.
- Verified authenticated full-page `GET /` invokes `kick` once per request.
- Verified with `good-vibes verify lint` (`golangci-lint run ./... --fix`: 0 issues) and `good-vibes verify --exec -- go test ./cmd/server/...` (pass).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 4 can update the documentation under `docs/` for the first-Sunday-visit prefill behavior.
