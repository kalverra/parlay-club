# Close-out

## Consequences

- Signed-in members can report bugs from any page with one tap, attaching a viewport screenshot and page context without
  needing a GitHub account.
- Bug reports persist in SQLite before any GitHub API call, so GitHub downtime or an unset `GITHUB_TOKEN` never drops a
  report.
- Storing JPEG screenshot blobs in SQLite adds write volume to the single database file and Litestream replica; this is
  bounded by client-side 1280px downscaling, a 1.3 MB data-URL cap, and a 1 MiB decoded image limit on the server.
- `html2canvas-pro` (215 KB) is vendored in `web/static/` and lazy-loaded only when a signed-in user opens the bug
  report dialog, avoiding bundle cost on anonymous or normal page loads.

## Alternatives considered

- **User GitHub OAuth:** Rejected because most club members do not have GitHub accounts. A server-side `GITHUB_TOKEN`
  files issues on their behalf.
- **Third-party object storage (GCS/S3) for screenshots:** Rejected to avoid extra buckets, IAM configuration, and
  billing; SQLite is already replicated by Litestream, and capability URLs (`/screenshots/{token}.jpg`) serve images
  directly to GitHub's markdown proxy.
- **`navigator.mediaDevices.getDisplayMedia`:** Rejected because it triggers OS screen-sharing prompts and is
  unsupported on iOS Safari. In-page DOM capture runs without extra permissions.
- **`html2canvas` 1.4.1:** Replaced with `html2canvas-pro` 2.6.4 because Tailwind v4 emits `oklch()` and `color-mix()`
  colors that `html2canvas` 1.4.1 rejects.

## Implementation notes

- Go's `net/http.ServeMux` requires wildcards to span an entire path segment, so `GET /screenshots/{file}` is registered
  on the outer mux (outside `RequireUser`) and the `.jpg` suffix is validated inside `ScreenshotHandler`.
- `bug-report-dialog` starts with an empty `#bug-preview-slot` container rather than a blank `<img>` tag so pages
  without a user picture do not emit an `<img>` element; `app.js` inserts `<img id="bug-preview">` only when capture
  succeeds.
- Client-supplied context fields (`page_url`, `screen_size`, `user_agent`) are collapsed to one line and stripped of
  backticks before formatting GitHub issue markdown, and `description` is capped at 4000 runes to stay well below
  GitHub's 65,536-character issue body limit.

## Follow-ups

- Manual phone-width browser verification of `html2canvas-pro` capture on iOS Safari / Chrome against Tailwind v4
  `oklch()` styles.
- Configure `GITHUB_TOKEN` (and `GITHUB_REPO` if different from `kalverra/parlay-club`) in Cloud Run secrets for
  production issue filing.
