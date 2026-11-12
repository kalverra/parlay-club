# Task 6: Update `docs/reference/routes.md` with `/bug-reports` and `/screenshots/{token}.jpg`

## Done

- Added `internal/web/bugs.go` and `internal/web/screenshots.go` to the `sources` list in
  [routes.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/in-app-bug-reports/docs/reference/routes.md).
- Documented `GET /screenshots/{token}.jpg` under `## Middleware` and `## Public routes`, noting the
  `GET /screenshots/{file}` pattern, `.jpg` suffix check, immutable cache headers, `nosniff`, and 404 behavior.
- Documented `POST /bug-reports` under `### Bug reports` and updated the `400` status code description to include bug
  report validation errors, 2 MiB body cap, supported screenshot MIME types, and `bug-report-success`/`bug-report-error`
  partials.
- Verified with `good-vibes docs lint` (`ok`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 3 adds the front-end bug report trigger, canvas screenshot capture, and `<dialog id="bug-report-dialog">` HTMX
  form; remember to handle 400 `bug-report-error` responses on swap in client JS.
