# Task 8: Screen capture, preview rendering, and form submission in `web/static/app.js`

## Done

- Added bug report screen capture in `web/static/app.js` using `window.html2canvas` on `document.body`
  (viewport-bounded), downscaling canvases wider than 1280px and encoding as JPEG (`0.8` quality) with a fallback to an
  empty screenshot on failure.
- Added preview rendering and diagnostic field population in `web/static/app.js`, inserting `<img id="bug-preview">`
  into `#bug-preview-slot` when capture succeeds, populating `#bug-input-screenshot`, `#bug-input-page-url`,
  `#bug-input-screen-size`, and `#bug-input-user-agent`, and opening `#bug-report-dialog`.
- Added HTMX `htmx:beforeSwap`, `htmx:beforeRequest`, and `htmx:responseError` handling for `#bug-report-form` so HTTP
  400 validation errors swap into `#bug-report-error`, non-400 errors display a fallback alert in `#bug-report-error`,
  and closing `#bug-report-dialog` via `[data-bug-dialog-close]` or `close` resets `#bug-report-body` to its initial
  form markup.
- Added web handler integration tests in `internal/web/bugs_test.go` (`TestBugReportSubmitServesScreenshotRoundTrip` and
  `TestBugReportSubmitGitHubFailureStillSavesLocally`) and static asset wiring tests in `internal/web/static_test.go`
  (`TestAppJSHandlesBugReportCaptureAndSubmission`).
- Verified `go test -count=1 ./internal/web/... ./web/...` passes.

## Deviations

None.

## Open issues

- None.

## Next-phase notes

- None.
