# Phase 2: Phase 2: Mux auth-boundary tests and documentation

## Done

### Task 3: Add `newMux` HTTP integration tests in `cmd/server/main_test.go` verifying unauthenticated requests, `/healthz`, `/readyz`, `/login`, HTMX partials, and SSE streams never trigger `kick`, while authenticated full-page `GET /` invokes it

- Added `TestMuxKickBoundary` in `cmd/server/main_test.go` exercising `newMux` with real `auth.RequireUser`, `groups.Scope`, `newHealthzHandler`, and `auth.Handlers` (`GET /login`).
- Verified unauthenticated requests (`GET /`, `GET /games`), `GET /healthz`, `GET /readyz`, `GET /login` (unauthenticated and authenticated), HTMX partials (`HX-Request: true`), and SSE streams (`/events` with `Accept: text/event-stream` and `/slips/7/events`) never invoke `kick`.
- Verified authenticated full-page `GET /` invokes `kick` once per request.
- Verified with `good-vibes verify lint` (`golangci-lint run ./... --fix`: 0 issues) and `good-vibes verify --exec -- go test ./cmd/server/...` (pass).

### Task 4: Docs: update `AGENTS.md`, `docs/concepts/data-refresh.md`, `docs/reference/data-sources.md`, `docs/guides/gcp-setup.md`, and `docs/runbooks/deploy.md` for first-authenticated-Sunday-visit pre-fill, `prefill:sunday` in `sync_state`, and optional `POST /internal/prefill` / Cloud Scheduler fallback, then run `good-vibes docs lint`

- Updated `AGENTS.md` free-tier guardrails to reflect first-authenticated-Sunday-visit pre-fill (`prefill:sunday` in `sync_state`) and `POST /internal/prefill` / Cloud Scheduler as an optional fallback.
- Updated `docs/concepts/data-refresh.md` (summary, auth/route boundary diagram, `Kick()` / `RunPrefill()` Sunday orchestration, one-pass flow, `prefill:sunday` staleness row, and guardrails).
- Updated `docs/reference/data-sources.md` budget and bulk-props sections for first-Sunday-visit `RunPrefill` (`prefill:sunday`) and optional `POST /internal/prefill` fallback.
- Updated `docs/guides/gcp-setup.md` and `docs/runbooks/deploy.md` to document `prefill-token` and the Cloud Scheduler job as an optional fallback alongside automatic first-Sunday-visit pre-fill.
- Ran `good-vibes docs lint`: `0 error(s), 38 warning(s)` (all warnings are pre-existing retired front-matter notices in untouched files; zero warnings on edited files).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

- Task 4 can update the documentation under `docs/` for the first-Sunday-visit prefill behavior.

### Task 4

- All Phase 2 tasks are complete.

## Files touched

- `AGENTS.md`
- `cmd/server/main_test.go`
- `docs/concepts/data-refresh.md`
- `docs/guides/gcp-setup.md`
- `docs/reference/data-sources.md`
- `docs/runbooks/deploy.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
