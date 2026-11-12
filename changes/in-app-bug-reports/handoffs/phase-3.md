# Phase 3: Phase 3: Client UI, Screen Capture, and Modal Workflow

## Done

### Task 7: Vendor screen capture script and add bug trigger and `<dialog id="bug-report-dialog">`

- Vendored `html2canvas` 1.4.1 into `web/static/html2canvas.min.js` and linked it with `defer` before `app.js` in
  `web/templates/layout.html`.
- Added the `#bug-report-trigger` header button and `{{template "bug-report-dialog" .}}` to `web/templates/layout.html`
  for signed-in users (`.User != nil`).
- Added `{{define "bug-report-dialog"}}` to `web/templates/partials.html` with `#bug-report-body`, `#bug-report-error`,
  `#bug-report-form` (`hx-post="/bug-reports"`), `#bug-preview-slot`, hidden diagnostic inputs, and
  `data-bug-dialog-close`.
- Regenerated `web/static/app.css` via `go generate ./...` and added embed/layout tests in `web/embed_test.go` and
  `internal/web/layout_test.go`.

### Task 8: Implement screen capture, preview, and submission in `app.js`, with integration tests

The worker wrote no notes.

## Deviations

### Task 7

- Used `<div id="bug-preview-slot">` instead of an empty `<img id="bug-preview">` in the initial `bug-report-dialog`
  markup so signed-in pages without a captured screenshot do not emit an `<img>` tag (existing avatar fallback tests
  assert `NotContains(t, out, "<img")` when the user has no picture URL).

## Open issues

### Task 7

- None.

## Next-phase notes

### Task 7

- Task 8 (`web/static/app.js`) should insert
  `<img id="bug-preview" alt="Screenshot preview" class="max-h-48 w-full object-contain">` into `#bug-preview-slot` and
  unhide `#bug-preview-slot` when `html2canvas` succeeds (and clear/hide it when capture fails or the dialog resets).
- On `htmx:beforeSwap` for `#bug-report-form`, allow HTTP 400 responses to swap into `#bug-report-error`
  (`detail.shouldSwap = true; detail.target = ...; detail.isError = false`) so the form stays intact for resubmission.

## Files touched

- `internal/web/layout_test.go`
- `web/embed_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/static/html2canvas.min.js`
- `web/templates/layout.html`
- `web/templates/partials.html`

## Run log

- Mode:
- Checkpoints: task-7, task-8
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
